package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type WaitlistStatus string

const (
	WaitlistStatusPending   WaitlistStatus = "PENDING"
	WaitlistStatusPromoted  WaitlistStatus = "PROMOTED"
	WaitlistStatusClaimed   WaitlistStatus = "CLAIMED"
	WaitlistStatusExpired   WaitlistStatus = "EXPIRED"
	WaitlistStatusCancelled WaitlistStatus = "CANCELLED"
)

type WaitlistEntry struct {
	ID            string         `json:"id"`
	EventID       string         `json:"eventId"`
	TierID        string         `json:"tierId,omitempty"`
	SeatID        string         `json:"seatId,omitempty"`
	UserID        string         `json:"userId"`
	CustomerName  string         `json:"customerName"`
	CustomerEmail string         `json:"customerEmail"`
	Quantity      int            `json:"quantity"`
	Status        WaitlistStatus `json:"status"`
	ReservationID string         `json:"reservationId,omitempty"`
	PromotedAt    *time.Time     `json:"promotedAt,omitempty"`
	ExpiresAt     *time.Time     `json:"expiresAt,omitempty"`
	Position      int            `json:"position,omitempty"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

func (s *Server) initWaitlistSchema(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	query := `
	CREATE TABLE IF NOT EXISTS waitlist (
		id VARCHAR(64) PRIMARY KEY,
		event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		tier_id VARCHAR(64),
		seat_id VARCHAR(64),
		user_id VARCHAR(64) NOT NULL,
		customer_name VARCHAR(255) NOT NULL,
		customer_email VARCHAR(255) NOT NULL,
		quantity INT NOT NULL DEFAULT 1,
		status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
		reservation_id VARCHAR(64),
		promoted_at TIMESTAMP WITH TIME ZONE,
		expires_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_waitlist_event_status_created ON waitlist(event_id, status, created_at ASC);
	CREATE INDEX IF NOT EXISTS idx_waitlist_user ON waitlist(user_id);
	CREATE INDEX IF NOT EXISTS idx_waitlist_reservation ON waitlist(reservation_id);
	`
	_, err := s.db.ExecContext(ctx, query)
	return err
}

func (s *Server) CreateWaitlistEntry(ctx context.Context, entry *WaitlistEntry) (*WaitlistEntry, error) {
	if s.db == nil {
		return nil, errors.New("database not available")
	}
	if entry.EventID == "" {
		return nil, errors.New("eventId is required")
	}
	if entry.UserID == "" {
		return nil, errors.New("userId is required")
	}
	if entry.CustomerName == "" {
		return nil, errors.New("customerName is required")
	}
	if entry.CustomerEmail == "" {
		return nil, errors.New("customerEmail is required")
	}
	if entry.Quantity <= 0 {
		entry.Quantity = 1
	}
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("wl-%d", time.Now().UnixNano())
	}
	if entry.Status == "" {
		entry.Status = WaitlistStatusPending
	}
	now := time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now

	query := `
	INSERT INTO waitlist (
		id, event_id, tier_id, seat_id, user_id, customer_name, customer_email,
		quantity, status, reservation_id, promoted_at, expires_at, created_at, updated_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	RETURNING created_at, updated_at
	`
	err := s.db.QueryRowContext(ctx, query,
		entry.ID, entry.EventID, entry.TierID, entry.SeatID, entry.UserID,
		entry.CustomerName, entry.CustomerEmail, entry.Quantity, string(entry.Status),
		entry.ReservationID, entry.PromotedAt, entry.ExpiresAt, entry.CreatedAt, entry.UpdatedAt,
	).Scan(&entry.CreatedAt, &entry.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert waitlist entry: %w", err)
	}

	pos, err := s.GetWaitlistPosition(ctx, entry)
	if err == nil {
		entry.Position = pos
	}

	return entry, nil
}

func (s *Server) GetWaitlistEntry(ctx context.Context, id string) (*WaitlistEntry, error) {
	if s.db == nil {
		return nil, errors.New("database not available")
	}
	query := `
	SELECT id, event_id, tier_id, seat_id, user_id, customer_name, customer_email,
	       quantity, status, reservation_id, promoted_at, expires_at, created_at, updated_at
	FROM waitlist
	WHERE id = $1
	`
	var e WaitlistEntry
	var tierID, seatID, resID sql.NullString
	var promotedAt, expiresAt sql.NullTime
	var statusStr string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.EventID, &tierID, &seatID, &e.UserID, &e.CustomerName, &e.CustomerEmail,
		&e.Quantity, &statusStr, &resID, &promotedAt, &expiresAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("waitlist entry not found")
		}
		return nil, err
	}

	e.Status = WaitlistStatus(statusStr)
	if tierID.Valid {
		e.TierID = tierID.String
	}
	if seatID.Valid {
		e.SeatID = seatID.String
	}
	if resID.Valid {
		e.ReservationID = resID.String
	}
	if promotedAt.Valid {
		e.PromotedAt = &promotedAt.Time
	}
	if expiresAt.Valid {
		e.ExpiresAt = &expiresAt.Time
	}

	pos, err := s.GetWaitlistPosition(ctx, &e)
	if err == nil {
		e.Position = pos
	}

	return &e, nil
}

func (s *Server) GetWaitlistPosition(ctx context.Context, entry *WaitlistEntry) (int, error) {
	if s.db == nil {
		return 0, errors.New("database not available")
	}
	if entry.Status == WaitlistStatusPromoted {
		return 1, nil
	}
	if entry.Status != WaitlistStatusPending {
		return 0, nil
	}

	var count int
	var err error
	if entry.TierID != "" {
		query := `
		SELECT COUNT(*)
		FROM waitlist
		WHERE event_id = $1
		  AND (tier_id = $2 OR tier_id IS NULL OR tier_id = '')
		  AND status = 'PENDING'
		  AND (created_at < $3 OR (created_at = $3 AND id < $4))
		`
		err = s.db.QueryRowContext(ctx, query, entry.EventID, entry.TierID, entry.CreatedAt, entry.ID).Scan(&count)
	} else {
		query := `
		SELECT COUNT(*)
		FROM waitlist
		WHERE event_id = $1
		  AND status = 'PENDING'
		  AND (created_at < $2 OR (created_at = $2 AND id < $3))
		`
		err = s.db.QueryRowContext(ctx, query, entry.EventID, entry.CreatedAt, entry.ID).Scan(&count)
	}

	if err != nil {
		return 0, err
	}
	return count + 1, nil
}

func (s *Server) FindNextWaitlistEntryForPromotion(ctx context.Context, eventID string, tierID string) (*WaitlistEntry, error) {
	if s.db == nil {
		return nil, errors.New("database not available")
	}

	query := `
	SELECT id, event_id, tier_id, seat_id, user_id, customer_name, customer_email,
	       quantity, status, reservation_id, promoted_at, expires_at, created_at, updated_at
	FROM waitlist
	WHERE event_id = $1
	  AND status = 'PENDING'
	  AND ($2 = '' OR tier_id IS NULL OR tier_id = '' OR tier_id = $2)
	ORDER BY created_at ASC, id ASC
	LIMIT 1
	`
	var e WaitlistEntry
	var tID, seatID, resID sql.NullString
	var promotedAt, expiresAt sql.NullTime
	var statusStr string

	err := s.db.QueryRowContext(ctx, query, eventID, tierID).Scan(
		&e.ID, &e.EventID, &tID, &seatID, &e.UserID, &e.CustomerName, &e.CustomerEmail,
		&e.Quantity, &statusStr, &resID, &promotedAt, &expiresAt, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No pending entries
		}
		return nil, err
	}

	e.Status = WaitlistStatus(statusStr)
	if tID.Valid {
		e.TierID = tID.String
	}
	if seatID.Valid {
		e.SeatID = seatID.String
	}
	if resID.Valid {
		e.ReservationID = resID.String
	}
	if promotedAt.Valid {
		e.PromotedAt = &promotedAt.Time
	}
	if expiresAt.Valid {
		e.ExpiresAt = &expiresAt.Time
	}
	e.Position = 1

	return &e, nil
}

func (s *Server) PromoteWaitlistEntry(ctx context.Context, waitlistID string, reservationID string, expiresAt time.Time) error {
	if s.db == nil {
		return errors.New("database not available")
	}
	query := `
	UPDATE waitlist
	SET status = 'PROMOTED',
	    reservation_id = $1,
	    promoted_at = NOW(),
	    expires_at = $2,
	    updated_at = NOW()
	WHERE id = $3 AND status = 'PENDING'
	`
	res, err := s.db.ExecContext(ctx, query, reservationID, expiresAt, waitlistID)
	if err != nil {
		return fmt.Errorf("failed to promote waitlist entry: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("waitlist entry not found or not in PENDING status")
	}
	return nil
}

func (s *Server) CancelWaitlistEntry(ctx context.Context, waitlistID string, userID string) error {
	if s.db == nil {
		return errors.New("database not available")
	}
	var query string
	var res sql.Result
	var err error

	if userID != "" && userID != "admin" {
		query = `
		UPDATE waitlist
		SET status = 'CANCELLED', updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND status IN ('PENDING', 'PROMOTED')
		`
		res, err = s.db.ExecContext(ctx, query, waitlistID, userID)
	} else {
		query = `
		UPDATE waitlist
		SET status = 'CANCELLED', updated_at = NOW()
		WHERE id = $1 AND status IN ('PENDING', 'PROMOTED')
		`
		res, err = s.db.ExecContext(ctx, query, waitlistID)
	}
	if err != nil {
		return fmt.Errorf("failed to cancel waitlist entry: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("waitlist entry not found or already terminal")
	}
	return nil
}

func (s *Server) ListWaitlistEntries(ctx context.Context, eventID, userID, status string) ([]*WaitlistEntry, error) {
	if s.db == nil {
		return nil, errors.New("database not available")
	}

	var conditions []string
	var args []interface{}
	argIdx := 1

	if eventID != "" {
		conditions = append(conditions, fmt.Sprintf("event_id = $%d", argIdx))
		args = append(args, eventID)
		argIdx++
	}
	if userID != "" {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, userID)
		argIdx++
	}
	if status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, status)
		argIdx++
	}

	query := `
	SELECT id, event_id, tier_id, seat_id, user_id, customer_name, customer_email,
	       quantity, status, reservation_id, promoted_at, expires_at, created_at, updated_at
	FROM waitlist
	`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at ASC, id ASC"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*WaitlistEntry
	for rows.Next() {
		var e WaitlistEntry
		var tierID, seatID, resID sql.NullString
		var promotedAt, expiresAt sql.NullTime
		var statusStr string

		if err := rows.Scan(
			&e.ID, &e.EventID, &tierID, &seatID, &e.UserID, &e.CustomerName, &e.CustomerEmail,
			&e.Quantity, &statusStr, &resID, &promotedAt, &expiresAt, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}

		e.Status = WaitlistStatus(statusStr)
		if tierID.Valid {
			e.TierID = tierID.String
		}
		if seatID.Valid {
			e.SeatID = seatID.String
		}
		if resID.Valid {
			e.ReservationID = resID.String
		}
		if promotedAt.Valid {
			e.PromotedAt = &promotedAt.Time
		}
		if expiresAt.Valid {
			e.ExpiresAt = &expiresAt.Time
		}

		entries = append(entries, &e)
	}

	for _, entry := range entries {
		pos, _ := s.GetWaitlistPosition(ctx, entry)
		entry.Position = pos
	}

	return entries, nil
}

type CreateWaitlistRequest struct {
	EventID          string `json:"eventId"`
	EventIDAlt       string `json:"event_id"`
	TierID           string `json:"tierId"`
	TierIDAlt        string `json:"tier_id"`
	SeatID           string `json:"seatId"`
	SeatIDAlt        string `json:"seat_id"`
	UserID           string `json:"userId"`
	UserIDAlt        string `json:"user_id"`
	CustomerName     string `json:"customerName"`
	CustomerNameAlt  string `json:"customer_name"`
	CustomerEmail    string `json:"customerEmail"`
	CustomerEmailAlt string `json:"customer_email"`
	Quantity         int    `json:"quantity"`
}

func (s *Server) handleWaitlist(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.Method {
	case http.MethodPost:
		s.createWaitlist(w, r)
	case http.MethodGet:
		s.listWaitlists(w, r)
	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (s *Server) createWaitlist(w http.ResponseWriter, r *http.Request) {
	var req CreateWaitlistRequest
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

	tierID := req.TierID
	if tierID == "" {
		tierID = req.TierIDAlt
	}

	seatID := req.SeatID
	if seatID == "" {
		seatID = req.SeatIDAlt
	}

	userID := req.UserID
	if userID == "" {
		userID = req.UserIDAlt
	}

	customerName := req.CustomerName
	if customerName == "" {
		customerName = req.CustomerNameAlt
	}

	customerEmail := req.CustomerEmail
	if customerEmail == "" {
		customerEmail = req.CustomerEmailAlt
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		if claims, err := s.validateJWT(authHeader); err == nil && claims != nil {
			if userID == "" || userID == "usr-guest" {
				userID = claims.UserID
			}
			if customerName == "" || customerName == "Valued Guest" {
				customerName = claims.Name
			}
			if customerEmail == "" || customerEmail == "guest@example.com" {
				customerEmail = claims.Email
			}
		}
	}
	if xUID := r.Header.Get("X-User-ID"); xUID != "" && (userID == "" || userID == "usr-guest") {
		userID = xUID
	}
	if xEmail := r.Header.Get("X-User-Email"); xEmail != "" && (customerEmail == "" || customerEmail == "guest@example.com") {
		customerEmail = xEmail
	}
	if xName := r.Header.Get("X-User-Name"); xName != "" && (customerName == "" || customerName == "Valued Guest") {
		customerName = xName
	}

	if userID == "" {
		userID = "usr-guest"
	}
	if customerName == "" {
		customerName = "Valued Customer"
	}
	if customerEmail == "" {
		customerEmail = "guest@example.com"
	}

	qty := req.Quantity
	if qty <= 0 {
		qty = 1
	}

	entry := &WaitlistEntry{
		EventID:       eventID,
		TierID:        tierID,
		SeatID:        seatID,
		UserID:        userID,
		CustomerName:  customerName,
		CustomerEmail: customerEmail,
		Quantity:      qty,
		Status:        WaitlistStatusPending,
	}

	created, err := s.CreateWaitlistEntry(r.Context(), entry)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) listWaitlists(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	eventID := q.Get("eventId")
	if eventID == "" {
		eventID = q.Get("event_id")
	}
	userID := q.Get("userId")
	if userID == "" {
		userID = q.Get("user_id")
	}
	status := q.Get("status")

	entries, err := s.ListWaitlistEntries(r.Context(), eventID, userID, status)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if entries == nil {
		entries = []*WaitlistEntry{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(entries)
}

func (s *Server) handleWaitlistByID(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/waitlist/")
	path = strings.TrimPrefix(path, "/waitlist/")

	if strings.HasSuffix(path, "/claim") {
		s.handleWaitlistClaim(w, r)
		return
	}

	if strings.HasSuffix(path, "/cancel") {
		id := strings.TrimSuffix(path, "/cancel")
		s.cancelWaitlistByID(w, r, strings.TrimSpace(id))
		return
	}

	id := strings.TrimSpace(path)
	if id == "" {
		http.Error(w, `{"error":"Waitlist ID is required"}`, http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		entry, err := s.GetWaitlistEntry(r.Context(), id)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entry)

	case http.MethodDelete:
		s.cancelWaitlistByID(w, r, id)

	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (s *Server) cancelWaitlistByID(w http.ResponseWriter, r *http.Request, id string) {
	userID := ""
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		if claims, err := s.validateJWT(authHeader); err == nil && claims != nil {
			userID = claims.UserID
		}
	}
	if xUID := r.Header.Get("X-User-ID"); xUID != "" {
		userID = xUID
	}

	err := s.CancelWaitlistEntry(r.Context(), id, userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"message": "Waitlist entry cancelled",
		"status":  "CANCELLED",
	})
}

func (s *Server) handleWaitlistClaim(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/waitlist/")
	path = strings.TrimPrefix(path, "/waitlist/")
	id := strings.TrimSuffix(path, "/claim")
	id = strings.TrimSpace(id)

	if id == "" {
		http.Error(w, `{"error":"Waitlist ID is required"}`, http.StatusBadRequest)
		return
	}

	entry, err := s.GetWaitlistEntry(r.Context(), id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Waitlist entry not found"})
		return
	}

	if entry.Status == WaitlistStatusClaimed {
		booking, bErr := s.getBookingByWaitlistID(entry.ID, entry.ReservationID)
		if bErr == nil && booking != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Idempotent-Replay", "true")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(booking)
			return
		}
	}

	if entry.Status != WaitlistStatusPromoted {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "NOT_PROMOTED",
			"message": fmt.Sprintf("Waitlist entry is not eligible for claim (status: %s)", entry.Status),
		})
		return
	}

	if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "HOLD_EXPIRED",
			"message": "Waitlist promotion hold has expired",
		})
		return
	}

	// Atomically mark entry CLAIMED to avoid double claiming
	res, err := s.db.ExecContext(r.Context(),
		`UPDATE waitlist SET status = 'CLAIMED', updated_at = NOW() WHERE id = $1 AND status = 'PROMOTED'`,
		entry.ID)
	if err != nil || res == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update waitlist entry status"})
		return
	}
	rowsAff, _ := res.RowsAffected()
	if rowsAff == 0 {
		booking, bErr := s.getBookingByWaitlistID(entry.ID, entry.ReservationID)
		if bErr == nil && booking != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Idempotent-Replay", "true")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(booking)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Waitlist entry already claimed"})
		return
	}

	var seats []string
	if entry.ReservationID != "" {
		var seatJSON string
		_ = s.db.QueryRowContext(r.Context(),
			`SELECT seat_ids FROM reservations WHERE reservation_id = $1`,
			entry.ReservationID).Scan(&seatJSON)
		if seatJSON != "" {
			_ = json.Unmarshal([]byte(seatJSON), &seats)
		}
	}
	if len(seats) == 0 && entry.SeatID != "" {
		seats = []string{entry.SeatID}
	}
	if len(seats) == 0 {
		seats = []string{"A1"}
	}

	eventTitle := entry.EventID
	var title string
	if err := s.db.QueryRowContext(r.Context(), "SELECT title FROM events WHERE id = $1", entry.EventID).Scan(&title); err == nil && title != "" {
		eventTitle = title
	}

	var price float64 = 60.0
	if entry.TierID != "" {
		var p float64
		if err := s.db.QueryRowContext(r.Context(), "SELECT price FROM ticket_tiers WHERE id = $1", entry.TierID).Scan(&p); err == nil && p > 0 {
			price = p
		}
	}
	totalAmount := price * float64(entry.Quantity)

	bookingID := fmt.Sprintf("bkg-wl-%d", time.Now().UnixNano())
	bookingRef := fmt.Sprintf("REF-WL-%d", time.Now().UnixNano())
	seatJSONBytes, _ := json.Marshal(seats)
	idempotencyKey := "waitlist-claim-" + entry.ID

	now := time.Now()
	_, err = s.db.ExecContext(r.Context(),
		`INSERT INTO bookings (
			id, booking_reference, user_id, customer_name, customer_email,
			event_id, event_title, tier_id, quantity, seat_ids, total_amount,
			status, reservation_id, payment_status, idempotency_key, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'CONFIRMED', $12, 'SUCCESS', $13, $14, $14)`,
		bookingID, bookingRef, entry.UserID, entry.CustomerName, entry.CustomerEmail,
		entry.EventID, eventTitle, entry.TierID, entry.Quantity, string(seatJSONBytes),
		totalAmount, entry.ReservationID, idempotencyKey, now,
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to create booking: %v", err)})
		return
	}

	var tickets []Ticket
	for i, seat := range seats {
		ticketID := fmt.Sprintf("tkt-%d-%d", time.Now().UnixNano(), i+1)
		ticketCode := fmt.Sprintf("TKT-%s-%s-%d", entry.EventID, seat, time.Now().UnixNano())
		seatLabel := fmt.Sprintf("Seat %s", seat)

		_, _ = s.db.ExecContext(r.Context(),
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

	if entry.ReservationID != "" {
		_, _ = s.db.ExecContext(r.Context(), "UPDATE reservations SET status = 'COMMITTED' WHERE reservation_id = $1", entry.ReservationID)
		_, _ = s.db.ExecContext(r.Context(), "UPDATE seat_items SET status = 'BOOKED' WHERE reservation_id = $1", entry.ReservationID)

		if s.httpClient != nil && s.inventoryServiceURL != "" {
			cPayload, _ := json.Marshal(map[string]string{"reservationId": entry.ReservationID})
			cReq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, s.inventoryServiceURL+"/inventory/commit", bytes.NewReader(cPayload))
			cReq.Header.Set("Content-Type", "application/json")
			_, _ = s.httpClient.Do(cReq)
		}
	}

	finalBooking, bErr := s.getBookingTx(bookingID)
	if bErr == nil && finalBooking != nil {
		finalBooking.Tickets = tickets
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(finalBooking)
		return
	}

	b := &Booking{
		ID:               bookingID,
		BookingID:        bookingID,
		BookingReference: bookingRef,
		UserID:           entry.UserID,
		CustomerName:     entry.CustomerName,
		CustomerEmail:    entry.CustomerEmail,
		EventID:          entry.EventID,
		EventTitle:       eventTitle,
		TierID:           entry.TierID,
		Quantity:         entry.Quantity,
		SeatIDs:          seats,
		TotalAmount:      totalAmount,
		TotalAmountAlt:   totalAmount,
		Status:           "CONFIRMED",
		ReservationID:    entry.ReservationID,
		PaymentStatus:    "SUCCESS",
		IdempotencyKey:   idempotencyKey,
		Tickets:          tickets,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(b)
}

func (s *Server) getBookingByWaitlistID(waitlistID, resID string) (*Booking, error) {
	var b Booking
	var seatsRaw string
	var tierID, rID, idemKey sql.NullString
	err := s.db.QueryRow(
		`SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, idempotency_key, created_at, updated_at 
		 FROM bookings WHERE idempotency_key = $1 OR (reservation_id = $2 AND reservation_id != '') LIMIT 1`,
		"waitlist-claim-"+waitlistID, resID,
	).Scan(
		&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
		&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
		&b.Status, &rID, &b.PaymentStatus, &idemKey, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if tierID.Valid {
		b.TierID = tierID.String
	}
	if rID.Valid {
		b.ReservationID = rID.String
	}
	if idemKey.Valid {
		b.IdempotencyKey = idemKey.String
	}
	json.Unmarshal([]byte(seatsRaw), &b.SeatIDs)
	b.BookingID = b.ID
	b.TotalAmountAlt = b.TotalAmount
	return &b, nil
}

