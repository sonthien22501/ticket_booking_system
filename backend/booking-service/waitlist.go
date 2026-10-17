package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
