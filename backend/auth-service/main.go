package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// User represents a user record in the database
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type Claims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type Server struct {
	db        *sql.DB
	jwtSecret []byte
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
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
				log.Printf("Auth Service connected to database successfully at %s:%s", dbHost, dbPort)
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

func (s *Server) migrateDB() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(64) PRIMARY KEY,
		email VARCHAR(255) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		role VARCHAR(32) NOT NULL DEFAULT 'customer',
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	`
	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Seed demo users if not present
	s.seedDefaultUser("admin@tickets.com", "admin123", "System Administrator", "admin")
	s.seedDefaultUser("user@tickets.com", "user123", "Demo Customer", "customer")

	return nil
}

func (s *Server) seedDefaultUser(email, password, name, role string) {
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM users WHERE email = $1", email).Scan(&count)
	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err == nil {
			userID := fmt.Sprintf("usr-%d", time.Now().UnixNano())
			now := time.Now()
			_, err = s.db.Exec(
				"INSERT INTO users (id, email, password_hash, name, role, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6)",
				userID, email, string(hash), name, role, now,
			)
			if err == nil {
				log.Printf("Seeded default %s user: %s", role, email)
			}
		}
	}
}

func (s *Server) generateToken(user *User) (string, error) {
	claims := Claims{
		UserID: user.ID,
		Email:  user.Email,
		Name:   user.Name,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "ticket-booking-auth-service",
			Subject:   user.ID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *Server) validateToken(tokenString string) (*Claims, error) {
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

	return nil, errors.New("invalid token")
}

func (s *Server) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
		return strings.TrimSpace(authHeader)
	}

	// Fallback to X-Auth-Token header
	if token := r.Header.Get("X-Auth-Token"); token != "" {
		return token
	}

	// Fallback to URL query parameter
	return r.URL.Query().Get("token")
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Auth-Token")
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
		"service": "auth-service",
	})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_REQUEST", "message": "Invalid JSON payload"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	req.Password = strings.TrimSpace(req.Password)
	req.Role = strings.TrimSpace(strings.ToLower(req.Role))

	if req.Email == "" || !strings.Contains(req.Email, "@") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_EMAIL", "message": "A valid email address is required"})
		return
	}

	if len(req.Password) < 6 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "WEAK_PASSWORD", "message": "Password must be at least 6 characters"})
		return
	}

	if req.Name == "" {
		req.Name = strings.Split(req.Email, "@")[0]
	}

	if req.Role != "admin" {
		req.Role = "customer"
	}

	// Check for duplicate email
	var count int
	_ = s.db.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM users WHERE email = $1", req.Email).Scan(&count)
	if count > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "EMAIL_EXISTS", "message": "An account with this email already exists"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "INTERNAL_ERROR", "message": "Failed to encrypt password"})
		return
	}

	userID := fmt.Sprintf("usr-%d", time.Now().UnixNano())
	now := time.Now()

	_, err = s.db.ExecContext(r.Context(),
		"INSERT INTO users (id, email, password_hash, name, role, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $6)",
		userID, req.Email, string(hash), req.Name, req.Role, now,
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "DATABASE_ERROR", "message": "Failed to create user"})
		return
	}

	user := &User{
		ID:        userID,
		Email:     req.Email,
		Name:      req.Name,
		Role:      req.Role,
		CreatedAt: now,
		UpdatedAt: now,
	}

	token, err := s.generateToken(user)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "TOKEN_ERROR", "message": "Failed to generate token"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
		},
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_REQUEST", "message": "Invalid JSON payload"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Password = strings.TrimSpace(req.Password)

	if req.Email == "" || req.Password == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "CREDENTIALS_REQUIRED", "message": "Email and password are required"})
		return
	}

	var user User
	err := s.db.QueryRowContext(r.Context(),
		"SELECT id, email, password_hash, name, role, created_at, updated_at FROM users WHERE email = $1",
		req.Email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.Role, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_CREDENTIALS", "message": "Invalid email or password"})
		return
	} else if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "DATABASE_ERROR", "message": "Internal query error"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_CREDENTIALS", "message": "Invalid email or password"})
		return
	}

	token, err := s.generateToken(&user)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "TOKEN_ERROR", "message": "Failed to generate token"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(AuthResponse{
		Token: token,
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Name:      user.Name,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
		},
	})
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	tokenStr := s.extractToken(r)
	if tokenStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":   false,
			"error":   "MISSING_TOKEN",
			"message": "Authorization token is missing",
		})
		return
	}

	claims, err := s.validateToken(tokenStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":   false,
			"error":   "INVALID_TOKEN",
			"message": err.Error(),
		})
		return
	}

	// Set headers for reverse proxy / API Gateway forwarding
	w.Header().Set("X-User-ID", claims.UserID)
	w.Header().Set("X-User-Email", claims.Email)
	w.Header().Set("X-User-Name", claims.Name)
	w.Header().Set("X-User-Role", claims.Role)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"valid": true,
		"user": map[string]string{
			"id":    claims.UserID,
			"email": claims.Email,
			"name":  claims.Name,
			"role":  claims.Role,
		},
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	tokenStr := s.extractToken(r)
	if tokenStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "UNAUTHORIZED", "message": "Authentication required"})
		return
	}

	claims, err := s.validateToken(tokenStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_TOKEN", "message": err.Error()})
		return
	}

	var user User
	err = s.db.QueryRowContext(r.Context(),
		"SELECT id, email, name, role, created_at, updated_at FROM users WHERE id = $1",
		claims.UserID,
	).Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "USER_NOT_FOUND", "message": "User not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(UserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
	})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	tokenStr := s.extractToken(r)
	if tokenStr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "UNAUTHORIZED", "message": "Authentication required"})
		return
	}

	claims, err := s.validateToken(tokenStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_TOKEN", "message": err.Error()})
		return
	}

	// RBAC Authorization check: Admin only
	if claims.Role != "admin" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "FORBIDDEN", "message": "Admin privileges required"})
		return
	}

	rows, err := s.db.QueryContext(r.Context(), "SELECT id, email, name, role, created_at FROM users ORDER BY created_at DESC")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "DATABASE_ERROR", "message": "Failed to list users"})
		return
	}
	defer rows.Close()

	var users []UserResponse
	for rows.Next() {
		var u UserResponse
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt); err == nil {
			users = append(users, u)
		}
	}

	if users == nil {
		users = []UserResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(users)
}

func main() {
	port := getEnv("PORT", "8084")
	jwtSecretKey := getEnv("JWT_SECRET", "ticket-booking-jwt-secret-key-2026")

	db, err := initDB()
	if err != nil {
		log.Fatalf("Fatal database error: %v", err)
	}
	defer db.Close()

	srv := &Server{
		db:        db,
		jwtSecret: []byte(jwtSecretKey),
	}

	if err := srv.migrateDB(); err != nil {
		log.Printf("Warning during DB migration: %v", err)
	}

	mux := http.NewServeMux()

	// Health routes
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/api/health", srv.handleHealth)
	mux.HandleFunc("/auth/health", srv.handleHealth)
	mux.HandleFunc("/api/auth/health", srv.handleHealth)

	// Auth Register routes
	mux.HandleFunc("/register", srv.handleRegister)
	mux.HandleFunc("/auth/register", srv.handleRegister)
	mux.HandleFunc("/api/auth/register", srv.handleRegister)

	// Auth Login routes
	mux.HandleFunc("/login", srv.handleLogin)
	mux.HandleFunc("/auth/login", srv.handleLogin)
	mux.HandleFunc("/api/auth/login", srv.handleLogin)

	// Auth Verify routes
	mux.HandleFunc("/verify", srv.handleVerify)
	mux.HandleFunc("/auth/verify", srv.handleVerify)
	mux.HandleFunc("/api/auth/verify", srv.handleVerify)

	// Auth Profile routes
	mux.HandleFunc("/me", srv.handleMe)
	mux.HandleFunc("/auth/me", srv.handleMe)
	mux.HandleFunc("/api/auth/me", srv.handleMe)

	// Admin Users routes
	mux.HandleFunc("/users", srv.handleUsers)
	mux.HandleFunc("/auth/users", srv.handleUsers)
	mux.HandleFunc("/api/auth/users", srv.handleUsers)

	serverAddr := ":" + port
	log.Printf("Auth Service starting on %s", serverAddr)
	if err := http.ListenAndServe(serverAddr, TracingMiddleware("auth-service", mux)); err != nil {
		log.Fatalf("Server terminated: %v", err)
	}
}
