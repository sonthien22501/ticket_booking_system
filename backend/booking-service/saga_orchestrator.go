package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
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
	so.rmq.ConsumeWithRetry("booking_inventory_reserved", "inventory.reserved", 3, func(body []byte, headers amqp.Table) error {
		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}
		log.Printf("Saga %s: Inventory Reserved", sagaID)

		// Move to next step: Payment Simulation
		booking, err := so.srv.getBookingTx(sagaID)
		if err != nil {
			log.Printf("Saga %s error: %v", sagaID, err)
			return err
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
		return nil
	})

	// Listen for inventory.reserve_failed
	so.rmq.ConsumeWithRetry("booking_inventory_reserve_failed", "inventory.reserve_failed", 3, func(body []byte, headers amqp.Table) error {
		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}
		log.Printf("Saga %s: Inventory Reserve Failed", sagaID)

		so.srv.db.Exec("UPDATE bookings SET status = 'CANCELLED' WHERE id = $1", sagaID)
		so.completeSaga(sagaID, SagaResult{Status: "FAILED", Error: "Inventory Conflict or Unavailable"})
		return nil
	})

	// Listen for inventory.committed
	so.rmq.ConsumeWithRetry("booking_inventory_committed", "inventory.committed", 3, func(body []byte, headers amqp.Table) error {
		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}
		log.Printf("Saga %s: Inventory Committed", sagaID)

		so.srv.db.Exec("UPDATE bookings SET status = 'CONFIRMED' WHERE id = $1", sagaID)
		
		booking, _ := so.srv.getBookingTx(sagaID)
		so.completeSaga(sagaID, SagaResult{Status: "SUCCESS", Booking: booking})
		return nil
	})

	// Listen for inventory.commit_failed
	so.rmq.ConsumeWithRetry("booking_inventory_commit_failed", "inventory.commit_failed", 3, func(body []byte, headers amqp.Table) error {
		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}
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
		return nil
	})
	
	// Listen for inventory.released
	so.rmq.ConsumeWithRetry("booking_inventory_released", "inventory.released", 3, func(body []byte, headers amqp.Table) error {
		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		log.Printf("Saga %s: Inventory Released (Compensated)", sagaID)
		return nil
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
	var tierID, resID sql.NullString
	err := s.db.QueryRow(
		`SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, created_at, updated_at 
		 FROM bookings WHERE id = $1`,
		bookingID,
	).Scan(
		&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
		&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
		&b.Status, &resID, &b.PaymentStatus, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if tierID.Valid {
		b.TierID = tierID.String
	}
	if resID.Valid {
		b.ReservationID = resID.String
	}
	json.Unmarshal([]byte(seatsRaw), &b.SeatIDs)
	b.BookingID = b.ID
	b.TotalAmountAlt = b.TotalAmount
	return &b, nil
}
