package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Server struct {
	db     *sql.DB
	worker *NotificationWorker
	mux    *http.ServeMux
}

func NewServer(db *sql.DB, worker *NotificationWorker) *Server {
	s := &Server{
		db:     db,
		worker: worker,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/api/health", s.handleHealth)

	s.mux.HandleFunc("/deliveries", s.handleDeliveries)
	s.mux.HandleFunc("/api/notifications/deliveries", s.handleDeliveries)

	s.mux.HandleFunc("/dispatch-test", s.handleDispatchTest)
	s.mux.HandleFunc("/api/notifications/dispatch-test", s.handleDispatchTest)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Enable basic CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "UP",
		"service":   "notification-service",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleDeliveries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	deliveries := s.worker.GetDeliveries()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(deliveries)
}

type DispatchTestRequest struct {
	NotificationEvent
	Channel string `json:"channel,omitempty"`
}

func (s *Server) handleDispatchTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DispatchTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.EventType == "" {
		req.EventType = "booking.confirmed"
	}
	if req.Recipient == "" {
		req.Recipient = "user@tickets.com"
	}

	targetChannel := ChannelEmail
	if strings.ToUpper(req.Channel) == ChannelSMS || strings.HasPrefix(req.Recipient, "+") {
		targetChannel = ChannelSMS
	}

	delivery, err := s.worker.processEvent(req.NotificationEvent, targetChannel)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to dispatch: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"delivery": delivery,
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgrespassword"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "ticket_db"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	var db *sql.DB
	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("[Warning] Failed to initialize DB pool: %v", err)
		db = nil
	} else if err = db.Ping(); err != nil {
		log.Printf("[Warning] Could not ping Postgres: %v (continuing with in-memory storage)", err)
	} else {
		log.Printf("[DB] Successfully connected to Postgres at %s:%s/%s", dbHost, dbPort, dbName)
	}

	rmqURL := os.Getenv("RABBITMQ_URL")
	if rmqURL == "" {
		rmqURL = "amqp://guest:guest@localhost:5672/"
	}

	var rmqConn *amqp.Connection
	for i := 0; i < 5; i++ {
		rmqConn, err = amqp.Dial(rmqURL)
		if err == nil {
			log.Printf("[RabbitMQ] Connected successfully to %s", rmqURL)
			break
		}
		log.Printf("[RabbitMQ] Connect attempt %d failed: %v. Retrying in 2s...", i+1, err)
		time.Sleep(2 * time.Second)
	}

	worker := NewNotificationWorker(db, rmqConn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := worker.StartConsumer(ctx); err != nil {
		log.Printf("[Warning] Failed to start RabbitMQ consumer: %v", err)
	}

	srv := NewServer(db, worker)
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: srv,
	}

	go func() {
		log.Printf("[Server] Notification Service listening on port %s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("[Server] Shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	if rmqConn != nil {
		_ = rmqConn.Close()
	}
	if db != nil {
		_ = db.Close()
	}
	log.Println("[Server] Exited.")
}
