package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}
		StructuredLog("INFO", "booking-service", corrID, fmt.Sprintf("Saga %s: Inventory Reserved", sagaID), nil)

		// Move to next step: Payment Simulation
		booking, err := so.srv.getBookingTx(sagaID)
		if err != nil {
			StructuredLog("ERROR", "booking-service", corrID, fmt.Sprintf("Saga %s error: %v", sagaID, err), nil)
			return err
		}

		resID, _ := msg["reservationId"].(string)

		// Update booking with reservationId
		so.srv.db.Exec("UPDATE bookings SET reservation_id = $1 WHERE id = $2", resID, sagaID)

		paymentSuccess := true
		if booking.PaymentStatus == "PENDING_tok_fail" {
			paymentSuccess = false
		}

		outHeaders := amqp.Table{"x-correlation-id": corrID}
		if paymentSuccess {
			// Payment success! Commit inventory.
			so.srv.db.Exec("UPDATE bookings SET payment_status = 'SUCCESS' WHERE id = $1", sagaID)

			commitPayload := map[string]interface{}{
				"saga_id":       sagaID,
				"reservationId": resID,
				"bookingId":     sagaID,
			}
			out, _ := json.Marshal(commitPayload)
			_ = so.rmq.PublishWithHeaders(SagaExchange, "inventory.commit", out, outHeaders)
		} else {
			// Payment failed! Compensate: release inventory.
			so.srv.db.Exec("UPDATE bookings SET payment_status = 'FAILED', status = 'CANCELLED' WHERE id = $1", sagaID)

			releasePayload := map[string]interface{}{
				"saga_id":       sagaID,
				"reservationId": resID,
				"reason":        "PAYMENT_FAILED",
			}
			out, _ := json.Marshal(releasePayload)
			_ = so.rmq.PublishWithHeaders(SagaExchange, "inventory.release", out, outHeaders)
			so.completeSaga(sagaID, SagaResult{Status: "FAILED", Error: "Payment Failed"})
		}
		return nil
	})

	// Listen for inventory.reserve_failed
	so.rmq.ConsumeWithRetry("booking_inventory_reserve_failed", "inventory.reserve_failed", 3, func(body []byte, headers amqp.Table) error {
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}
		StructuredLog("WARN", "booking-service", corrID, fmt.Sprintf("Saga %s: Inventory Reserve Failed", sagaID), nil)

		so.srv.db.Exec("UPDATE bookings SET status = 'CANCELLED' WHERE id = $1", sagaID)
		so.completeSaga(sagaID, SagaResult{Status: "FAILED", Error: "Inventory Conflict or Unavailable"})
		return nil
	})

	// Listen for inventory.committed
	so.rmq.ConsumeWithRetry("booking_inventory_committed", "inventory.committed", 3, func(body []byte, headers amqp.Table) error {
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}
		StructuredLog("INFO", "booking-service", corrID, fmt.Sprintf("Saga %s: Inventory Committed", sagaID), nil)

		so.srv.db.Exec("UPDATE bookings SET status = 'CONFIRMED' WHERE id = $1", sagaID)

		booking, _ := so.srv.getBookingTx(sagaID)
		so.completeSaga(sagaID, SagaResult{Status: "SUCCESS", Booking: booking})
		return nil
	})

	// Listen for inventory.commit_failed
	so.rmq.ConsumeWithRetry("booking_inventory_commit_failed", "inventory.commit_failed", 3, func(body []byte, headers amqp.Table) error {
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}
		resID, _ := msg["reservationId"].(string)
		StructuredLog("ERROR", "booking-service", corrID, fmt.Sprintf("Saga %s: Inventory Commit Failed", sagaID), nil)

		so.srv.db.Exec("UPDATE bookings SET status = 'CANCELLED' WHERE id = $1", sagaID)

		releasePayload := map[string]interface{}{
			"saga_id":       sagaID,
			"reservationId": resID,
			"reason":        "COMMIT_FAILED",
		}
		out, _ := json.Marshal(releasePayload)
		outHeaders := amqp.Table{"x-correlation-id": corrID}
		_ = so.rmq.PublishWithHeaders(SagaExchange, "inventory.release", out, outHeaders)

		so.completeSaga(sagaID, SagaResult{Status: "FAILED", Error: "Inventory Commit Failed"})
		return nil
	})

	// Listen for inventory.released
	so.rmq.ConsumeWithRetry("booking_inventory_released", "inventory.released", 3, func(body []byte, headers amqp.Table) error {
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		StructuredLog("INFO", "booking-service", corrID, fmt.Sprintf("Saga %s: Inventory Released (Compensated)", sagaID), nil)
		return nil
	})

	// Listen for inventory.cancelled
	so.rmq.ConsumeWithRetry("booking_inventory_cancelled", "inventory.cancelled", 3, func(body []byte, headers amqp.Table) error {
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}
		StructuredLog("INFO", "booking-service", corrID, fmt.Sprintf("Saga %s: Inventory Cancelled (Seats Restored)", sagaID), nil)

		so.srv.db.Exec("UPDATE bookings SET status = 'CANCELLED', updated_at = NOW() WHERE id = $1", sagaID)
		so.srv.db.Exec("UPDATE tickets SET status = 'CANCELLED' WHERE booking_id = $1", sagaID)

		booking, _ := so.srv.getBookingTx(sagaID)
		so.completeSaga(sagaID, SagaResult{Status: "CANCELLED", Booking: booking})
		return nil
	})

	// Listen for inventory.cancel_failed
	so.rmq.ConsumeWithRetry("booking_inventory_cancel_failed", "inventory.cancel_failed", 3, func(body []byte, headers amqp.Table) error {
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}
		StructuredLog("WARN", "booking-service", corrID, fmt.Sprintf("Saga %s: Inventory Cancel Failed", sagaID), nil)

		errMsg, _ := msg["error"].(string)
		if errMsg == "" {
			errMsg = "Inventory Cancellation Failed"
		}
		so.completeSaga(sagaID, SagaResult{Status: "FAILED", Error: errMsg})
		return nil
	})

	// Listen for inventory.reservation_expired
	so.rmq.ConsumeWithRetry("booking_inventory_expired", "inventory.reservation_expired", 3, func(body []byte, headers amqp.Table) error {
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		resID, _ := msg["reservationId"].(string)
		if resID != "" {
			StructuredLog("INFO", "booking-service", corrID, fmt.Sprintf("Marking booking expired for reservation %s", resID), nil)
			so.srv.db.Exec("UPDATE bookings SET status = 'EXPIRED', updated_at = NOW() WHERE reservation_id = $1 AND status = 'PENDING'", resID)
		}
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

	corrID := CorrelationIDFromContext(ctx)
	if corrID == "" {
		corrID = GenerateCorrelationID()
	}

	// Trigger step 1 with correlation ID preserved
	reservePayload["saga_id"] = booking.ID
	out, _ := json.Marshal(reservePayload)
	headers := amqp.Table{"x-correlation-id": corrID}
	_ = so.rmq.PublishWithHeaders(SagaExchange, "inventory.reserve", out, headers)

	select {
	case res := <-ch:
		return res
	case <-ctx.Done():
		return SagaResult{Status: "TIMEOUT", Error: "Saga timed out"}
	case <-time.After(30 * time.Second):
		return SagaResult{Status: "TIMEOUT", Error: "Saga timed out"}
	}
}

func (so *SagaOrchestrator) ExecuteCancellationSaga(ctx context.Context, booking *Booking, cancelPayload map[string]interface{}) SagaResult {
	so.mu.Lock()
	ch := make(chan SagaResult, 1)
	so.activeSagas[booking.ID] = ch
	so.mu.Unlock()

	defer func() {
		so.mu.Lock()
		delete(so.activeSagas, booking.ID)
		so.mu.Unlock()
	}()

	corrID := CorrelationIDFromContext(ctx)
	if corrID == "" {
		corrID = GenerateCorrelationID()
	}

	cancelPayload["saga_id"] = booking.ID
	out, _ := json.Marshal(cancelPayload)
	headers := amqp.Table{"x-correlation-id": corrID}
	_ = so.rmq.PublishWithHeaders(SagaExchange, "inventory.cancel", out, headers)

	select {
	case res := <-ch:
		return res
	case <-ctx.Done():
		return SagaResult{Status: "TIMEOUT", Error: "Cancellation saga timed out"}
	case <-time.After(30 * time.Second):
		return SagaResult{Status: "TIMEOUT", Error: "Cancellation saga timed out"}
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
	var tierID, resID, idemKey sql.NullString
	err := s.db.QueryRow(
		`SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, idempotency_key, created_at, updated_at 
		 FROM bookings WHERE id = $1`,
		bookingID,
	).Scan(
		&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
		&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
		&b.Status, &resID, &b.PaymentStatus, &idemKey, &b.CreatedAt, &b.UpdatedAt,
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
	if idemKey.Valid {
		b.IdempotencyKey = idemKey.String
	}
	json.Unmarshal([]byte(seatsRaw), &b.SeatIDs)
	b.BookingID = b.ID
	b.TotalAmountAlt = b.TotalAmount
	return &b, nil
}
