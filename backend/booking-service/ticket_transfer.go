package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type TicketTransferRequest struct {
	TicketID       string `json:"ticketId,omitempty"`
	TicketCode     string `json:"ticketCode,omitempty"`
	RecipientEmail string `json:"recipientEmail"`
	RecipientName  string `json:"recipientName"`
	FromUserID     string `json:"fromUserId,omitempty"`
	Note           string `json:"note,omitempty"`
}

type TicketTransferResponse struct {
	TransferID     string    `json:"transferId"`
	BookingID      string    `json:"bookingId"`
	NewBookingID   string    `json:"newBookingId"`
	OldTicketID    string    `json:"oldTicketId"`
	NewTicketID    string    `json:"newTicketId"`
	OldTicketCode  string    `json:"oldTicketCode"`
	NewTicketCode  string    `json:"newTicketCode"`
	RecipientEmail string    `json:"recipientEmail"`
	RecipientName  string    `json:"recipientName"`
	SeatLabel      string    `json:"seatLabel"`
	Status         string    `json:"status"`
	TransferredAt  time.Time `json:"transferredAt"`
	Message        string    `json:"message"`
}

type TicketTransferRecord struct {
	ID                 string    `json:"id"`
	OriginalTicketID   string    `json:"originalTicketId"`
	OriginalTicketCode string    `json:"originalTicketCode"`
	NewTicketID        string    `json:"newTicketId"`
	NewTicketCode      string    `json:"newTicketCode"`
	BookingID          string    `json:"bookingId"`
	NewBookingID       string    `json:"newBookingId"`
	FromUserID         string    `json:"fromUserId"`
	FromCustomerEmail  string    `json:"fromCustomerEmail"`
	RecipientEmail     string    `json:"recipientEmail"`
	RecipientName      string    `json:"recipientName"`
	Note               string    `json:"note"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"createdAt"`
}

func (s *Server) initTicketTransferSchema(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	query := `
	CREATE TABLE IF NOT EXISTS ticket_transfers (
		id VARCHAR(64) PRIMARY KEY,
		original_ticket_id VARCHAR(64) NOT NULL,
		original_ticket_code VARCHAR(128) NOT NULL,
		new_ticket_id VARCHAR(64) NOT NULL,
		new_ticket_code VARCHAR(128) NOT NULL,
		booking_id VARCHAR(64) NOT NULL,
		new_booking_id VARCHAR(64) NOT NULL,
		from_user_id VARCHAR(64),
		from_customer_email VARCHAR(255),
		recipient_email VARCHAR(255) NOT NULL,
		recipient_name VARCHAR(255) NOT NULL,
		note TEXT,
		status VARCHAR(32) NOT NULL DEFAULT 'COMPLETED',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_ticket_transfers_orig ON ticket_transfers(original_ticket_id);
	CREATE INDEX IF NOT EXISTS idx_ticket_transfers_recipient ON ticket_transfers(recipient_email);
	`
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func generateTransferToken(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s-%s", prefix, time.Now().Format("20060102150405"), hex.EncodeToString(b))
}

func (s *Server) handleTransferTicket(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req TicketTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid request payload: %v"}`, err), http.StatusBadRequest)
		return
	}

	ticketIdent := strings.TrimSpace(req.TicketCode)
	if ticketIdent == "" {
		ticketIdent = strings.TrimSpace(req.TicketID)
	}
	if ticketIdent == "" {
		http.Error(w, `{"error":"ticketCode or ticketId is required"}`, http.StatusBadRequest)
		return
	}

	recipientEmail := strings.TrimSpace(strings.ToLower(req.RecipientEmail))
	if recipientEmail == "" || !strings.Contains(recipientEmail, "@") {
		http.Error(w, `{"error":"Valid recipientEmail is required"}`, http.StatusBadRequest)
		return
	}

	recipientName := strings.TrimSpace(req.RecipientName)
	if recipientName == "" {
		recipientName = strings.Split(recipientEmail, "@")[0]
	}

	if s.db == nil {
		http.Error(w, `{"error":"Database not initialized"}`, http.StatusInternalServerError)
		return
	}

	// 1. Query original ticket
	var tkt Ticket
	var seatID sql.NullString
	err := s.db.QueryRowContext(
		r.Context(),
		`SELECT id, booking_id, seat_id, seat_label, ticket_code, status, created_at 
		 FROM tickets WHERE ticket_code = $1 OR id = $1`,
		ticketIdent,
	).Scan(&tkt.ID, &tkt.BookingID, &seatID, &tkt.SeatLabel, &tkt.TicketCode, &tkt.Status, &tkt.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Ticket not found"})
			return
		}
		http.Error(w, fmt.Sprintf(`{"error":"Database query error: %v"}`, err), http.StatusInternalServerError)
		return
	}
	if seatID.Valid {
		tkt.SeatID = seatID.String
	}

	// 2. Validate ticket eligibility
	if tkt.Status == "USED" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Cannot transfer a ticket that has already been redeemed and checked in",
		})
		return
	}
	if tkt.Status == "CANCELLED" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Cannot transfer a ticket from a cancelled booking",
		})
		return
	}
	if tkt.Status == "TRANSFERRED" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Ticket has already been transferred to another attendee",
		})
		return
	}

	// 3. Query source booking details
	var b Booking
	var tierID, resID, idemKey, curr sql.NullString
	var seatsRaw string
	err = s.db.QueryRowContext(
		r.Context(),
		`SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, idempotency_key, currency, created_at, updated_at 
		 FROM bookings WHERE id = $1`,
		tkt.BookingID,
	).Scan(
		&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
		&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
		&b.Status, &resID, &b.PaymentStatus, &idemKey, &curr, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to retrieve parent booking: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if b.Status == "CANCELLED" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Associated booking has been cancelled",
		})
		return
	}

	// Prevent self-transfer
	if strings.EqualFold(b.CustomerEmail, recipientEmail) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Cannot transfer ticket to the same email address as the current ticket holder",
		})
		return
	}

	// 4. Generate new ticket and booking records
	transferID := generateTransferToken("xfer")
	newBookingID := generateTransferToken("bkg-xfer")
	newBookingRef := fmt.Sprintf("REF-XFER-%s", time.Now().Format("15040500"))
	newTicketID := generateTransferToken("tkt-xfer")
	newTicketCode := fmt.Sprintf("TKT-XFER-%s", time.Now().Format("060102150405"))

	currencyCode := "USD"
	if curr.Valid && curr.String != "" {
		currencyCode = curr.String
	}

	recipientSeatJSON := "[]"
	if tkt.SeatID != "" {
		recipientSeatJSON = fmt.Sprintf("[\"%s\"]", tkt.SeatID)
	}

	// 5. Execute transfer atomically inside database transaction
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to start database transaction: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// 5a. Invalidate original ticket
	_, err = tx.ExecContext(
		r.Context(),
		`UPDATE tickets SET status = 'TRANSFERRED' WHERE id = $1`,
		tkt.ID,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to invalidate source ticket: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// 5b. Create recipient booking passbook record
	_, err = tx.ExecContext(
		r.Context(),
		`INSERT INTO bookings (
			id, booking_reference, user_id, customer_name, customer_email, 
			event_id, event_title, tier_id, quantity, seat_ids, 
			total_amount, status, payment_status, currency, 
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, $9, 0.00, 'CONFIRMED', 'PAID', $10, NOW(), NOW())`,
		newBookingID, newBookingRef, "", recipientName, recipientEmail,
		b.EventID, b.EventTitle, tierID, recipientSeatJSON, currencyCode,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to create recipient booking: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// 5c. Create recipient ticket with new secure ticket code
	_, err = tx.ExecContext(
		r.Context(),
		`INSERT INTO tickets (id, booking_id, seat_id, seat_label, ticket_code, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, 'VALID', NOW())`,
		newTicketID, newBookingID, tkt.SeatID, tkt.SeatLabel, newTicketCode,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to create recipient ticket: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// 5d. Record transfer audit log
	fromUserID := req.FromUserID
	if fromUserID == "" {
		fromUserID = b.UserID
	}
	_, err = tx.ExecContext(
		r.Context(),
		`INSERT INTO ticket_transfers (
			id, original_ticket_id, original_ticket_code, new_ticket_id, new_ticket_code,
			booking_id, new_booking_id, from_user_id, from_customer_email, recipient_email, recipient_name, note, status, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 'COMPLETED', NOW())`,
		transferID, tkt.ID, tkt.TicketCode, newTicketID, newTicketCode,
		tkt.BookingID, newBookingID, fromUserID, b.CustomerEmail, recipientEmail, recipientName, req.Note,
	)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Failed to record transfer log: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Transaction commit error: %v"}`, err), http.StatusInternalServerError)
		return
	}

	resp := TicketTransferResponse{
		TransferID:     transferID,
		BookingID:      tkt.BookingID,
		NewBookingID:   newBookingID,
		OldTicketID:    tkt.ID,
		NewTicketID:    newTicketID,
		OldTicketCode:  tkt.TicketCode,
		NewTicketCode:  newTicketCode,
		RecipientEmail: recipientEmail,
		RecipientName:  recipientName,
		SeatLabel:      tkt.SeatLabel,
		Status:         "COMPLETED",
		TransferredAt:  time.Now(),
		Message:        fmt.Sprintf("Ticket successfully transferred to %s (%s). The original QR code is now permanently disabled.", recipientName, recipientEmail),
	}

	// Dispatch In-App Notification to Recipient (Milestone 28)
	go func(r TicketTransferResponse) {
		notifData, _ := json.Marshal(map[string]interface{}{
			"transferId":     r.TransferID,
			"bookingId":      r.NewBookingID,
			"newTicketCode":  r.NewTicketCode,
			"seatLabel":      r.SeatLabel,
			"recipientEmail": r.RecipientEmail,
		})
		_ = s.dispatchNotification(context.Background(), Notification{
			RecipientEmail: r.RecipientEmail,
			Type:           "TICKET_TRANSFERRED",
			Title:          "Ticket Received!",
			Message:        fmt.Sprintf("You received a digital ticket for seat %s! Ticket code: %s", r.SeatLabel, r.NewTicketCode),
			Data:           notifData,
		})
	}(resp)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleListTransfers(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	if s.db == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]TicketTransferRecord{})
		return
	}

	email := strings.TrimSpace(r.URL.Query().Get("email"))
	bookingID := strings.TrimSpace(r.URL.Query().Get("bookingId"))

	query := `SELECT id, original_ticket_id, original_ticket_code, new_ticket_id, new_ticket_code, booking_id, new_booking_id, COALESCE(from_user_id, ''), COALESCE(from_customer_email, ''), recipient_email, recipient_name, COALESCE(note, ''), status, created_at FROM ticket_transfers`
	var conditions []string
	var args []interface{}

	if email != "" {
		args = append(args, email)
		conditions = append(conditions, fmt.Sprintf("(from_customer_email = $%d OR recipient_email = $%d)", len(args), len(args)))
	}
	if bookingID != "" {
		args = append(args, bookingID)
		conditions = append(conditions, fmt.Sprintf("(booking_id = $%d OR new_booking_id = $%d)", len(args), len(args)))
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT 50"

	rows, err := s.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Query error: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var records []TicketTransferRecord
	for rows.Next() {
		var rec TicketTransferRecord
		if err := rows.Scan(
			&rec.ID, &rec.OriginalTicketID, &rec.OriginalTicketCode, &rec.NewTicketID, &rec.NewTicketCode,
			&rec.BookingID, &rec.NewBookingID, &rec.FromUserID, &rec.FromCustomerEmail,
			&rec.RecipientEmail, &rec.RecipientName, &rec.Note, &rec.Status, &rec.CreatedAt,
		); err == nil {
			records = append(records, rec)
		}
	}
	if records == nil {
		records = []TicketTransferRecord{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(records)
}
