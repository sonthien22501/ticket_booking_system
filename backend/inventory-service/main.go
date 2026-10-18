package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/lib/pq"
)

// DistributedLockManager manages distributed locks via Redis
type DistributedLockManager struct {
	rdb *redis.Client
}

func NewDistributedLockManager(rdb *redis.Client) *DistributedLockManager {
	return &DistributedLockManager{
		rdb: rdb,
	}
}

// AcquireLock attempts to acquire a lock for a specific key. Returns true if acquired.
func (dlm *DistributedLockManager) AcquireLock(ctx context.Context, key string, expiration time.Duration) bool {
	locked, err := dlm.rdb.SetNX(ctx, "lock:"+key, "1", expiration).Result()
	if err != nil {
		log.Printf("Redis lock error: %v", err)
		return false
	}
	return locked
}

// ReleaseLock releases the distributed lock
func (dlm *DistributedLockManager) ReleaseLock(ctx context.Context, key string) {
	dlm.rdb.Del(ctx, "lock:"+key)
}

type SeatItem struct {
	ID            string     `json:"id"`
	EventID       string     `json:"eventId"`
	TierID        string     `json:"tierId"`
	SeatRow       string     `json:"row"`
	SeatNumber    int        `json:"number"`
	Status        string     `json:"status"` // AVAILABLE, HOLD, BOOKED, BLOCKED
	ReservationID *string    `json:"reservationId,omitempty"`
	HoldExpiresAt *time.Time `json:"holdExpiresAt,omitempty"`
}

type TierInventorySummary struct {
	TierID         string `json:"tier_id"`
	TierIDAlt      string `json:"tierId"`
	TierName       string `json:"tier_name"`
	TierNameAlt    string `json:"tierName"`
	TotalSeats     int    `json:"total_seats"`
	TotalSeatsAlt  int    `json:"totalSeats"`
	AvailableSeats int    `json:"available_seats"`
	AvailSeatsAlt  int    `json:"availableSeats"`
	ReservedSeats  int    `json:"reserved_seats"`
	BookedSeats    int    `json:"booked_seats"`
}

type InventoryResponse struct {
	EventID        string                 `json:"eventId"`
	EventIDAlt     string                 `json:"event_id"`
	TotalSeats     int                    `json:"totalSeats"`
	TotalSeatsAlt  int                    `json:"total_seats"`
	AvailableSeats int                    `json:"availableSeats"`
	AvailSeatsAlt  int                    `json:"available_seats"`
	ReservedSeats  int                    `json:"reservedSeats"`
	BookedSeats    int                    `json:"bookedSeats"`
	Summary        map[string]interface{} `json:"summary"`
	Tiers          []TierInventorySummary `json:"tiers"`
}

type ReserveRequest struct {
	EventID        string   `json:"eventId"`
	EventIDAlt     string   `json:"event_id"`
	TierID         string   `json:"tierId"`
	TierIDAlt      string   `json:"tier_id"`
	Seats          []string `json:"seats"`
	SeatIDsAlt     []string `json:"seat_ids"`
	TicketCount    int      `json:"ticketCount"`
	QuantityAlt    int      `json:"quantity"`
	HoldSeconds    int      `json:"holdSeconds"`
	HoldSecondsAlt int      `json:"hold_seconds"`
}

type ReserveResponse struct {
	ReservationID    string   `json:"reservationId"`
	ReservationIDAlt string   `json:"reservation_id"`
	EventID          string   `json:"eventId"`
	Status           string   `json:"status"`
	ExpiresAt        string   `json:"expiresAt"`
	ExpiresAtAlt     string   `json:"expires_at"`
	Seats            []string `json:"seats"`
	Quantity         int      `json:"quantity"`
}

type CommitRequest struct {
	ReservationID    string `json:"reservationId"`
	ReservationIDAlt string `json:"reservation_id"`
	BookingID        string `json:"bookingId"`
	BookingIDAlt     string `json:"booking_id"`
}

type ReleaseRequest struct {
	ReservationID    string `json:"reservationId"`
	ReservationIDAlt string `json:"reservation_id"`
	Reason           string `json:"reason,omitempty"`
}

type CancelRequest struct {
	ReservationID    string   `json:"reservationId"`
	ReservationIDAlt string   `json:"reservation_id"`
	BookingID        string   `json:"bookingId"`
	BookingIDAlt     string   `json:"booking_id"`
	EventID          string   `json:"eventId"`
	EventIDAlt       string   `json:"event_id"`
	TierID           string   `json:"tierId"`
	TierIDAlt        string   `json:"tier_id"`
	Quantity         int      `json:"quantity"`
	SeatIDs          []string `json:"seatIds"`
	SeatIDsAlt       []string `json:"seat_ids"`
	Reason           string   `json:"reason,omitempty"`
}

type Server struct {
	db        *sql.DB
	rdb       *redis.Client
	rmq       *RabbitMQ
	lockMgr   *DistributedLockManager
	stopChan  chan struct{}
	startTime time.Time
	reqCount  int64
}

func initRedis() (*redis.Client, error) {
	redisHost := getEnv("REDIS_HOST", "redis")
	redisPort := getEnv("REDIS_PORT", "6379")

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("could not connect to redis: %w", err)
	}

	log.Printf("Inventory Service connected to redis successfully at %s:%s", redisHost, redisPort)
	return rdb, nil
}

func initDB() (*sql.DB, error) {
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgrespassword")
	dbName := getEnv("DB_NAME", "ticket_db")

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	var db *sql.DB
	var err error

	for i := 1; i <= 30; i++ {
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Printf("Inventory Service connected to database successfully at %s:%s", dbHost, dbPort)
				db.SetMaxOpenConns(50)
				db.SetMaxIdleConns(25)
				db.SetConnMaxLifetime(5 * time.Minute)
				return db, nil
			}
		}
		log.Printf("Waiting for database connection (attempt %d/30): %v", i, err)
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("could not connect to database after 30 attempts: %w", err)
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

// Background Lease Janitor: cleans up expired holds every 3 seconds
func (s *Server) startJanitor() {
	ticker := time.NewTicker(3 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				s.sweepExpiredHolds()
			case <-s.stopChan:
				ticker.Stop()
				return
			}
		}
	}()
}

func (s *Server) sweepExpiredHolds() int {
	if s.db == nil {
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Find expired reservations in HOLD status
	rows, err := s.db.QueryContext(ctx,
		`SELECT reservation_id, event_id, tier_id, quantity, seat_ids 
		 FROM reservations 
		 WHERE status = 'HOLD' AND expires_at < NOW()`)
	if err != nil {
		return 0
	}
	defer rows.Close()

	type ExpiredRes struct {
		resID   string
		eventID string
		tierID  sql.NullString
		qty     int
		seatIDs []string
	}

	var expiredList []ExpiredRes
	for rows.Next() {
		var exp ExpiredRes
		var seatsRaw string
		if err := rows.Scan(&exp.resID, &exp.eventID, &exp.tierID, &exp.qty, &seatsRaw); err == nil {
			_ = json.Unmarshal([]byte(seatsRaw), &exp.seatIDs)
			expiredList = append(expiredList, exp)
		}
	}

	reclaimedCount := 0
	for _, exp := range expiredList {
		// Acquire event lock to avoid racing with active reservations
		if !s.lockMgr.AcquireLock(ctx, exp.eventID, 5*time.Second) {
			continue // Skip if another node is processing this event
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err == nil {
			_, _ = tx.ExecContext(ctx,
				`UPDATE reservations SET status = 'EXPIRED' WHERE reservation_id = $1 AND status = 'HOLD'`,
				exp.resID)

			_, _ = tx.ExecContext(ctx,
				`UPDATE seat_items 
				 SET status = 'AVAILABLE', reservation_id = NULL, hold_expires_at = NULL, updated_at = NOW() 
				 WHERE reservation_id = $1 AND status = 'HOLD'`,
				exp.resID)

			if exp.tierID.Valid && exp.tierID.String != "" {
				_, _ = tx.ExecContext(ctx,
					`UPDATE event_inventory 
					 SET available_seats = available_seats + $1, 
					     reserved_seats = GREATEST(0, reserved_seats - $1), 
					     version = version + 1, 
					     updated_at = NOW() 
					 WHERE event_id = $2 AND tier_id = $3`,
					exp.qty, exp.eventID, exp.tierID.String)
			}
			_ = tx.Commit()
			reclaimedCount++
			log.Printf("[Janitor] Reclaimed expired reservation %s for event %s (qty: %d)", exp.resID, exp.eventID, exp.qty)

			// Publish event across message broker
			if s.rmq != nil {
				evtPayload := map[string]interface{}{
					"reservationId": exp.resID,
					"eventId":       exp.eventID,
					"tierId":        exp.tierID.String,
					"seatIds":       exp.seatIDs,
					"quantity":      exp.qty,
					"status":        "EXPIRED",
					"reason":        "HOLD_TIMEOUT",
				}
				if out, errMarshal := json.Marshal(evtPayload); errMarshal == nil {
					_ = s.rmq.Publish("inventory.reservation_expired", out)
				}
			}
		}
		s.lockMgr.ReleaseLock(ctx, exp.eventID)
	}

	// 2. Also sweep any orphan seat items that expired without reservation linkage
	_, _ = s.db.ExecContext(ctx,
		`UPDATE seat_items 
		 SET status = 'AVAILABLE', reservation_id = NULL, hold_expires_at = NULL, updated_at = NOW() 
		 WHERE status = 'HOLD' AND hold_expires_at < NOW()`)

	return reclaimedCount
}

func (s *Server) handleGetReservation(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/inventory/reservations/")
	path = strings.TrimPrefix(path, "/inventory/reservations/")
	resID := strings.TrimSpace(path)

	if resID == "" {
		http.Error(w, `{"error":"Reservation ID is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var eventID, status, seatsRaw string
	var tierID sql.NullString
	var qty int
	var expiresAt, createdAt time.Time

	err := s.db.QueryRowContext(ctx,
		`SELECT reservation_id, event_id, tier_id, seat_ids, quantity, status, expires_at, created_at 
		 FROM reservations 
		 WHERE reservation_id = $1`,
		resID,
	).Scan(&resID, &eventID, &tierID, &seatsRaw, &qty, &status, &expiresAt, &createdAt)

	if err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "RESERVATION_NOT_FOUND",
			"message": "Reservation not found",
		})
		return
	} else if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Database error: %v"}`, err), http.StatusInternalServerError)
		return
	}

	var seatIDs []string
	_ = json.Unmarshal([]byte(seatsRaw), &seatIDs)

	now := time.Now()
	remaining := int(expiresAt.Sub(now).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"reservationId":    resID,
		"eventId":          eventID,
		"tierId":           tierID.String,
		"seatIds":          seatIDs,
		"quantity":         qty,
		"status":           status,
		"expiresAt":        expiresAt.Format(time.RFC3339),
		"createdAt":        createdAt.Format(time.RFC3339),
		"remainingSeconds": remaining,
		"isExpired":        now.After(expiresAt) && status == "HOLD",
	})
}

func (s *Server) handleJanitorSweep(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	reclaimed := s.sweepExpiredHolds()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "SUCCESS",
		"reclaimedCount": reclaimed,
		"message":        fmt.Sprintf("Sweep complete. Reclaimed %d expired reservations", reclaimed),
	})
}

// GET /inventory/{eventId} or /inventory/events/{eventId}
func (s *Server) handleGetInventory(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/inventory/events/")
	path = strings.TrimPrefix(path, "/api/inventory/")
	path = strings.TrimPrefix(path, "/inventory/events/")
	path = strings.TrimPrefix(path, "/inventory/")

	// If subpath is "seats", route to seats handler
	if strings.HasSuffix(path, "/seats") {
		s.handleGetSeats(w, r)
		return
	}

	eventID := strings.TrimSpace(path)
	if eventID == "" {
		http.Error(w, `{"error":"Event ID is required"}`, http.StatusBadRequest)
		return
	}

	// Query tier inventory
	rows, err := s.db.Query(
		`SELECT i.tier_id, t.name, i.total_seats, i.available_seats, i.reserved_seats, i.booked_seats
		 FROM event_inventory i
		 LEFT JOIN ticket_tiers t ON i.tier_id = t.id
		 WHERE i.event_id = $1`,
		eventID,
	)
	if err != nil {
		log.Printf("Query error in getInventory: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Database error: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tiers []TierInventorySummary
	totalSeats := 0
	availSeats := 0
	reservedSeats := 0
	bookedSeats := 0

	for rows.Next() {
		var t TierInventorySummary
		var tName sql.NullString
		if err := rows.Scan(&t.TierID, &tName, &t.TotalSeats, &t.AvailableSeats, &t.ReservedSeats, &t.BookedSeats); err == nil {
			t.TierIDAlt = t.TierID
			t.TierName = tName.String
			t.TierNameAlt = tName.String
			t.TotalSeatsAlt = t.TotalSeats
			t.AvailSeatsAlt = t.AvailableSeats

			totalSeats += t.TotalSeats
			availSeats += t.AvailableSeats
			reservedSeats += t.ReservedSeats
			bookedSeats += t.BookedSeats
			tiers = append(tiers, t)
		}
	}

	if len(tiers) == 0 {
		// Check if event exists
		var exists bool
		_ = s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM events WHERE id = $1)", eventID).Scan(&exists)
		if !exists {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "NOT_FOUND",
				"message": fmt.Sprintf("Event %s not found", eventID),
			})
			return
		}
	}

	resp := InventoryResponse{
		EventID:        eventID,
		EventIDAlt:     eventID,
		TotalSeats:     totalSeats,
		TotalSeatsAlt:  totalSeats,
		AvailableSeats: availSeats,
		AvailSeatsAlt:  availSeats,
		ReservedSeats:  reservedSeats,
		BookedSeats:    bookedSeats,
		Summary: map[string]interface{}{
			"total_seats":     totalSeats,
			"totalSeats":      totalSeats,
			"available_seats": availSeats,
			"availableSeats":  availSeats,
			"reserved_seats":  reservedSeats,
			"booked_seats":    bookedSeats,
		},
		Tiers: tiers,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// GET /inventory/{eventId}/seats
func (s *Server) handleGetSeats(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := r.URL.Path
	path = strings.TrimPrefix(path, "/api/inventory/events/")
	path = strings.TrimPrefix(path, "/api/inventory/")
	path = strings.TrimPrefix(path, "/inventory/events/")
	path = strings.TrimPrefix(path, "/inventory/")
	path = strings.TrimSuffix(path, "/seats")
	eventID := strings.TrimSpace(path)

	tierID := r.URL.Query().Get("tier_id")
	if tierID == "" {
		tierID = r.URL.Query().Get("tierId")
	}

	query := `SELECT id, event_id, tier_id, seat_row, seat_number, status, reservation_id, hold_expires_at 
	          FROM seat_items WHERE event_id = $1`
	var args []interface{}
	args = append(args, eventID)

	if tierID != "" {
		query += " AND tier_id = $2"
		args = append(args, tierID)
	}
	query += " ORDER BY seat_row ASC, seat_number ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Printf("Query error in getSeats: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Database error: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var seats []SeatItem
	for rows.Next() {
		var s SeatItem
		var resID sql.NullString
		var expires sql.NullTime
		if err := rows.Scan(&s.ID, &s.EventID, &s.TierID, &s.SeatRow, &s.SeatNumber, &s.Status, &resID, &expires); err == nil {
			if resID.Valid {
				s.ReservationID = &resID.String
			}
			if expires.Valid {
				s.HoldExpiresAt = &expires.Time
			}
			seats = append(seats, s)
		}
	}

	if seats == nil {
		seats = []SeatItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"event_id": eventID,
		"eventId":  eventID,
		"seats":    seats,
	})
}

// POST /inventory/reserve
func (s *Server) handleReserve(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req ReserveRequest
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

	quantity := req.TicketCount
	if quantity <= 0 {
		quantity = req.QuantityAlt
	}
	if quantity <= 0 {
		if len(seats) > 0 {
			quantity = len(seats)
		} else {
			quantity = 1
		}
	}

	holdSeconds := req.HoldSeconds
	if holdSeconds <= 0 {
		holdSeconds = req.HoldSecondsAlt
	}
	if holdSeconds <= 0 {
		holdSeconds = 600 // 10 minutes default
	}

	tierID := req.TierID
	if tierID == "" {
		tierID = req.TierIDAlt
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// 1. Distributed serialization via Redis Mutex
	lockAcquired := false
	for i := 0; i < 20; i++ { // Retry for up to 2 seconds
		if s.lockMgr.AcquireLock(ctx, eventID, 5*time.Second) {
			lockAcquired = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !lockAcquired {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "SYSTEM_BUSY",
			"message": "The system is currently experiencing high traffic for this event. Please try again.",
		})
		return
	}
	defer s.lockMgr.ReleaseLock(context.Background(), eventID)

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		http.Error(w, `{"error":"Database error starting transaction"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// 2. Resolve target tier if not provided
	if tierID == "" {
		if len(seats) > 0 {
			// Infer tier from requested seats
			_ = tx.QueryRowContext(ctx,
				`SELECT tier_id FROM seat_items WHERE event_id = $1 AND (id = $2 OR (seat_row || seat_number) = $2) LIMIT 1`,
				eventID, seats[0]).Scan(&tierID)
		}
		if tierID == "" {
			_ = tx.QueryRowContext(ctx,
				`SELECT tier_id FROM event_inventory WHERE event_id = $1 ORDER BY available_seats DESC LIMIT 1`,
				eventID).Scan(&tierID)
		}
	}

	var reservedSeatIDs []string

	// 3. Handle discrete seat selection or auto-selection
	if len(seats) > 0 {
		// Specific seat reservation
		for _, seatReq := range seats {
			var seatID, currentStatus, seatTierID string
			var holdExpires sql.NullTime

			// Query with FOR UPDATE lock
			err := tx.QueryRowContext(ctx,
				`SELECT id, status, tier_id, hold_expires_at 
				 FROM seat_items 
				 WHERE event_id = $1 AND (id = $2 OR (seat_row || seat_number) = $2) 
				 FOR UPDATE`,
				eventID, seatReq,
			).Scan(&seatID, &currentStatus, &seatTierID, &holdExpires)

			if err == sql.ErrNoRows {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":             "SEAT_UNAVAILABLE",
					"message":           fmt.Sprintf("Seat %s not found in event %s", seatReq, eventID),
					"conflicting_seats": []string{seatReq},
				})
				return
			} else if err != nil {
				log.Printf("Seat lock error: %v", err)
				http.Error(w, `{"error":"Database lock failure"}`, http.StatusInternalServerError)
				return
			}

			// Check if seat is currently available or hold has expired
			isAvailable := (currentStatus == "AVAILABLE") ||
				(currentStatus == "HOLD" && holdExpires.Valid && holdExpires.Time.Before(time.Now()))

			if !isAvailable {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":             "SEAT_UNAVAILABLE",
					"message":           fmt.Sprintf("Seat %s is already reserved or booked", seatReq),
					"conflicting_seats": []string{seatReq},
				})
				return
			}

			if tierID == "" {
				tierID = seatTierID
			}
			reservedSeatIDs = append(reservedSeatIDs, seatID)
		}
	} else {
		// Auto-select requested quantity of available seats
		rows, err := tx.QueryContext(ctx,
			`SELECT id FROM seat_items 
			 WHERE event_id = $1 AND tier_id = $2 AND (status = 'AVAILABLE' OR (status = 'HOLD' AND hold_expires_at < NOW()))
			 LIMIT $3 FOR UPDATE SKIP LOCKED`,
			eventID, tierID, quantity)
		if err != nil {
			http.Error(w, `{"error":"Database error selecting seats"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var sid string
			if err := rows.Scan(&sid); err == nil {
				reservedSeatIDs = append(reservedSeatIDs, sid)
			}
		}

		if len(reservedSeatIDs) < quantity {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "SEAT_UNAVAILABLE",
				"message": "Not enough seats available for requested quantity",
			})
			return
		}
	}

	actualQty := len(reservedSeatIDs)
	if actualQty == 0 {
		actualQty = quantity
	}

	// 4. Atomic conditional inventory update on event_inventory
	var newAvail int
	res := tx.QueryRowContext(ctx,
		`UPDATE event_inventory 
		 SET available_seats = available_seats - $1, 
		     reserved_seats = reserved_seats + $1, 
		     version = version + 1, 
		     updated_at = NOW() 
		 WHERE event_id = $2 AND tier_id = $3 AND available_seats >= $1 
		 RETURNING available_seats`,
		actualQty, eventID, tierID,
	)
	if err := res.Scan(&newAvail); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "SEAT_UNAVAILABLE",
			"message": "Insufficient available inventory to reserve seats",
		})
		return
	}

	// 5. Generate reservation
	resID := fmt.Sprintf("res-%d-%s", time.Now().UnixNano(), eventID)
	expiresAt := time.Now().Add(time.Duration(holdSeconds) * time.Second)

	// Update seat items to HOLD
	if len(reservedSeatIDs) > 0 {
		_, err = tx.ExecContext(ctx,
			`UPDATE seat_items 
			 SET status = 'HOLD', reservation_id = $1, hold_expires_at = $2, updated_at = NOW() 
			 WHERE event_id = $3 AND id = ANY($4)`,
			resID, expiresAt, eventID, pq.Array(reservedSeatIDs),
		)
		if err != nil {
			log.Printf("Failed to update seat items to HOLD: %v", err)
			http.Error(w, `{"error":"Failed to update seats to hold"}`, http.StatusInternalServerError)
			return
		}
	}

	// Record reservation in table
	seatsJSON, _ := json.Marshal(reservedSeatIDs)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO reservations (reservation_id, event_id, tier_id, seat_ids, quantity, status, expires_at) 
		 VALUES ($1, $2, $3, $4, $5, 'HOLD', $6)`,
		resID, eventID, tierID, string(seatsJSON), actualQty, expiresAt,
	)
	if err != nil {
		log.Printf("Failed to insert reservation: %v", err)
		http.Error(w, `{"error":"Failed to create reservation record"}`, http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error":"Transaction commit failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ReserveResponse{
		ReservationID:    resID,
		ReservationIDAlt: resID,
		EventID:          eventID,
		Status:           "HOLD",
		ExpiresAt:        expiresAt.Format(time.RFC3339),
		ExpiresAtAlt:     expiresAt.Format(time.RFC3339),
		Seats:            reservedSeatIDs,
		Quantity:         actualQty,
	})
}

// POST /inventory/commit
func (s *Server) handleCommit(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req CommitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid request payload: %v"}`, err), http.StatusBadRequest)
		return
	}

	resID := req.ReservationID
	if resID == "" {
		resID = req.ReservationIDAlt
	}
	if resID == "" {
		http.Error(w, `{"error":"reservationId is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// Query reservation
	var eventID, status string
	var tierID sql.NullString
	var qty int
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT event_id, tier_id, quantity, status, expires_at FROM reservations WHERE reservation_id = $1`,
		resID,
	).Scan(&eventID, &tierID, &qty, &status, &expiresAt)

	if err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "RESERVATION_NOT_FOUND"})
		return
	} else if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Database error: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if status == "COMMITTED" {
		// Idempotent success
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":        "COMMITTED",
			"reservationId": resID,
		})
		return
	}

	if status != "HOLD" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "RESERVATION_INVALID_STATE",
			"message": fmt.Sprintf("Reservation status is %s", status),
		})
		return
	}

	if expiresAt.Before(time.Now()) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "RESERVATION_EXPIRED",
			"message": "Hold lease has expired",
		})
		return
	}

	// Acquire event lock
	lockAcquired := false
	for i := 0; i < 20; i++ {
		if s.lockMgr.AcquireLock(ctx, eventID, 5*time.Second) {
			lockAcquired = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !lockAcquired {
		http.Error(w, `{"error":"SYSTEM_BUSY", "message":"Could not acquire lock"}`, http.StatusConflict)
		return
	}
	defer s.lockMgr.ReleaseLock(context.Background(), eventID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, `{"error":"Transaction begin error"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Update reservation
	_, err = tx.ExecContext(ctx,
		`UPDATE reservations SET status = 'COMMITTED' WHERE reservation_id = $1`,
		resID)
	if err != nil {
		http.Error(w, `{"error":"Failed to update reservation"}`, http.StatusInternalServerError)
		return
	}

	// Update seats to BOOKED
	_, err = tx.ExecContext(ctx,
		`UPDATE seat_items 
		 SET status = 'BOOKED', hold_expires_at = NULL, updated_at = NOW() 
		 WHERE reservation_id = $1`,
		resID)
	if err != nil {
		http.Error(w, `{"error":"Failed to mark seats as booked"}`, http.StatusInternalServerError)
		return
	}

	// Update event_inventory counts
	if tierID.Valid && tierID.String != "" {
		_, err = tx.ExecContext(ctx,
			`UPDATE event_inventory 
			 SET reserved_seats = GREATEST(0, reserved_seats - $1), 
			     booked_seats = booked_seats + $1, 
			     version = version + 1, 
			     updated_at = NOW() 
			 WHERE event_id = $2 AND tier_id = $3`,
			qty, eventID, tierID.String)
		if err != nil {
			log.Printf("Error updating inventory counts during commit: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error":"Failed to commit transaction"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":        "COMMITTED",
		"reservationId": resID,
	})
}

// POST /inventory/release
func (s *Server) handleRelease(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req ReleaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid request payload: %v"}`, err), http.StatusBadRequest)
		return
	}

	resID := req.ReservationID
	if resID == "" {
		resID = req.ReservationIDAlt
	}
	if resID == "" {
		http.Error(w, `{"error":"reservationId is required"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var eventID, status string
	var tierID sql.NullString
	var qty int
	err := s.db.QueryRowContext(ctx,
		`SELECT event_id, tier_id, quantity, status FROM reservations WHERE reservation_id = $1`,
		resID,
	).Scan(&eventID, &tierID, &qty, &status)

	if err == sql.ErrNoRows {
		// Idempotent release
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":        "RELEASED",
			"reservationId": resID,
		})
		return
	} else if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Database error: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if status != "HOLD" {
		// Already released or committed
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":        status,
			"reservationId": resID,
		})
		return
	}

	lockAcquired := false
	for i := 0; i < 20; i++ {
		if s.lockMgr.AcquireLock(ctx, eventID, 5*time.Second) {
			lockAcquired = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !lockAcquired {
		http.Error(w, `{"error":"SYSTEM_BUSY", "message":"Could not acquire lock"}`, http.StatusConflict)
		return
	}
	defer s.lockMgr.ReleaseLock(context.Background(), eventID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, `{"error":"Transaction begin error"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE reservations SET status = 'RELEASED' WHERE reservation_id = $1 AND status = 'HOLD'`,
		resID)
	if err != nil {
		http.Error(w, `{"error":"Failed to update reservation"}`, http.StatusInternalServerError)
		return
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE seat_items 
		 SET status = 'AVAILABLE', reservation_id = NULL, hold_expires_at = NULL, updated_at = NOW() 
		 WHERE reservation_id = $1 AND status = 'HOLD'`,
		resID)
	if err != nil {
		http.Error(w, `{"error":"Failed to revert seat items"}`, http.StatusInternalServerError)
		return
	}

	if tierID.Valid && tierID.String != "" {
		_, err = tx.ExecContext(ctx,
			`UPDATE event_inventory 
			 SET available_seats = available_seats + $1, 
			     reserved_seats = GREATEST(0, reserved_seats - $1), 
			     version = version + 1, 
			     updated_at = NOW() 
			 WHERE event_id = $2 AND tier_id = $3`,
			qty, eventID, tierID.String)
		if err != nil {
			log.Printf("Error restoring inventory counts during release: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error":"Failed to commit transaction"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":        "RELEASED",
		"reservationId": resID,
	})
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req CancelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid request payload: %v"}`, err), http.StatusBadRequest)
		return
	}

	resID := req.ReservationID
	if resID == "" {
		resID = req.ReservationIDAlt
	}
	eventID := req.EventID
	if eventID == "" {
		eventID = req.EventIDAlt
	}
	tierIDStr := req.TierID
	if tierIDStr == "" {
		tierIDStr = req.TierIDAlt
	}
	qty := req.Quantity
	seatIDs := req.SeatIDs
	if len(seatIDs) == 0 {
		seatIDs = req.SeatIDsAlt
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var dbEventID, dbStatus string
	var dbTierID sql.NullString
	var dbQty int

	hasRes := false
	if resID != "" {
		err := s.db.QueryRowContext(ctx,
			`SELECT event_id, tier_id, quantity, status FROM reservations WHERE reservation_id = $1`,
			resID,
		).Scan(&dbEventID, &dbTierID, &dbQty, &dbStatus)
		if err == nil {
			hasRes = true
			if eventID == "" {
				eventID = dbEventID
			}
			if tierIDStr == "" && dbTierID.Valid {
				tierIDStr = dbTierID.String
			}
			if qty == 0 {
				qty = dbQty
			}
		}
	}

	// Idempotency: If already cancelled or released
	if hasRes && (dbStatus == "CANCELLED" || dbStatus == "RELEASED") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "CANCELLED",
			"reservationId": resID,
			"message":       "Reservation already cancelled",
		})
		return
	}

	if eventID == "" {
		http.Error(w, `{"error":"MISSING_EVENT_ID", "message":"eventId or valid reservationId is required"}`, http.StatusBadRequest)
		return
	}

	// Acquire event lock to prevent race conditions during seat restoration
	lockAcquired := false
	for i := 0; i < 20; i++ {
		if s.lockMgr.AcquireLock(ctx, eventID, 5*time.Second) {
			lockAcquired = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !lockAcquired {
		http.Error(w, `{"error":"SYSTEM_BUSY", "message":"Could not acquire lock"}`, http.StatusConflict)
		return
	}
	defer s.lockMgr.ReleaseLock(context.Background(), eventID)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, `{"error":"Transaction begin error"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if hasRes {
		_, err = tx.ExecContext(ctx,
			`UPDATE reservations SET status = 'CANCELLED' WHERE reservation_id = $1`,
			resID)
		if err != nil {
			http.Error(w, `{"error":"Failed to update reservation"}`, http.StatusInternalServerError)
			return
		}

		// Revert seats by reservation_id
		_, err = tx.ExecContext(ctx,
			`UPDATE seat_items 
			 SET status = 'AVAILABLE', reservation_id = NULL, hold_expires_at = NULL, updated_at = NOW() 
			 WHERE reservation_id = $1`,
			resID)
		if err != nil {
			http.Error(w, `{"error":"Failed to revert seat items"}`, http.StatusInternalServerError)
			return
		}
	}

	// Also revert seats by explicit seat IDs if supplied
	if len(seatIDs) > 0 {
		for _, seat := range seatIDs {
			_, _ = tx.ExecContext(ctx,
				`UPDATE seat_items 
				 SET status = 'AVAILABLE', reservation_id = NULL, hold_expires_at = NULL, updated_at = NOW() 
				 WHERE event_id = $1 AND (id = $2 OR seat_row || seat_number::text = $2 OR seat_number::text = $2)`,
				eventID, seat)
		}
	}

	// Restore event_inventory
	if tierIDStr != "" && qty > 0 {
		if dbStatus == "HOLD" {
			_, err = tx.ExecContext(ctx,
				`UPDATE event_inventory 
				 SET available_seats = available_seats + $1, 
				     reserved_seats = GREATEST(0, reserved_seats - $1), 
				     version = version + 1, 
				     updated_at = NOW() 
				 WHERE event_id = $2 AND tier_id = $3`,
				qty, eventID, tierIDStr)
		} else {
			// Committed or directly booked
			_, err = tx.ExecContext(ctx,
				`UPDATE event_inventory 
				 SET available_seats = available_seats + $1, 
				     booked_seats = GREATEST(0, booked_seats - $1), 
				     version = version + 1, 
				     updated_at = NOW() 
				 WHERE event_id = $2 AND tier_id = $3`,
				qty, eventID, tierIDStr)
		}
		if err != nil {
			log.Printf("Error restoring inventory counts during cancellation: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error":"Failed to commit transaction"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "CANCELLED",
		"reservationId": resID,
		"eventId":       eventID,
		"tierId":        tierIDStr,
		"seatIds":       seatIDs,
		"message":       "Reservation and seats cancelled successfully",
	})
}

func main() {
	port := getEnv("PORT", "8082")

	db, err := initDB()
	if err != nil {
		log.Fatalf("Fatal database initialization error: %v", err)
	}
	defer db.Close()

	rdb, err := initRedis()
	if err != nil {
		log.Fatalf("Fatal redis initialization error: %v", err)
	}
	defer rdb.Close()

	srv := &Server{
		db:        db,
		rdb:       rdb,
		lockMgr:   NewDistributedLockManager(rdb),
		stopChan:  make(chan struct{}),
		startTime: time.Now(),
	}

	// Start background janitor
	srv.startJanitor()

	mux := http.NewServeMux()

	// Health routes
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/api/health", srv.handleHealth)
	mux.HandleFunc("/api/inventory/health", srv.handleHealth)

	// Metrics routes (R23)
	mux.HandleFunc("/metrics", srv.handleMetrics)
	mux.HandleFunc("/api/metrics", srv.handleMetrics)
	mux.HandleFunc("/api/inventory/metrics", srv.handleMetrics)

	// Actions
	mux.HandleFunc("/inventory/reserve", srv.handleReserve)
	mux.HandleFunc("/api/inventory/reserve", srv.handleReserve)

	mux.HandleFunc("/inventory/commit", srv.handleCommit)
	mux.HandleFunc("/api/inventory/commit", srv.handleCommit)

	mux.HandleFunc("/inventory/release", srv.handleRelease)
	mux.HandleFunc("/api/inventory/release", srv.handleRelease)

	mux.HandleFunc("/inventory/cancel", srv.handleCancel)
	mux.HandleFunc("/api/inventory/cancel", srv.handleCancel)

	// Janitor manual sweep routes
	mux.HandleFunc("/inventory/janitor/sweep", srv.handleJanitorSweep)
	mux.HandleFunc("/api/inventory/janitor/sweep", srv.handleJanitorSweep)

	// Reservation inspection routes
	mux.HandleFunc("/inventory/reservations/", srv.handleGetReservation)
	mux.HandleFunc("/api/inventory/reservations/", srv.handleGetReservation)

	// Query routes
	mux.HandleFunc("/inventory/", srv.handleGetInventory)
	mux.HandleFunc("/api/inventory/", srv.handleGetInventory)

	rmq, err := InitRabbitMQ("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Printf("Warning: RabbitMQ init failed: %v", err)
	} else {
		defer rmq.Close()
		srv.rmq = rmq
		srv.startSagaWorker(rmq)
	}

	serverAddr := ":" + port
	log.Printf("Seat Inventory Service starting on %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, TracingMiddleware("inventory-service", mux)); err != nil {
		log.Fatalf("Server terminated: %v", err)
	}
}
