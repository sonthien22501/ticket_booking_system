package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"
)

type SagaOrchestrator struct {
	rmq       *RabbitMQ
	srv       *Server
	activeSagas map[string]chan SagaResult
	mu        sync.RWMutex
}

type SagaResult struct {
	Status string
	Error  string
	Booking *Booking
}

func NewSagaOrchestrator(rmq *RabbitMQ, srv *Server) *SagaOrchestrator {
	so := &SagaOrchestrator{
		rmq:         rmq,
		srv:         srv,
		activeSagas: make(map[string]chan SagaResult),
	}
	so.startConsumers()
	return so
}

func (so *SagaOrchestrator) startConsumers() {
	// Listen for inventory.reserved
	so.rmq.Consume("booking_inventory_reserved", "inventory.reserved", func(body []byte) {
		var msg map[string]interface{}
		json.Unmarshal(body, &msg)
		sagaID, _ := msg["saga_id"].(string)
		log.Printf("Saga %s: Inventory Reserved", sagaID)

		// Move to next step: Payment Simulation
		booking, err := so.srv.getBookingTx(sagaID)
		if err != nil {
			log.Printf("Saga %s error: %v", sagaID, err)
			return
		}

		resID, _ := msg["reservationId"].(string)
		
		// Update booking with reservationId
		so.srv.db.Exec("UPDATE bookings SET reservation_id = $1 WHERE id = $2", resID, sagaID)

		paymentSuccess := true
		// Hack to extract original payment request for simulation - in real world this is stored in state
		// We will simulate payment failure if the payment status is marked as 'FAILED' in db, or we pass it via AMQP...
		// For simplicity, we check if customer name contains "tok_fail" as a hack since we don't have payment table.
		if booking.PaymentStatus == "PENDING_tok_fail" {
			paymentSuccess = false
		}

		if paymentSuccess {
			// Payment success! Commit inventory.
			so.srv.db.Exec("UPDATE bookings SET payment_status = 'SUCCESS' WHERE id = $1", sagaID)
			
			commitPayload := map[string]interface{}{
				"saga_id": sagaID,
				"reservationId": resID,
				"bookingId": sagaID,
			}
			out, _ := json.Marshal(commitPayload)
			so.rmq.Publish("inventory.commit", out)
		} else {
			// Payment failed! Compensate: release inventory.
			so.srv.db.Exec("UPDATE bookings SET payment_status = 'FAILED', status = 'CANCELLED' WHERE id = $1", sagaID)
			
			releasePayload := map[string]interface{}{
				"saga_id": sagaID,
				"reservationId": resID,
				"reason": "PAYMENT_FAILED",
			}
			out, _ := json.Marshal(releasePayload)
			so.rmq.Publish("inventory.release", out)
			so.completeSaga(sagaID, SagaResult{Status: "FAILED", Error: "Payment Failed"})
		}
	})

	// Listen for inventory.reserve_failed
	so.rmq.Consume("booking_inventory_reserve_failed", "inventory.reserve_failed", func(body []byte) {
		var msg map[string]interface{}
		json.Unmarshal(body, &msg)
		sagaID, _ := msg["saga_id"].(string)
		log.Printf("Saga %s: Inventory Reserve Failed", sagaID)

		so.srv.db.Exec("UPDATE bookings SET status = 'CANCELLED' WHERE id = $1", sagaID)
		so.completeSaga(sagaID, SagaResult{Status: "FAILED", Error: "Inventory Conflict or Unavailable"})
	})

	// Listen for inventory.committed
	so.rmq.Consume("booking_inventory_committed", "inventory.committed", func(body []byte) {
		var msg map[string]interface{}
		json.Unmarshal(body, &msg)
		sagaID, _ := msg["saga_id"].(string)
		log.Printf("Saga %s: Inventory Committed", sagaID)

		so.srv.db.Exec("UPDATE bookings SET status = 'CONFIRMED' WHERE id = $1", sagaID)
		
		booking, _ := so.srv.getBookingTx(sagaID)
		so.completeSaga(sagaID, SagaResult{Status: "SUCCESS", Booking: booking})
	})

	// Listen for inventory.commit_failed
	so.rmq.Consume("booking_inventory_commit_failed", "inventory.commit_failed", func(body []byte) {
		var msg map[string]interface{}
		json.Unmarshal(body, &msg)
		sagaID, _ := msg["saga_id"].(string)
		resID, _ := msg["reservationId"].(string)
		log.Printf("Saga %s: Inventory Commit Failed", sagaID)

		so.srv.db.Exec("UPDATE bookings SET status = 'CANCELLED' WHERE id = $1", sagaID)
		
		releasePayload := map[string]interface{}{
			"saga_id": sagaID,
			"reservationId": resID,
			"reason": "COMMIT_FAILED",
		}
		out, _ := json.Marshal(releasePayload)
		so.rmq.Publish("inventory.release", out)
		
		so.completeSaga(sagaID, SagaResult{Status: "FAILED", Error: "Inventory Commit Failed"})
	})
	
	// Listen for inventory.released
	so.rmq.Consume("booking_inventory_released", "inventory.released", func(body []byte) {
		var msg map[string]interface{}
		json.Unmarshal(body, &msg)
		sagaID, _ := msg["saga_id"].(string)
		log.Printf("Saga %s: Inventory Released (Compensated)", sagaID)
		// Compensation complete
	})
}

func (so *SagaOrchestrator) ExecuteSaga(ctx context.Context, booking *Booking, reservePayload map[string]interface{}) SagaResult {
	so.mu.Lock()
	ch := make(chan SagaResult, 1)
	so.activeSagas[booking.ID] = ch
	so.mu.Unlock()

	defer func() {
		so.mu.Lock()
		delete(so.activeSagas, booking.ID)
		so.mu.Unlock()
	}()

	// Trigger step 1
	reservePayload["saga_id"] = booking.ID
	out, _ := json.Marshal(reservePayload)
	so.rmq.Publish("inventory.reserve", out)

	select {
	case res := <-ch:
		return res
	case <-ctx.Done():
		return SagaResult{Status: "TIMEOUT", Error: "Saga timed out"}
	case <-time.After(30 * time.Second):
		return SagaResult{Status: "TIMEOUT", Error: "Saga timed out"}
	}
}

func (so *SagaOrchestrator) completeSaga(sagaID string, res SagaResult) {
	so.mu.RLock()
	ch, exists := so.activeSagas[sagaID]
	so.mu.RUnlock()

	if exists {
		ch <- res
	}
}

func (s *Server) getBookingTx(bookingID string) (*Booking, error) {
	var b Booking
	var seatsRaw string
	err := s.db.QueryRow(`SELECT id, payment_status, seat_ids FROM bookings WHERE id = $1`, bookingID).Scan(&b.ID, &b.PaymentStatus, &seatsRaw)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(seatsRaw), &b.SeatIDs)
	b.BookingID = b.ID
	return &b, nil
}
