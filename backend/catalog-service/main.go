package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type TicketTier struct {
	ID        string    `json:"id"`
	TierID    string    `json:"tierId"`
	EventID   string    `json:"eventId"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Capacity  int       `json:"capacity"`
	CreatedAt time.Time `json:"createdAt"`
}

type Event struct {
	ID            string       `json:"id"`
	EventID       string       `json:"eventId"`
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	Category      string       `json:"category"`
	VenueName     string       `json:"venueName"`
	VenueNameAlt  string       `json:"venue_name"`
	VenueLocation string       `json:"venueLocation"`
	VenueLocAlt   string       `json:"venue_location"`
	StartTime     time.Time    `json:"startTime"`
	StartTimeAlt  time.Time    `json:"start_time"`
	EndTime       time.Time    `json:"endTime"`
	EndTimeAlt    time.Time    `json:"end_time"`
	Status        string       `json:"status"`
	ImageURL      string       `json:"imageUrl"`
	ImageURLAlt   string       `json:"image_url"`
	Tiers         []TicketTier `json:"tiers"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

type CreateEventRequest struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Category      string `json:"category"`
	VenueName     string `json:"venueName"`
	VenueNameAlt  string `json:"venue_name"`
	VenueLocation string `json:"venueLocation"`
	VenueLocAlt   string `json:"venue_location"`
	StartTime     string `json:"startTime"`
	StartTimeAlt  string `json:"start_time"`
	EndTime       string `json:"endTime"`
	EndTimeAlt    string `json:"end_time"`
	ImageURL      string `json:"imageUrl"`
	ImageURLAlt   string `json:"image_url"`
	Tiers         []struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		Price    float64 `json:"price"`
		Capacity int     `json:"capacity"`
	} `json:"tiers"`
}

type Server struct {
	db *sql.DB
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

	// Retry loop for database readiness
	for i := 1; i <= 30; i++ {
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				log.Printf("Catalog Service connected to database successfully at %s:%s", dbHost, dbPort)
				db.SetMaxOpenConns(25)
				db.SetMaxIdleConns(10)
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
		"service": "catalog-service",
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.listEvents(w, r)
	case http.MethodPost:
		s.createEvent(w, r)
	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")

	query := `SELECT id, title, description, category, venue_name, venue_location, start_time, end_time, status, image_url, created_at, updated_at FROM events WHERE 1=1`
	var args []interface{}
	idx := 1

	if category != "" {
		query += fmt.Sprintf(" AND category ILIKE $%d", idx)
		args = append(args, "%"+category+"%")
		idx++
	}

	if search != "" {
		query += fmt.Sprintf(" AND (title ILIKE $%d OR description ILIKE $%d OR venue_name ILIKE $%d)", idx, idx, idx)
		args = append(args, "%"+search+"%")
		idx++
	}

	query += " ORDER BY start_time ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Printf("Query error in listEvents: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Database query error: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var desc, img sql.NullString
		err := rows.Scan(
			&e.ID, &e.Title, &desc, &e.Category,
			&e.VenueName, &e.VenueLocation,
			&e.StartTime, &e.EndTime,
			&e.Status, &img,
			&e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		if desc.Valid {
			e.Description = desc.String
		}
		if img.Valid {
			e.ImageURL = img.String
			e.ImageURLAlt = img.String
		}
		e.EventID = e.ID
		e.VenueNameAlt = e.VenueName
		e.VenueLocAlt = e.VenueLocation
		e.StartTimeAlt = e.StartTime
		e.EndTimeAlt = e.EndTime

		// Fetch tiers for this event
		e.Tiers = s.fetchTiersForEvent(e.ID)
		events = append(events, e)
	}

	if events == nil {
		events = []Event{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(events)
}

func (s *Server) fetchTiersForEvent(eventID string) []TicketTier {
	tierRows, err := s.db.Query(
		`SELECT id, event_id, name, price, capacity, created_at FROM ticket_tiers WHERE event_id = $1 ORDER BY price DESC`,
		eventID,
	)
	if err != nil {
		log.Printf("Failed to fetch tiers for event %s: %v", eventID, err)
		return []TicketTier{}
	}
	defer tierRows.Close()

	var tiers []TicketTier
	for tierRows.Next() {
		var t TicketTier
		if err := tierRows.Scan(&t.ID, &t.EventID, &t.Name, &t.Price, &t.Capacity, &t.CreatedAt); err == nil {
			t.TierID = t.ID
			tiers = append(tiers, t)
		}
	}
	if tiers == nil {
		tiers = []TicketTier{}
	}
	return tiers
}

func (s *Server) handleEventByID(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/events/")
	path = strings.TrimPrefix(path, "/events/")
	eventID := strings.TrimSpace(path)

	if eventID == "" {
		http.Error(w, `{"error":"Event ID is required"}`, http.StatusBadRequest)
		return
	}

	var e Event
	var desc, img sql.NullString
	err := s.db.QueryRow(
		`SELECT id, title, description, category, venue_name, venue_location, start_time, end_time, status, image_url, created_at, updated_at FROM events WHERE id = $1`,
		eventID,
	).Scan(
		&e.ID, &e.Title, &desc, &e.Category,
		&e.VenueName, &e.VenueLocation,
		&e.StartTime, &e.EndTime,
		&e.Status, &img,
		&e.CreatedAt, &e.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Event not found", "eventId": eventID})
		return
	} else if err != nil {
		log.Printf("QueryRow error: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Database error: %v"}`, err), http.StatusInternalServerError)
		return
	}

	if desc.Valid {
		e.Description = desc.String
	}
	if img.Valid {
		e.ImageURL = img.String
		e.ImageURLAlt = img.String
	}
	e.EventID = e.ID
	e.VenueNameAlt = e.VenueName
	e.VenueLocAlt = e.VenueLocation
	e.StartTimeAlt = e.StartTime
	e.EndTimeAlt = e.EndTime
	e.Tiers = s.fetchTiersForEvent(e.ID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(e)
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid request payload: %v"}`, err), http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		req.ID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	venueName := req.VenueName
	if venueName == "" {
		venueName = req.VenueNameAlt
	}
	venueLoc := req.VenueLocation
	if venueLoc == "" {
		venueLoc = req.VenueLocAlt
	}
	imgURL := req.ImageURL
	if imgURL == "" {
		imgURL = req.ImageURLAlt
	}

	startTimeStr := req.StartTime
	if startTimeStr == "" {
		startTimeStr = req.StartTimeAlt
	}
	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		startTime = time.Now().Add(24 * time.Hour)
	}

	endTimeStr := req.EndTime
	if endTimeStr == "" {
		endTimeStr = req.EndTimeAlt
	}
	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		endTime = startTime.Add(3 * time.Hour)
	}

	tx, err := s.db.Begin()
	if err != nil {
		http.Error(w, `{"error":"Database transaction failed"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO events (id, title, description, category, venue_name, venue_location, start_time, end_time, status, image_url) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'ACTIVE', $9)`,
		req.ID, req.Title, req.Description, req.Category, venueName, venueLoc, startTime, endTime, imgURL,
	)
	if err != nil {
		log.Printf("Insert event failed: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"Failed to insert event: %v"}`, err), http.StatusInternalServerError)
		return
	}

	for i, tier := range req.Tiers {
		tierID := tier.ID
		if tierID == "" {
			tierID = fmt.Sprintf("%s-tier-%d", req.ID, i+1)
		}
		_, err = tx.Exec(
			`INSERT INTO ticket_tiers (id, event_id, name, price, capacity) VALUES ($1, $2, $3, $4, $5)`,
			tierID, req.ID, tier.Name, tier.Price, tier.Capacity,
		)
		if err != nil {
			log.Printf("Insert tier failed: %v", err)
			http.Error(w, fmt.Sprintf(`{"error":"Failed to insert tier: %v"}`, err), http.StatusInternalServerError)
			return
		}

		// Insert inventory record
		_, err = tx.Exec(
			`INSERT INTO event_inventory (event_id, tier_id, total_seats, available_seats, reserved_seats, booked_seats, version) VALUES ($1, $2, $3, $3, 0, 0, 0) ON CONFLICT DO NOTHING`,
			req.ID, tierID, tier.Capacity,
		)
		if err != nil {
			log.Printf("Insert inventory failed: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error":"Failed to commit transaction"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "CREATED",
		"id":      req.ID,
		"eventId": req.ID,
	})
}

func main() {
	port := getEnv("PORT", "8081")

	db, err := initDB()
	if err != nil {
		log.Fatalf("Fatal database initialization error: %v", err)
	}
	defer db.Close()

	srv := &Server{db: db}

	mux := http.NewServeMux()

	// Health routes
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/api/health", srv.handleHealth)
	mux.HandleFunc("/api/events/health", srv.handleHealth)

	// Events collection routes
	mux.HandleFunc("/events", srv.handleEvents)
	mux.HandleFunc("/api/events", srv.handleEvents)

	// Single event routes
	mux.HandleFunc("/events/", srv.handleEventByID)
	mux.HandleFunc("/api/events/", srv.handleEventByID)

	serverAddr := ":" + port
	log.Printf("Event Catalog Service starting on %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		log.Fatalf("Server terminated: %v", err)
	}
}
