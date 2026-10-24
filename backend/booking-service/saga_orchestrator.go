package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type SagaOrchestrator struct {
	rmq         *RabbitMQ
	srv         *Server
	activeSagas map[string]chan SagaResult
	mu          sync.RWMutex
}

type SagaResult struct {
	Status  string
	Error   string
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
		if booking, err := so.srv.getBookingTx(sagaID); err == nil && booking != nil {
			go func() {
				_ = so.TriggerWaitlistPromotion(context.Background(), corrID, booking.EventID, booking.TierID, booking.SeatIDs)
			}()
		}
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

		eventID, _ := msg["eventId"].(string)
		tierID, _ := msg["tierId"].(string)
		var seatIDs []string
		if sRaw, ok := msg["seatIds"].([]interface{}); ok {
			for _, s := range sRaw {
				if sStr, ok := s.(string); ok {
					seatIDs = append(seatIDs, sStr)
				}
			}
		}
		if booking != nil {
			if eventID == "" {
				eventID = booking.EventID
			}
			if tierID == "" {
				tierID = booking.TierID
			}
			if len(seatIDs) == 0 {
				seatIDs = booking.SeatIDs
			}
		}
		if eventID != "" {
			go func() {
				_ = so.TriggerWaitlistPromotion(context.Background(), corrID, eventID, tierID, seatIDs)
			}()
		}
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
		eventID, _ := msg["eventId"].(string)
		tierID, _ := msg["tierId"].(string)
		var seatIDs []string
		if sRaw, ok := msg["seatIds"].([]interface{}); ok {
			for _, s := range sRaw {
				if sStr, ok := s.(string); ok {
					seatIDs = append(seatIDs, sStr)
				}
			}
		}

		if resID != "" {
			StructuredLog("INFO", "booking-service", corrID, fmt.Sprintf("Marking booking expired for reservation %s", resID), nil)
			so.srv.db.Exec("UPDATE bookings SET status = 'EXPIRED', updated_at = NOW() WHERE reservation_id = $1 AND status = 'PENDING'", resID)
		}
		if eventID != "" {
			go func() {
				_ = so.TriggerWaitlistPromotion(context.Background(), corrID, eventID, tierID, seatIDs)
			}()
		}
		return nil
	})
}

func (so *SagaOrchestrator) TriggerWaitlistPromotion(ctx context.Context, corrID, eventID, tierID string, seatIDs []string) error {
	if eventID == "" {
		return fmt.Errorf("eventID is required for waitlist promotion")
	}
	if corrID == "" {
		corrID = GenerateCorrelationID()
	}

	StructuredLog("INFO", "booking-service", corrID, fmt.Sprintf("TriggerWaitlistPromotion initiated for event %s (tier: %s, seats: %v)", eventID, tierID, seatIDs), nil)

	// 1. Concurrency safety: acquire Redis distributed lock
	lockKey := fmt.Sprintf("waitlist:lock:%s", eventID)
	if so.srv != nil && so.srv.rdb != nil {
		acquired, err := so.srv.rdb.SetNX(ctx, lockKey, corrID, 10*time.Second).Result()
		if err != nil || !acquired {
			for i := 0; i < 5; i++ {
				time.Sleep(100 * time.Millisecond)
				if acq, _ := so.srv.rdb.SetNX(ctx, lockKey, corrID, 10*time.Second).Result(); acq {
					acquired = true
					break
				}
			}
		}
		if acquired {
			defer so.srv.rdb.Del(context.Background(), lockKey)
		}
	}

	// 2. Query next FIFO pending waitlist entry
	entry, err := so.srv.FindNextWaitlistEntryForPromotion(ctx, eventID, tierID)
	if err != nil {
		StructuredLog("ERROR", "booking-service", corrID, fmt.Sprintf("FindNextWaitlistEntryForPromotion error: %v", err), nil)
		return err
	}
	if entry == nil {
		StructuredLog("INFO", "booking-service", corrID, fmt.Sprintf("No pending waitlist entry found for event %s (tier: %s)", eventID, tierID), nil)
		return nil
	}

	// 3. Place hold reservation
	var resID string
	expiresAt := time.Now().Add(600 * time.Second)

	targetSeats := seatIDs
	if len(targetSeats) == 0 && entry.SeatID != "" {
		targetSeats = []string{entry.SeatID}
	}

	targetTier := entry.TierID
	if targetTier == "" {
		targetTier = tierID
	}

	reservePayload := map[string]interface{}{
		"eventId":     eventID,
		"tierId":      targetTier,
		"seats":       targetSeats,
		"ticketCount": entry.Quantity,
		"quantity":    entry.Quantity,
		"holdSeconds": 600,
	}

	if so.srv.httpClient != nil && so.srv.inventoryServiceURL != "" {
		reqBody, _ := json.Marshal(reservePayload)
		hReq, hErr := http.NewRequestWithContext(ctx, http.MethodPost, so.srv.inventoryServiceURL+"/inventory/reserve", bytes.NewReader(reqBody))
		if hErr == nil {
			hReq.Header.Set("Content-Type", "application/json")
			hReq.Header.Set("X-Correlation-ID", corrID)
			hResp, doErr := so.srv.httpClient.Do(hReq)
			if doErr == nil && hResp != nil {
				defer hResp.Body.Close()
				if hResp.StatusCode == http.StatusOK || hResp.StatusCode == http.StatusCreated {
					var rResp struct {
						ReservationID string   `json:"reservationId"`
						ExpiresAt     string   `json:"expiresAt"`
						Seats         []string `json:"seats"`
					}
					if errDecode := json.NewDecoder(hResp.Body).Decode(&rResp); errDecode == nil && rResp.ReservationID != "" {
						resID = rResp.ReservationID
						if parsedExp, errP := time.Parse(time.RFC3339, rResp.ExpiresAt); errP == nil {
							expiresAt = parsedExp
						}
					}
				}
			}
		}
	}

	if resID == "" && so.srv.db != nil {
		resID = fmt.Sprintf("res-wl-%d-%s", time.Now().UnixNano(), eventID)
		seatsJSON, _ := json.Marshal(targetSeats)
		_, errIns := so.srv.db.ExecContext(ctx,
			`INSERT INTO reservations (reservation_id, event_id, tier_id, seat_ids, quantity, status, expires_at)
			 VALUES ($1, $2, $3, $4, $5, 'HOLD', $6)`,
			resID, eventID, targetTier, string(seatsJSON), entry.Quantity, expiresAt)
		if errIns != nil {
			StructuredLog("ERROR", "booking-service", corrID, fmt.Sprintf("Failed to create reservation: %v", errIns), nil)
			return fmt.Errorf("failed to create fallback reservation: %w", errIns)
		}
		if len(targetSeats) > 0 {
			_, _ = so.srv.db.ExecContext(ctx,
				`UPDATE seat_items SET status = 'HOLD', reservation_id = $1, hold_expires_at = $2, updated_at = NOW() WHERE event_id = $3 AND id = ANY($4)`,
				resID, expiresAt, eventID, pq.Array(targetSeats))
		}
	}

	// 4. Update waitlist entry status to PROMOTED
	err = so.srv.PromoteWaitlistEntry(ctx, entry.ID, resID, expiresAt)
	if err != nil {
		StructuredLog("ERROR", "booking-service", corrID, fmt.Sprintf("PromoteWaitlistEntry failed for entry %s: %v", entry.ID, err), nil)
		return err
	}

	// 5. Publish waitlist.promoted on saga_exchange
	if so.rmq != nil {
		promotedMsg := map[string]interface{}{
			"waitlistId":    entry.ID,
			"eventId":       entry.EventID,
			"tierId":        targetTier,
			"seatId":        entry.SeatID,
			"userId":        entry.UserID,
			"customerName":  entry.CustomerName,
			"customerEmail": entry.CustomerEmail,
			"quantity":      entry.Quantity,
			"reservationId": resID,
			"expiresAt":     expiresAt.Format(time.RFC3339),
			"status":        "PROMOTED",
		}
		out, _ := json.Marshal(promotedMsg)
		headers := amqp.Table{"x-correlation-id": corrID}
		_ = so.rmq.PublishWithHeaders(SagaExchange, "waitlist.promoted", out, headers)
	}

	StructuredLog("INFO", "booking-service", corrID, fmt.Sprintf("Successfully promoted waitlist entry %s for user %s on event %s (reservation %s)", entry.ID, entry.UserID, eventID, resID), nil)
	return nil
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
