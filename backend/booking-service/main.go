package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Claims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type PaymentInfo struct {
	Method       string `json:"method"`
	CardToken    string `json:"cardToken"`
	CardTokenAlt string `json:"card_token"`
}

type CreateBookingRequest struct {
	EventID           string       `json:"eventId"`
	EventIDAlt        string       `json:"event_id"`
	UserID            string       `json:"userId"`
	UserIDAlt         string       `json:"user_id"`
	CustomerName      string       `json:"customerName"`
	CustomerNameAlt   string       `json:"customer_name"`
	CustomerEmail     string       `json:"customerEmail"`
	CustomerEmailAlt  string       `json:"customer_email"`
	TierID            string       `json:"tierId"`
	TierIDAlt         string       `json:"tier_id"`
	Seats             []string     `json:"seats"`
	SeatIDsAlt        []string     `json:"seat_ids"`
	TicketCount       int          `json:"ticketCount"`
	QuantityAlt       int          `json:"quantity"`
	PaymentInfo       *PaymentInfo `json:"paymentInfo"`
	PaymentInfoAlt    *PaymentInfo `json:"payment_info"`
	IdempotencyKey    string       `json:"idempotencyKey"`
	IdempotencyKeyAlt string       `json:"idempotency_key"`
	HoldSeconds       int          `json:"holdSeconds"`
	HoldSecondsAlt    int          `json:"hold_seconds"`
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
	IdempotencyKey   string    `json:"idempotencyKey,omitempty"`
	Tickets          []Ticket  `json:"tickets"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type VerifyTicketRequest struct {
	TicketCode string `json:"ticketCode"`
	EventID    string `json:"eventId,omitempty"`
	Action     string `json:"action,omitempty"` // "VERIFY" (default) or "CHECK_IN"
}

type VerifyTicketResponse struct {
	Valid   bool     `json:"valid"`
	Status  string   `json:"status"` // "VALID", "USED", "CANCELLED", "INVALID"
	Message string   `json:"message"`
	Ticket  *Ticket  `json:"ticket,omitempty"`
	Booking *Booking `json:"booking,omitempty"`
}

type Server struct {
	db                  *sql.DB
	rdb                 *redis.Client
	rmq                 *RabbitMQ
	httpClient          *http.Client
	catalogServiceURL   string
	inventoryServiceURL string
	sagaOrchestrator    *SagaOrchestrator
	webhookManager      *WebhookManager
	jwtSecret           []byte
	startTime           time.Time
	reqCount            int64
}

func (s *Server) validateJWT(tokenString string) (*Claims, error) {
	tokenString = strings.TrimSpace(tokenString)
	if strings.HasPrefix(strings.ToLower(tokenString), "bearer ") {
		tokenString = strings.TrimSpace(tokenString[7:])
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
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

				// Ensure idempotency column and unique index exist
				_, _ = db.Exec(`
					ALTER TABLE bookings ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(128);
					CREATE UNIQUE INDEX IF NOT EXISTS idx_bookings_idempotency ON bookings(idempotency_key) WHERE idempotency_key IS NOT NULL AND idempotency_key != '';
				`)

				return db, nil
			}
		}
		log.Printf("Waiting for database connection (attempt %d/30): %v", i, err)
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("could not connect to database after 30 attempts: %w", err)
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
		log.Printf("Warning: booking-service could not connect to redis at %s:%s: %v", redisHost, redisPort, err)
		return rdb, err
	}

	log.Printf("Booking Service connected to redis successfully at %s:%s", redisHost, redisPort)
	return rdb, nil
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
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Idempotency-Key, X-Idempotency-Key, X-User-ID, X-User-Role")
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

	customerEmail := req.CustomerEmail
	if customerEmail == "" {
		customerEmail = req.CustomerEmailAlt
	}

	userID := req.UserID
	if userID == "" {
		userID = req.UserIDAlt
	}

	// Extract authentication context if provided
	authHeader := r.Header.Get("Authorization")
	xUserID := r.Header.Get("X-User-ID")
	xUserEmail := r.Header.Get("X-User-Email")
	xUserName := r.Header.Get("X-User-Name")

	if xUserID != "" {
		if userID == "" || userID == "usr-guest" {
			userID = xUserID
		}
		if (customerEmail == "" || customerEmail == "guest@example.com") && xUserEmail != "" {
			customerEmail = xUserEmail
		}
		if (customerName == "" || customerName == "Valued Guest") && xUserName != "" {
			customerName = xUserName
		}
	} else if authHeader != "" {
		if claims, err := s.validateJWT(authHeader); err == nil && claims != nil {
			if userID == "" || userID == "usr-guest" {
				userID = claims.UserID
			}
			if (customerEmail == "" || customerEmail == "guest@example.com") && claims.Email != "" {
				customerEmail = claims.Email
			}
			if (customerName == "" || customerName == "Valued Guest") && claims.Name != "" {
				customerName = claims.Name
			}
		}
	}

	if customerName == "" {
		customerName = "Valued Guest"
	}
	if customerEmail == "" {
		customerEmail = "guest@example.com"
	}
	if userID == "" {
		userID = "usr-guest"
	}

	tierID := req.TierID
	if tierID == "" {
		tierID = req.TierIDAlt
	}

	// Extract idempotency key from headers or request payload
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(r.Header.Get("X-Idempotency-Key"))
	}
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	}
	if idempotencyKey == "" {
		idempotencyKey = strings.TrimSpace(req.IdempotencyKeyAlt)
	}

	var redisIdemKey string
	if idempotencyKey != "" {
		redisIdemKey = "idempotency:booking:" + idempotencyKey

		// Step 1: Check Redis cache for completed or in-progress transaction
		if s.rdb != nil {
			cachedVal, err := s.rdb.Get(r.Context(), redisIdemKey).Result()
			if err == nil {
				if cachedVal == "IN_PROGRESS" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusConflict)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error":   "IDEMPOTENCY_IN_PROGRESS",
						"message": "A request with this idempotency key is currently being processed. Please retry shortly.",
					})
					return
				}
				// Replay cached booking response
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Idempotent-Replay", "true")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(cachedVal))
				return
			}
		}

		// Step 2: Check Postgres fallback if Redis cache expired or missed
		if existing, err := s.getBookingByIdempotencyKey(idempotencyKey); err == nil && existing != nil {
			respBytes, _ := json.Marshal(existing)
			if s.rdb != nil {
				_ = s.rdb.Set(context.Background(), redisIdemKey, string(respBytes), 24*time.Hour).Err()
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Idempotent-Replay", "true")
			w.WriteHeader(http.StatusOK)
			w.Write(respBytes)
			return
		}

		// Step 3: Atomic distributed lock to prevent concurrent duplicate execution
		if s.rdb != nil {
			locked, err := s.rdb.SetNX(r.Context(), redisIdemKey, "IN_PROGRESS", 60*time.Second).Result()
			if err == nil && !locked {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":   "IDEMPOTENCY_IN_PROGRESS",
					"message": "A request with this idempotency key is currently being processed. Please retry shortly.",
				})
				return
			}
		}
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

	var dbIdemKey interface{} = nil
	if idempotencyKey != "" {
		dbIdemKey = idempotencyKey
	}

	now := time.Now()
	_, err := s.db.ExecContext(r.Context(),
		`INSERT INTO bookings (id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, payment_status, idempotency_key, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'PENDING', $12, $13, $14, $14)`,
		bookingID, bookingRef, userID, customerName, customerEmail, eventID, eventTitle, tierID, ticketCount, string(seatJSON), totalAmount, paymentStatus, dbIdemKey, now,
	)
	if err != nil {
		if redisIdemKey != "" && s.rdb != nil {
			s.rdb.Del(context.Background(), redisIdemKey)
		}
		if strings.Contains(err.Error(), "idx_bookings_idempotency") || strings.Contains(err.Error(), "unique constraint") {
			if existing, gErr := s.getBookingByIdempotencyKey(idempotencyKey); gErr == nil && existing != nil {
				respBytes, _ := json.Marshal(existing)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Idempotent-Replay", "true")
				w.WriteHeader(http.StatusOK)
				w.Write(respBytes)
				return
			}
			http.Error(w, `{"error":"IDEMPOTENCY_CONFLICT","message":"Duplicate idempotency key"}`, http.StatusConflict)
			return
		}
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
		IdempotencyKey:   idempotencyKey,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	holdSeconds := req.HoldSeconds
	if holdSeconds <= 0 {
		holdSeconds = req.HoldSecondsAlt
	}
	if holdSeconds <= 0 {
		holdSeconds = 600
	}

	reservePayload := map[string]interface{}{
		"eventId":     eventID,
		"tierId":      tierID,
		"seats":       seats,
		"ticketCount": ticketCount,
		"holdSeconds": holdSeconds,
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
		var finalBooking *Booking
		b, err := s.getBookingTx(bookingID)
		if err == nil && b != nil {
			b.Tickets = tickets
			b.Status = "CONFIRMED"
			b.BookingID = b.ID
			b.TotalAmountAlt = b.TotalAmount
			b.IdempotencyKey = idempotencyKey
			finalBooking = b
		} else {
			sagaRes.Booking.Tickets = tickets
			sagaRes.Booking.Status = "CONFIRMED"
			sagaRes.Booking.IdempotencyKey = idempotencyKey
			finalBooking = sagaRes.Booking
		}

		// Cache completed booking in Redis
		if redisIdemKey != "" && s.rdb != nil {
			finalJSON, _ := json.Marshal(finalBooking)
			_ = s.rdb.Set(context.Background(), redisIdemKey, string(finalJSON), 24*time.Hour).Err()
		}

		// Trigger Partner Webhooks & Asynchronous Domain Events (Milestone 19 / R26, R22)
		if s.webhookManager != nil {
			go func(b *Booking) {
				eventID := b.EventID
				if eventID == "" {
					eventID = b.ID
				}
				// 1. Order Placed / Confirmed Event

				_, _ = s.webhookManager.DispatchEvent(eventID, "order.placed", map[string]interface{}{
					"bookingId":     b.ID,
					"customerEmail": b.CustomerEmail,
					"customerName":  b.CustomerName,
					"seats":         b.SeatIDs,
					"totalAmount":   b.TotalAmount,
					"timestamp":     time.Now().UTC().Format(time.RFC3339),
				})

				// 2. Revenue Threshold Event Check
				var totalRevenue float64
				if s.db != nil {
					_ = s.db.QueryRow("SELECT COALESCE(SUM(total_amount), 0) FROM bookings WHERE event_id = $1 AND status = 'CONFIRMED'", eventID).Scan(&totalRevenue)
					if totalRevenue > 0 {
						_, _ = s.webhookManager.DispatchEvent(eventID, "event.revenue_threshold", map[string]interface{}{
							"currentRevenue": totalRevenue,
							"currency":       "USD",
							"timestamp":      time.Now().UTC().Format(time.RFC3339),
						})
					}
				}
			}(finalBooking)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(finalBooking)

	} else if sagaRes.Status == "FAILED" {
		if redisIdemKey != "" && s.rdb != nil {
			s.rdb.Del(context.Background(), redisIdemKey)
		}
		w.WriteHeader(http.StatusConflict) // Or 402 Payment Required depending on error, keep simple
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "BOOKING_FAILED",
			"message": sagaRes.Error,
		})
	} else {
		if redisIdemKey != "" && s.rdb != nil {
			s.rdb.Del(context.Background(), redisIdemKey)
		}
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{
			"status":    "PROCESSING",
			"bookingId": bookingID,
		})
	}
}

func (s *Server) getBookingByIdempotencyKey(key string) (*Booking, error) {
	var b Booking
	var seatsRaw string
	var tierID, resID, idemKey sql.NullString
	err := s.db.QueryRow(
		`SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, idempotency_key, created_at, updated_at 
		 FROM bookings WHERE idempotency_key = $1`,
		key,
	).Scan(
		&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
		&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
		&b.Status, &resID, &b.PaymentStatus, &idemKey, &b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	b.BookingID = b.ID
	b.TotalAmountAlt = b.TotalAmount
	if tierID.Valid {
		b.TierID = tierID.String
	}
	if resID.Valid {
		b.ReservationID = resID.String
	}
	if idemKey.Valid {
		b.IdempotencyKey = idemKey.String
	}
	_ = json.Unmarshal([]byte(seatsRaw), &b.SeatIDs)
	b.Tickets = s.fetchTicketsForBooking(b.ID)
	return &b, nil
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
		if corrID := CorrelationIDFromContext(ctx); corrID != "" {
			req.Header.Set("X-Correlation-ID", corrID)
			req.Header.Set("X-Request-ID", corrID)
		}
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
	path = strings.TrimPrefix(path, "/")

	isCancel := false
	if strings.HasSuffix(path, "/cancel") {
		isCancel = true
		path = strings.TrimSuffix(path, "/cancel")
		path = strings.TrimSuffix(path, "/")
	} else if r.Method == http.MethodDelete {
		isCancel = true
	}

	bookingID := strings.TrimSpace(path)
	if bookingID == "" {
		http.Error(w, `{"error":"Booking ID is required"}`, http.StatusBadRequest)
		return
	}

	if isCancel {
		s.handleCancelBooking(w, r, bookingID)
		return
	}

	var b Booking
	var seatsRaw string
	var tierID, resID, idemKey sql.NullString
	err := s.db.QueryRow(
		`SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, idempotency_key, created_at, updated_at 
		 FROM bookings WHERE id = $1 OR booking_reference = $1`,
		bookingID,
	).Scan(
		&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
		&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
		&b.Status, &resID, &b.PaymentStatus, &idemKey, &b.CreatedAt, &b.UpdatedAt,
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
	if idemKey.Valid {
		b.IdempotencyKey = idemKey.String
	}
	_ = json.Unmarshal([]byte(seatsRaw), &b.SeatIDs)

	// Fetch tickets
	b.Tickets = s.fetchTicketsForBooking(b.ID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(b)
}

func (s *Server) handleCancelBooking(w http.ResponseWriter, r *http.Request, bookingID string) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// 1. Fetch booking
	var b Booking
	var seatsRaw string
	var tierID, resID, idemKey sql.NullString
	err := s.db.QueryRow(
		`SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, idempotency_key, created_at, updated_at 
		 FROM bookings WHERE id = $1 OR booking_reference = $1`,
		bookingID,
	).Scan(
		&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
		&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
		&b.Status, &resID, &b.PaymentStatus, &idemKey, &b.CreatedAt, &b.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "BOOKING_NOT_FOUND",
			"message": "Booking not found",
		})
		return
	} else if err != nil {
		log.Printf("Query error for booking during cancellation: %v", err)
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
	if idemKey.Valid {
		b.IdempotencyKey = idemKey.String
	}
	_ = json.Unmarshal([]byte(seatsRaw), &b.SeatIDs)

	// 2. Authorization verification
	requesterID := r.Header.Get("X-User-ID")
	requesterRole := r.Header.Get("X-User-Role")
	authHeader := r.Header.Get("Authorization")
	if (requesterID == "" || requesterRole == "") && authHeader != "" {
		if claims, errJwt := s.validateJWT(authHeader); errJwt == nil && claims != nil {
			requesterID = claims.UserID
			requesterRole = claims.Role
		}
	}

	if b.UserID != "" && b.UserID != "usr-guest" && requesterID != "" {
		if requesterRole != "admin" && b.UserID != requesterID {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "FORBIDDEN",
				"message": "You do not have permission to cancel this booking",
			})
			return
		}
	}

	// 3. Idempotency check: If already cancelled
	if b.Status == "CANCELLED" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"bookingId":        b.ID,
			"bookingReference": b.BookingReference,
			"status":           "CANCELLED",
			"message":          "Booking is already cancelled",
		})
		return
	}

	if b.Status != "CONFIRMED" && b.Status != "PENDING" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "INVALID_STATE",
			"message": fmt.Sprintf("Cannot cancel booking with status '%s'", b.Status),
		})
		return
	}

	// 4. Execute Cancellation Saga
	cancelPayload := map[string]interface{}{
		"bookingId":     b.ID,
		"reservationId": b.ReservationID,
		"eventId":       b.EventID,
		"tierId":        b.TierID,
		"quantity":      b.Quantity,
		"seatIds":       b.SeatIDs,
		"reason":        "USER_CANCELLATION",
	}

	if s.sagaOrchestrator != nil {
		res := s.sagaOrchestrator.ExecuteCancellationSaga(r.Context(), &b, cancelPayload)
		if res.Status == "CANCELLED" {
			if b.IdempotencyKey != "" && s.rdb != nil {
				_ = s.rdb.Del(context.Background(), "idempotency:booking:"+b.IdempotencyKey).Err()
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"bookingId":        b.ID,
				"bookingReference": b.BookingReference,
				"status":           "CANCELLED",
				"message":          "Booking cancelled and seats released successfully",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "CANCELLATION_FAILED",
			"message": res.Error,
		})
		return
	}

	// Fallback direct HTTP cancellation
	body, _ := json.Marshal(cancelPayload)
	reqCancel, reqErr := http.NewRequestWithContext(r.Context(), http.MethodPost, s.inventoryServiceURL+"/inventory/cancel", bytes.NewReader(body))
	if reqErr == nil {
		reqCancel.Header.Set("Content-Type", "application/json")
		if corrID := CorrelationIDFromContext(r.Context()); corrID != "" {
			reqCancel.Header.Set("X-Correlation-ID", corrID)
			reqCancel.Header.Set("X-Request-ID", corrID)
		}
		resp, errHttp := s.httpClient.Do(reqCancel)
		if errHttp == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			s.db.Exec("UPDATE bookings SET status = 'CANCELLED', updated_at = NOW() WHERE id = $1", b.ID)
			s.db.Exec("UPDATE tickets SET status = 'CANCELLED' WHERE booking_id = $1", b.ID)
			if b.IdempotencyKey != "" && s.rdb != nil {
				_ = s.rdb.Del(context.Background(), "idempotency:booking:"+b.IdempotencyKey).Err()
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"bookingId":        b.ID,
				"bookingReference": b.BookingReference,
				"status":           "CANCELLED",
				"message":          "Booking cancelled and seats released successfully",
			})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   "CANCELLATION_FAILED",
		"message": "Failed to cancel booking via inventory service",
	})
}

func (s *Server) handleVerifyTicket(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req VerifyTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid request payload: %v"}`, err), http.StatusBadRequest)
		return
	}

	ticketCode := strings.TrimSpace(req.TicketCode)
	if ticketCode == "" {
		http.Error(w, `{"error":"ticketCode is required"}`, http.StatusBadRequest)
		return
	}

	if s.db == nil {
		http.Error(w, `{"error":"Database not initialized"}`, http.StatusInternalServerError)
		return
	}

	// Query ticket
	var tkt Ticket
	var seatID sql.NullString
	err := s.db.QueryRow(
		`SELECT id, booking_id, seat_id, seat_label, ticket_code, status, created_at FROM tickets WHERE ticket_code = $1`,
		ticketCode,
	).Scan(&tkt.ID, &tkt.BookingID, &seatID, &tkt.SeatLabel, &tkt.TicketCode, &tkt.Status, &tkt.CreatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(VerifyTicketResponse{
				Valid:   false,
				Status:  "INVALID",
				Message: "Ticket code not found",
			})
			return
		}
		http.Error(w, fmt.Sprintf(`{"error":"Database error: %v"}`, err), http.StatusInternalServerError)
		return
	}
	if seatID.Valid {
		tkt.SeatID = seatID.String
	}
	tkt.TicketID = tkt.ID

	// Query booking
	var b Booking
	var tierID, resID, idemKey sql.NullString
	var seatsRaw string
	err = s.db.QueryRow(
		`SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, idempotency_key, created_at, updated_at 
		 FROM bookings WHERE id = $1`,
		tkt.BookingID,
	).Scan(
		&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
		&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
		&b.Status, &resID, &b.PaymentStatus, &idemKey, &b.CreatedAt, &b.UpdatedAt,
	)
	if err == nil {
		b.BookingID = b.ID
		b.TotalAmountAlt = b.TotalAmount
		if tierID.Valid {
			b.TierID = tierID.String
		}
		if resID.Valid {
			b.ReservationID = resID.String
		}
		if idemKey.Valid {
			b.IdempotencyKey = idemKey.String
		}
		_ = json.Unmarshal([]byte(seatsRaw), &b.SeatIDs)
	}

	// Check if event ID was supplied and matches
	if req.EventID != "" && b.EventID != "" && req.EventID != b.EventID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(VerifyTicketResponse{
			Valid:   false,
			Status:  "INVALID",
			Message: "Ticket does not belong to specified event",
			Ticket:  &tkt,
			Booking: &b,
		})
		return
	}

	// If booking is CANCELLED
	if b.Status == "CANCELLED" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(VerifyTicketResponse{
			Valid:   false,
			Status:  "CANCELLED",
			Message: "Associated booking has been cancelled",
			Ticket:  &tkt,
			Booking: &b,
		})
		return
	}

	// Action handling: CHECK_IN vs VERIFY
	action := strings.ToUpper(strings.TrimSpace(req.Action))
	if action == "CHECK_IN" {
		if tkt.Status == "USED" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(VerifyTicketResponse{
				Valid:   false,
				Status:  "USED",
				Message: "Ticket has already been redeemed and checked in",
				Ticket:  &tkt,
				Booking: &b,
			})
			return
		}

		// Update ticket status to USED
		_, updateErr := s.db.Exec(`UPDATE tickets SET status = 'USED' WHERE id = $1`, tkt.ID)
		if updateErr != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Failed to update ticket status: %v"}`, updateErr), http.StatusInternalServerError)
			return
		}
		tkt.Status = "USED"

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(VerifyTicketResponse{
			Valid:   true,
			Status:  "USED",
			Message: "Ticket successfully checked in",
			Ticket:  &tkt,
			Booking: &b,
		})
		return
	}

	// Default: VERIFY
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(VerifyTicketResponse{
		Valid:   tkt.Status == "VALID",
		Status:  tkt.Status,
		Message: "Ticket is valid for admission",
		Ticket:  &tkt,
		Booking: &b,
	})
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

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		userID = r.URL.Query().Get("user_id")
	}

	if userID == "" && email == "" {
		authHeader := r.Header.Get("Authorization")
		xUserID := r.Header.Get("X-User-ID")
		if xUserID != "" {
			userID = xUserID
		} else if authHeader != "" {
			if claims, err := s.validateJWT(authHeader); err == nil && claims != nil {
				userID = claims.UserID
			}
		}
	}

	query := `SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, created_at, updated_at FROM bookings WHERE 1=1`
	var args []interface{}
	idx := 1

	if userID != "" {
		query += fmt.Sprintf(" AND user_id = $%d", idx)
		args = append(args, userID)
		idx++
	}
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

func (s *Server) handleDLQMessages(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if s.rmq == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "RabbitMQ is not connected"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		limit := 100
		if lStr := r.URL.Query().Get("limit"); lStr != "" {
			if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
				limit = l
			}
		}
		messages, err := s.rmq.PeekDLQ(limit)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to peek DLQ: %v", err)})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"queueName": SagaDLQ,
			"count":     len(messages),
			"messages":  messages,
		})
	case http.MethodDelete:
		purged, err := s.rmq.PurgeDLQ()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to purge DLQ: %v", err)})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "PURGED",
			"purged": purged,
		})
	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDLQStats(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if s.rmq == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "RabbitMQ is not connected"})
		return
	}
	stats, err := s.rmq.GetDLQStats()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to get DLQ stats: %v", err)})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleDLQRetry(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if s.rmq == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "RabbitMQ is not connected"})
		return
	}
	reprocessed, err := s.rmq.ReprocessDLQ()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to reprocess DLQ: %v", err)})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "REPROCESSED",
		"reprocessed": reprocessed,
	})
}

type DLQSimulateRequest struct {
	Type         string      `json:"type"` // poison, retry_fail, direct_dlq
	RoutingKey   string      `json:"routingKey"`
	Payload      interface{} `json:"payload"`
	ErrorMessage string      `json:"errorMessage"`
}

func (s *Server) handleDLQSimulate(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if s.rmq == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "RabbitMQ is not connected"})
		return
	}

	var req DLQSimulateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid payload"})
		return
	}

	routingKey := req.RoutingKey
	if routingKey == "" {
		routingKey = "inventory.reserve"
	}

	var body []byte
	switch p := req.Payload.(type) {
	case string:
		body = []byte(p)
	default:
		body, _ = json.Marshal(p)
	}
	if len(body) == 0 {
		body = []byte(`{"invalid":true}`)
	}

	switch req.Type {
	case "poison":
		err := s.rmq.Publish(routingKey, body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to publish: %v", err)})
			return
		}
	case "retry_fail":
		headers := amqp.Table{
			"x-simulate-error": req.ErrorMessage,
		}
		if req.ErrorMessage == "" {
			headers["x-simulate-error"] = "Simulated transient failure for DLQ retry test"
		}
		err := s.rmq.PublishWithHeaders(SagaExchange, routingKey, body, headers)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to publish: %v", err)})
			return
		}
	case "direct_dlq":
		err := s.rmq.PublishToDLQ(routingKey, body, "MANUAL_SIMULATION", req.ErrorMessage, 0)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to publish to DLQ: %v", err)})
			return
		}
	default:
		err := s.rmq.Publish(routingKey, body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to publish: %v", err)})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "SIMULATED",
		"type":       req.Type,
		"routingKey": routingKey,
	})
}

func (s *Server) startBookingJanitor() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			if s.db == nil {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, _ = s.db.ExecContext(ctx,
				`UPDATE bookings 
				 SET status = 'EXPIRED', updated_at = NOW() 
				 WHERE status = 'PENDING' AND created_at < NOW() - INTERVAL '15 minutes'`)
			cancel()
		}
	}()
}

func main() {
	port := getEnv("PORT", "8083")
	catalogURL := getEnv("CATALOG_SERVICE_URL", "http://catalog-service:8081")
	inventoryURL := getEnv("INVENTORY_SERVICE_URL", "http://inventory-service:8082")
	jwtSecretKey := getEnv("JWT_SECRET", "ticket-booking-jwt-secret-key-2026")

	db, err := initDB()
	if err != nil {
		log.Fatalf("Fatal database initialization error: %v", err)
	}
	defer db.Close()

	rdb, err := initRedis()
	if err != nil {
		log.Printf("Warning: Redis initialization error: %v", err)
	} else if rdb != nil {
		defer rdb.Close()
	}

	srv := &Server{
		db:  db,
		rdb: rdb,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		catalogServiceURL:   catalogURL,
		inventoryServiceURL: inventoryURL,
		jwtSecret:           []byte(jwtSecretKey),
		startTime:           time.Now(),
	}

	rmq, err := InitRabbitMQ("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Printf("Warning: RabbitMQ init failed: %v", err)
	} else {
		defer rmq.Close()
		srv.rmq = rmq
		srv.sagaOrchestrator = NewSagaOrchestrator(rmq, srv)
	}

	srv.startBookingJanitor()
	if err := srv.initWaitlistSchema(context.Background()); err != nil {
		log.Printf("Warning: failed to init waitlist schema: %v", err)
	}

	// Initialize Partner Webhook Manager (Milestone 19 / R26, R22)
	srv.webhookManager = NewWebhookManager(srv.db, srv.rmq)

	mux := http.NewServeMux()

	// Health routes
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/api/health", srv.handleHealth)
	mux.HandleFunc("/api/bookings/health", srv.handleHealth)

	// Metrics routes (R23)
	mux.HandleFunc("/metrics", srv.handleMetrics)
	mux.HandleFunc("/api/metrics", srv.handleMetrics)
	mux.HandleFunc("/api/bookings/metrics", srv.handleMetrics)

	// Bookings collection routes
	mux.HandleFunc("/bookings", srv.handleBookings)
	mux.HandleFunc("/api/bookings", srv.handleBookings)

	// Ticket verification and pass check-in routes (Milestone 23)
	mux.HandleFunc("/bookings/verify-ticket", srv.handleVerifyTicket)
	mux.HandleFunc("/api/bookings/verify-ticket", srv.handleVerifyTicket)

	// Single booking routes
	mux.HandleFunc("/bookings/", srv.handleBookingByID)
	mux.HandleFunc("/api/bookings/", srv.handleBookingByID)

	// Waitlist routes
	mux.HandleFunc("/waitlist", srv.handleWaitlist)
	mux.HandleFunc("/api/waitlist", srv.handleWaitlist)
	mux.HandleFunc("/waitlist/", srv.handleWaitlistByID)
	mux.HandleFunc("/api/waitlist/", srv.handleWaitlistByID)

	// Dead Letter Queue (DLQ) management routes
	mux.HandleFunc("/dlq/messages", srv.handleDLQMessages)
	mux.HandleFunc("/api/dlq/messages", srv.handleDLQMessages)
	mux.HandleFunc("/dlq/stats", srv.handleDLQStats)
	mux.HandleFunc("/api/dlq/stats", srv.handleDLQStats)
	mux.HandleFunc("/dlq/retry", srv.handleDLQRetry)
	mux.HandleFunc("/api/dlq/retry", srv.handleDLQRetry)
	mux.HandleFunc("/dlq/simulate", srv.handleDLQSimulate)
	mux.HandleFunc("/api/dlq/simulate", srv.handleDLQSimulate)

	// Partner Webhook subsystem routes (Milestone 19 / R26, R22)
	mux.HandleFunc("/webhooks/subscriptions", srv.handleWebhookSubscriptions)
	mux.HandleFunc("/api/webhooks/subscriptions", srv.handleWebhookSubscriptions)
	mux.HandleFunc("/webhooks/subscriptions/", srv.handleWebhookSubscriptionByID)
	mux.HandleFunc("/api/webhooks/subscriptions/", srv.handleWebhookSubscriptionByID)
	mux.HandleFunc("/webhooks/deliveries", srv.handleWebhookDeliveries)
	mux.HandleFunc("/api/webhooks/deliveries", srv.handleWebhookDeliveries)
	mux.HandleFunc("/webhooks/dispatch", srv.handleWebhookTestDispatch)
	mux.HandleFunc("/api/webhooks/dispatch", srv.handleWebhookTestDispatch)

	serverAddr := ":" + port
	log.Printf("Booking Service starting on %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, TracingMiddleware("booking-service", mux)); err != nil {
		log.Fatalf("Server terminated: %v", err)
	}
}
