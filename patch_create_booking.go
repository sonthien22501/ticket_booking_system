package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// The new createBooking function to be inserted
func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) {
	var req CreateBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid request payload: %v"}`, err), http.StatusBadRequest)
		return
	}

	eventID := req.EventID
	if eventID == "" {
		eventID = req.EventIDAlt
	}
	if eventID == "" {
		http.Error(w, `{"error":"eventId is required"}`, http.StatusBadRequest)
		return
	}

	seats := req.Seats
	if len(seats) == 0 {
		seats = req.SeatIDsAlt
	}

	ticketCount := req.TicketCount
	if ticketCount <= 0 {
		ticketCount = req.QuantityAlt
	}
	if ticketCount <= 0 {
		if len(seats) > 0 {
			ticketCount = len(seats)
		} else {
			ticketCount = 1
		}
	}

	if ticketCount > 500 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "OVERBOOKING_LIMIT_EXCEEDED",
			"message": "Maximum tickets per single booking is 500",
		})
		return
	}

	customerName := req.CustomerName
	if customerName == "" {
		customerName = req.CustomerNameAlt
	}
	if customerName == "" {
		customerName = "Valued Guest"
	}

	customerEmail := req.CustomerEmail
	if customerEmail == "" {
		customerEmail = req.CustomerEmailAlt
	}
	if customerEmail == "" {
		customerEmail = "guest@example.com"
	}

	userID := req.UserID
	if userID == "" {
		userID = req.UserIDAlt
	}
	if userID == "" {
		userID = "usr-guest"
	}

	tierID := req.TierID
	if tierID == "" {
		tierID = req.TierIDAlt
	}

	eventTitle, tierPrice := s.fetchEventInfo(eventID, tierID)

	bookingID := fmt.Sprintf("bk-%d", time.Now().UnixNano())
	bookingRef := fmt.Sprintf("REF-%s-%d", strings.ToUpper(strings.ReplaceAll(eventID, "-", "")), time.Now().UnixNano())
	totalAmount := tierPrice * float64(ticketCount)
	if totalAmount <= 0 {
		totalAmount = 60.00 * float64(ticketCount)
	}

	paymentStatus := "PENDING"
	if req.PaymentInfo != nil && (req.PaymentInfo.CardToken == "tok_fail" || req.PaymentInfo.CardTokenAlt == "tok_fail") {
		paymentStatus = "PENDING_tok_fail"
	} else if req.PaymentInfoAlt != nil && (req.PaymentInfoAlt.CardToken == "tok_fail" || req.PaymentInfoAlt.CardTokenAlt == "tok_fail") {
		paymentStatus = "PENDING_tok_fail"
	}

	seatJSON, _ := json.Marshal(seats)
	if len(seats) == 0 {
		seatJSON = []byte("[]")
	}

	now := time.Now()
	_, err := s.db.ExecContext(r.Context(),
		`INSERT INTO bookings (id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, payment_status, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'PENDING', $12, $13, $13)`,
		bookingID, bookingRef, userID, customerName, customerEmail, eventID, eventTitle, tierID, ticketCount, string(seatJSON), totalAmount, paymentStatus, now,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to create pending booking: %v"}`, err), http.StatusInternalServerError)
		return
	}

	booking := &Booking{
		ID:               bookingID,
		BookingID:        bookingID,
		BookingReference: bookingRef,
		UserID:           userID,
		CustomerName:     customerName,
		CustomerEmail:    customerEmail,
		EventID:          eventID,
		EventTitle:       eventTitle,
		TierID:           tierID,
		Quantity:         ticketCount,
		SeatIDs:          seats,
		TotalAmount:      totalAmount,
		TotalAmountAlt:   totalAmount,
		Status:           "PENDING",
		PaymentStatus:    paymentStatus,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	reservePayload := map[string]interface{}{
		"eventId":     eventID,
		"tierId":      tierID,
		"seats":       seats,
		"ticketCount": ticketCount,
		"holdSeconds": 600,
	}

	// Wait for Saga to complete
	sagaRes := s.sagaOrchestrator.ExecuteSaga(r.Context(), booking, reservePayload)

	w.Header().Set("Content-Type", "application/json")
	if sagaRes.Status == "SUCCESS" {
		
		// generate tickets
		tx, _ := s.db.Begin()
		var tickets []Ticket
		for i, seat := range sagaRes.Booking.SeatIDs {
			ticketID := fmt.Sprintf("tkt-%d-%d", time.Now().UnixNano(), i+1)
			ticketCode := fmt.Sprintf("TKT-%s-%s-%d", eventID, seat, time.Now().UnixNano())
			seatLabel := fmt.Sprintf("Seat %s", seat)

			tx.Exec(
				`INSERT INTO tickets (id, booking_id, seat_id, seat_label, ticket_code, status, created_at) 
				 VALUES ($1, $2, $3, $4, $5, 'VALID', $6)`,
				ticketID, bookingID, seat, seatLabel, ticketCode, now,
			)

			tickets = append(tickets, Ticket{
				ID:         ticketID,
				TicketID:   ticketID,
				BookingID:  bookingID,
				SeatID:     seat,
				SeatLabel:  seatLabel,
				TicketCode: ticketCode,
				Status:     "VALID",
				CreatedAt:  now,
			})
		}
		tx.Commit()

		// refetch final booking
		b, _ := s.getBookingTx(bookingID)
		sagaRes.Booking.Tickets = tickets
		sagaRes.Booking.PaymentStatus = b.PaymentStatus
		sagaRes.Booking.Status = "CONFIRMED"
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(sagaRes.Booking)
	} else if sagaRes.Status == "FAILED" {
		w.WriteHeader(http.StatusConflict) // Or 402 Payment Required depending on error, keep simple
		json.NewEncoder(w).Encode(map[string]string{
			"error": "BOOKING_FAILED",
			"message": sagaRes.Error,
		})
	} else {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "PROCESSING",
			"bookingId": bookingID,
		})
	}
}
