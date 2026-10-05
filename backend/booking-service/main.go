package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type PaymentInfo struct {
	Method    string `json:"method"`
	CardToken string `json:"cardToken"`
	CardTokenAlt string `json:"card_token"`
}

type CreateBookingRequest struct {
	EventID        string       `json:"eventId"`
	EventIDAlt     string       `json:"event_id"`
	UserID         string       `json:"userId"`
	UserIDAlt      string       `json:"user_id"`
	CustomerName   string       `json:"customerName"`
	CustomerNameAlt string      `json:"customer_name"`
	CustomerEmail  string       `json:"customerEmail"`
	CustomerEmailAlt string     `json:"customer_email"`
	TierID         string       `json:"tierId"`
	TierIDAlt      string       `json:"tier_id"`
	Seats          []string     `json:"seats"`
	SeatIDsAlt     []string     `json:"seat_ids"`
	TicketCount    int          `json:"ticketCount"`
	QuantityAlt    int          `json:"quantity"`
	PaymentInfo    *PaymentInfo `json:"paymentInfo"`
	PaymentInfoAlt *PaymentInfo `json:"payment_info"`
	IdempotencyKey string       `json:"idempotencyKey"`
}

type Ticket struct {
	ID         string    `json:"id"`
	TicketID   string    `json:"ticketId"`
	BookingID  string    `json:"bookingId"`
	SeatID     string    `json:"seatId"`
	SeatLabel  string    `json:"seatLabel"`
	TicketCode string    `json:"ticketCode"`
	Status     string    `json:"status"` // VALID, USED, CANCELLED
	CreatedAt  time.Time `json:"createdAt"`
}

type Booking struct {
	ID               string    `json:"id"`
	BookingID        string    `json:"bookingId"`
	BookingReference string    `json:"bookingReference"`
	UserID           string    `json:"userId"`
	CustomerName     string    `json:"customerName"`
	CustomerEmail    string    `json:"customerEmail"`
	EventID          string    `json:"eventId"`
	EventTitle       string    `json:"eventTitle"`
	TierID           string    `json:"tierId"`
	Quantity         int       `json:"quantity"`
	SeatIDs          []string  `json:"seatIds"`
	TotalAmount      float64   `json:"totalAmount"`
	TotalAmountAlt   float64   `json:"total_price"`
	Status           string    `json:"status"` // CONFIRMED, PENDING, CANCELLED
	ReservationID    string    `json:"reservationId"`
	PaymentStatus    string    `json:"paymentStatus"`
	Tickets          []Ticket  `json:"tickets"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type Server struct {
	db                  *sql.DB
	httpClient          *http.Client
	catalogServiceURL   string
	inventoryServiceURL string
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
				log.Printf("Booking Service connected to database successfully at %s:%s", dbHost, dbPort)
				db.SetMaxOpenConns(50)
				db.SetMaxIdleConns(20)
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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "UP",
		"service": "booking-service",
	})
}

func (s *Server) handleBookings(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.Method {
	case http.MethodPost:
		s.createBooking(w, r)
	case http.MethodGet:
		s.listBookings(w, r)
	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

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

	// Boundary check: large overbooking requests
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

	// 1. Validate Event & Fetch Event Details from Catalog Service (or fallback to DB)
	eventTitle, tierPrice := s.fetchEventInfo(eventID, tierID)

	// 2. SAGA STEP 1: Hold Seats in Inventory Service
	reservePayload := map[string]interface{}{
		"eventId":     eventID,
		"tierId":      tierID,
		"seats":       seats,
		"ticketCount": ticketCount,
		"holdSeconds": 600,
	}
	payloadBytes, _ := json.Marshal(reservePayload)

	reserveURL := fmt.Sprintf("%s/inventory/reserve", s.inventoryServiceURL)
	reserveReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, reserveURL, bytes.NewReader(payloadBytes))
	if err != nil {
		http.Error(w, `{"error":"Internal error creating reserve request"}`, http.StatusInternalServerError)
		return
	}
	reserveReq.Header.Set("Content-Type", "application/json")

	reserveResp, err := s.httpClient.Do(reserveReq)
	if err != nil {
		log.Printf("Call to inventory reserve failed: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Inventory service communication error: %v"}`, err), http.StatusServiceUnavailable)
		return
	}
	defer reserveResp.Body.Close()

	if reserveResp.StatusCode == http.StatusConflict {
		// Propagation of seat unavailable / conflict
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.Copy(w, reserveResp.Body)
		return
	}

	if reserveResp.StatusCode != http.StatusOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(reserveResp.StatusCode)
		_, _ = io.Copy(w, reserveResp.Body)
		return
	}

	var reserveResult struct {
		ReservationID string   `json:"reservationId"`
		Seats         []string `json:"seats"`
		Quantity      int      `json:"quantity"`
	}
	if err := json.NewDecoder(reserveResp.Body).Decode(&reserveResult); err != nil {
		http.Error(w, `{"error":"Invalid response from inventory service"}`, http.StatusInternalServerError)
		return
	}

	reservationID := reserveResult.ReservationID
	confirmedSeats := reserveResult.Seats
	if len(confirmedSeats) == 0 {
		confirmedSeats = seats
	}

	// 3. SAGA STEP 2: Payment Simulation
	paymentSuccess := true
	if req.PaymentInfo != nil && (req.PaymentInfo.CardToken == "tok_fail" || req.PaymentInfo.CardTokenAlt == "tok_fail") {
		paymentSuccess = false
	} else if req.PaymentInfoAlt != nil && (req.PaymentInfoAlt.CardToken == "tok_fail" || req.PaymentInfoAlt.CardTokenAlt == "tok_fail") {
		paymentSuccess = false
	}

	if !paymentSuccess {
		// Compensating transaction: Release hold
		s.releaseInventory(r.Context(), reservationID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "PAYMENT_FAILED",
			"message": "Payment simulation was declined",
		})
		return
	}

	// 4. SAGA STEP 3: Commit Inventory Reservation
	bookingID := fmt.Sprintf("bk-%d", time.Now().UnixNano())
	commitPayload := map[string]string{
		"reservationId": reservationID,
		"bookingId":     bookingID,
	}
	commitBytes, _ := json.Marshal(commitPayload)
	commitURL := fmt.Sprintf("%s/inventory/commit", s.inventoryServiceURL)
	commitReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, commitURL, bytes.NewReader(commitBytes))
	if err == nil {
		commitReq.Header.Set("Content-Type", "application/json")
		commitResp, commitErr := s.httpClient.Do(commitReq)
		if commitErr != nil || commitResp.StatusCode != http.StatusOK {
			log.Printf("Commit failed, attempting release compensation: %v", commitErr)
			s.releaseInventory(r.Context(), reservationID)
			http.Error(w, `{"error":"Inventory commit failed"}`, http.StatusInternalServerError)
			return
		}
		commitResp.Body.Close()
	}

	// 5. Persist Booking & Tickets
	bookingRef := fmt.Sprintf("REF-%s-%d", strings.ToUpper(strings.ReplaceAll(eventID, "-", "")), time.Now().UnixNano())
	totalAmount := tierPrice * float64(ticketCount)
	if totalAmount <= 0 {
		totalAmount = 60.00 * float64(ticketCount)
	}

	seatJSON, _ := json.Marshal(confirmedSeats)

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		http.Error(w, `{"error":"Database error persisting booking"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	now := time.Now()
	_, err = tx.ExecContext(r.Context(),
		`INSERT INTO bookings (id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'CONFIRMED', $12, 'SUCCESS', $13, $13)`,
		bookingID, bookingRef, userID, customerName, customerEmail, eventID, eventTitle, tierID, ticketCount, string(seatJSON), totalAmount, reservationID, now,
	)
	if err != nil {
		log.Printf("Insert booking error: %v", err)
		s.releaseInventory(r.Context(), reservationID)
		http.Error(w, fmt.Sprintf(`{"error":"Failed to save booking: %v"}`, err), http.StatusInternalServerError)
		return
	}

	var tickets []Ticket
	for i, seat := range confirmedSeats {
		ticketID := fmt.Sprintf("tkt-%d-%d", time.Now().UnixNano(), i+1)
		ticketCode := fmt.Sprintf("TKT-%s-%s-%d", eventID, seat, time.Now().UnixNano())
		seatLabel := fmt.Sprintf("Seat %s", seat)

		_, err = tx.ExecContext(r.Context(),
			`INSERT INTO tickets (id, booking_id, seat_id, seat_label, ticket_code, status, created_at) 
			 VALUES ($1, $2, $3, $4, $5, 'VALID', $6)`,
			ticketID, bookingID, seat, seatLabel, ticketCode, now,
		)
		if err != nil {
			log.Printf("Insert ticket error: %v", err)
		}

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

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error":"Transaction commit error"}`, http.StatusInternalServerError)
		return
	}

	resp := Booking{
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
		SeatIDs:          confirmedSeats,
		TotalAmount:      totalAmount,
		TotalAmountAlt:   totalAmount,
		Status:           "CONFIRMED",
		ReservationID:    reservationID,
		PaymentStatus:    "SUCCESS",
		Tickets:          tickets,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) releaseInventory(ctx context.Context, reservationID string) {
	if reservationID == "" {
		return
	}
	payload := map[string]string{
		"reservationId": reservationID,
		"reason":        "CANCELLED_OR_FAILED",
	}
	body, _ := json.Marshal(payload)
	releaseURL := fmt.Sprintf("%s/inventory/release", s.inventoryServiceURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, releaseURL, bytes.NewReader(body))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		resp, doErr := s.httpClient.Do(req)
		if doErr == nil {
			resp.Body.Close()
		}
	}
}

func (s *Server) fetchEventInfo(eventID, tierID string) (string, float64) {
	// Try database directly first for zero-latency reliability
	var title string
	var price float64

	err := s.db.QueryRow("SELECT title FROM events WHERE id = $1", eventID).Scan(&title)
	if err == nil && title != "" {
		if tierID != "" {
			_ = s.db.QueryRow("SELECT price FROM ticket_tiers WHERE event_id = $1 AND id = $2", eventID, tierID).Scan(&price)
		} else {
			_ = s.db.QueryRow("SELECT price FROM ticket_tiers WHERE event_id = $1 ORDER BY price ASC LIMIT 1", eventID).Scan(&price)
		}
		if price <= 0 {
			price = 60.00
		}
		return title, price
	}

	// Fallback to Catalog Service HTTP endpoint
	catalogURL := fmt.Sprintf("%s/events/%s", s.catalogServiceURL, eventID)
	resp, err := s.httpClient.Get(catalogURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		var eventData struct {
			Title string `json:"title"`
			Tiers []struct {
				ID    string  `json:"id"`
				Price float64 `json:"price"`
			} `json:"tiers"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&eventData); err == nil {
			title = eventData.Title
			for _, t := range eventData.Tiers {
				if t.ID == tierID || tierID == "" {
					price = t.Price
					break
				}
			}
		}
	}

	if title == "" {
		title = fmt.Sprintf("Event %s", eventID)
	}
	if price <= 0 {
		price = 60.00
	}
	return title, price
}

func (s *Server) handleBookingByID(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/bookings/")
	path = strings.TrimPrefix(path, "/bookings/")
	bookingID := strings.TrimSpace(path)

	if bookingID == "" {
		http.Error(w, `{"error":"Booking ID is required"}`, http.StatusBadRequest)
		return
	}

	var b Booking
	var seatsRaw string
	var tierID, resID sql.NullString
	err := s.db.QueryRow(
		`SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, created_at, updated_at 
		 FROM bookings WHERE id = $1 OR booking_reference = $1`,
		bookingID,
	).Scan(
		&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
		&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
		&b.Status, &resID, &b.PaymentStatus, &b.CreatedAt, &b.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Booking not found"})
		return
	} else if err != nil {
		log.Printf("Query error for booking: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Database error: %v"}`, err), http.StatusInternalServerError)
		return
	}

	b.BookingID = b.ID
	b.TotalAmountAlt = b.TotalAmount
	if tierID.Valid {
		b.TierID = tierID.String
	}
	if resID.Valid {
		b.ReservationID = resID.String
	}
	_ = json.Unmarshal([]byte(seatsRaw), &b.SeatIDs)

	// Fetch tickets
	b.Tickets = s.fetchTicketsForBooking(b.ID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(b)
}

func (s *Server) listBookings(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		email = r.URL.Query().Get("customerEmail")
	}
	eventID := r.URL.Query().Get("eventId")
	if eventID == "" {
		eventID = r.URL.Query().Get("event_id")
	}

	query := `SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, created_at, updated_at FROM bookings WHERE 1=1`
	var args []interface{}
	idx := 1

	if email != "" {
		query += fmt.Sprintf(" AND customer_email ILIKE $%d", idx)
		args = append(args, email)
		idx++
	}
	if eventID != "" {
		query += fmt.Sprintf(" AND event_id = $%d", idx)
		args = append(args, eventID)
		idx++
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Database query error: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var bookings []Booking
	for rows.Next() {
		var b Booking
		var seatsRaw string
		var tierID, resID sql.NullString
		if err := rows.Scan(
			&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
			&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
			&b.Status, &resID, &b.PaymentStatus, &b.CreatedAt, &b.UpdatedAt,
		); err == nil {
			b.BookingID = b.ID
			b.TotalAmountAlt = b.TotalAmount
			if tierID.Valid {
				b.TierID = tierID.String
			}
			if resID.Valid {
				b.ReservationID = resID.String
			}
			_ = json.Unmarshal([]byte(seatsRaw), &b.SeatIDs)
			b.Tickets = s.fetchTicketsForBooking(b.ID)
			bookings = append(bookings, b)
		}
	}

	if bookings == nil {
		bookings = []Booking{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bookings)
}

func (s *Server) fetchTicketsForBooking(bookingID string) []Ticket {
	rows, err := s.db.Query(
		`SELECT id, booking_id, seat_id, seat_label, ticket_code, status, created_at FROM tickets WHERE booking_id = $1`,
		bookingID,
	)
	if err != nil {
		return []Ticket{}
	}
	defer rows.Close()

	var tickets []Ticket
	for rows.Next() {
		var t Ticket
		var seatID sql.NullString
		if err := rows.Scan(&t.ID, &t.BookingID, &seatID, &t.SeatLabel, &t.TicketCode, &t.Status, &t.CreatedAt); err == nil {
			t.TicketID = t.ID
			if seatID.Valid {
				t.SeatID = seatID.String
			}
			tickets = append(tickets, t)
		}
	}
	if tickets == nil {
		tickets = []Ticket{}
	}
	return tickets
}

func main() {
	port := getEnv("PORT", "8083")
	catalogURL := getEnv("CATALOG_SERVICE_URL", "http://catalog-service:8081")
	inventoryURL := getEnv("INVENTORY_SERVICE_URL", "http://inventory-service:8082")

	db, err := initDB()
	if err != nil {
		log.Fatalf("Fatal database initialization error: %v", err)
	}
	defer db.Close()

	srv := &Server{
		db: db,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		catalogServiceURL:   catalogURL,
		inventoryServiceURL: inventoryURL,
	}

	mux := http.NewServeMux()

	// Health routes
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/api/health", srv.handleHealth)
	mux.HandleFunc("/api/bookings/health", srv.handleHealth)

	// Bookings collection routes
	mux.HandleFunc("/bookings", srv.handleBookings)
	mux.HandleFunc("/api/bookings", srv.handleBookings)

	// Single booking routes
	mux.HandleFunc("/bookings/", srv.handleBookingByID)
	mux.HandleFunc("/api/bookings/", srv.handleBookingByID)

	serverAddr := ":" + port
	log.Printf("Booking Service starting on %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		log.Fatalf("Server terminated: %v", err)
	}
}
