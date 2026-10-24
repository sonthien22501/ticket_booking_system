package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Notification struct {
	ID             string          `json:"id"`
	UserID         string          `json:"userId,omitempty"`
	RecipientEmail string          `json:"recipientEmail"`
	Type           string          `json:"type"` // WAITLIST_PROMOTED, TICKET_TRANSFERRED, BOOKING_CONFIRMED, SYSTEM_ALERT
	Title          string          `json:"title"`
	Message        string          `json:"message"`
	Data           json.RawMessage `json:"data,omitempty"`
	IsRead         bool            `json:"isRead"`
	CreatedAt      time.Time       `json:"createdAt"`
}

type NotificationListResponse struct {
	Notifications []Notification `json:"notifications"`
	Total         int            `json:"total"`
	UnreadCount   int            `json:"unreadCount"`
}

type CreateNotificationRequest struct {
	UserID         string                 `json:"userId,omitempty"`
	RecipientEmail string                 `json:"recipientEmail"`
	Type           string                 `json:"type"`
	Title          string                 `json:"title"`
	Message        string                 `json:"message"`
	Data           map[string]interface{} `json:"data,omitempty"`
}

// In-memory fallback if database is not initialized
var (
	memNotifsMu sync.RWMutex
	memNotifs   = make([]Notification, 0)
)

func generateNotifID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("notif-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

func (s *Server) dispatchNotification(ctx context.Context, notif Notification) error {
	if notif.ID == "" {
		notif.ID = generateNotifID()
	}
	if notif.CreatedAt.IsZero() {
		notif.CreatedAt = time.Now().UTC()
	}
	if len(notif.Data) == 0 {
		notif.Data = json.RawMessage("{}")
	}

	if s.db != nil {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO notifications (id, user_id, recipient_email, type, title, message, data, is_read, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO NOTHING
		`, notif.ID, notif.UserID, notif.RecipientEmail, notif.Type, notif.Title, notif.Message, notif.Data, notif.IsRead, notif.CreatedAt)
		if err == nil {
			return nil
		}
	}

	// In-memory fallback
	memNotifsMu.Lock()
	defer memNotifsMu.Unlock()
	memNotifs = append([]Notification{notif}, memNotifs...)
	return nil
}

func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodGet:
		email := strings.TrimSpace(r.URL.Query().Get("email"))
		userID := strings.TrimSpace(r.URL.Query().Get("userId"))

		if email == "" && userID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "MISSING_FILTER",
				"message": "Either 'email' or 'userId' query parameter must be provided",
			})
			return
		}

		list, unreadCount, err := s.queryNotifications(r.Context(), email, userID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "QUERY_ERROR",
				"message": err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(NotificationListResponse{
			Notifications: list,
			Total:         len(list),
			UnreadCount:   unreadCount,
		})

	case http.MethodPost:
		var req CreateNotificationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "INVALID_JSON",
				"message": err.Error(),
			})
			return
		}

		if req.RecipientEmail == "" && req.UserID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "VALIDATION_FAILED",
				"message": "recipientEmail or userId is required",
			})
			return
		}

		dataBytes, _ := json.Marshal(req.Data)
		notif := Notification{
			ID:             generateNotifID(),
			UserID:         req.UserID,
			RecipientEmail: req.RecipientEmail,
			Type:           req.Type,
			Title:          req.Title,
			Message:        req.Message,
			Data:           dataBytes,
			IsRead:         false,
			CreatedAt:      time.Now().UTC(),
		}

		if err := s.dispatchNotification(r.Context(), notif); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "DISPATCH_ERROR",
				"message": err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(notif)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) queryNotifications(ctx context.Context, email, userID string) ([]Notification, int, error) {
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx, `
			SELECT id, COALESCE(user_id, ''), recipient_email, type, title, message, COALESCE(data, '{}'::jsonb), is_read, created_at
			FROM notifications
			WHERE ($1 != '' AND recipient_email ILIKE $1)
			   OR ($2 != '' AND user_id = $2)
			ORDER BY created_at DESC
			LIMIT 50
		`, email, userID)
		if err == nil {
			defer rows.Close()
			var notifs []Notification
			unread := 0
			for rows.Next() {
				var n Notification
				var dataStr []byte
				if err := rows.Scan(&n.ID, &n.UserID, &n.RecipientEmail, &n.Type, &n.Title, &n.Message, &dataStr, &n.IsRead, &n.CreatedAt); err == nil {
					n.Data = dataStr
					if !n.IsRead {
						unread++
					}
					notifs = append(notifs, n)
				}
			}
			return notifs, unread, nil
		}
	}

	// Memory fallback
	memNotifsMu.RLock()
	defer memNotifsMu.RUnlock()
	var filtered []Notification
	unread := 0
	for _, n := range memNotifs {
		match := false
		if email != "" && strings.EqualFold(n.RecipientEmail, email) {
			match = true
		}
		if userID != "" && n.UserID == userID {
			match = true
		}
		if match {
			if !n.IsRead {
				unread++
			}
			filtered = append(filtered, n)
		}
	}
	return filtered, unread, nil
}

func (s *Server) handleNotificationByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Expected path: /api/notifications/{id}/read or /notifications/{id}/read
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/notifications/")
	trimmed = strings.TrimPrefix(trimmed, "/notifications/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 || parts[1] != "read" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	notifID := parts[0]

	if s.db != nil {
		_, _ = s.db.ExecContext(r.Context(), "UPDATE notifications SET is_read = true WHERE id = $1", notifID)
	}

	memNotifsMu.Lock()
	for i := range memNotifs {
		if memNotifs[i].ID == notifID {
			memNotifs[i].IsRead = true
		}
	}
	memNotifsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      notifID,
		"isRead":  true,
	})
}

func (s *Server) handleNotificationReadAll(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	email := strings.TrimSpace(r.URL.Query().Get("email"))
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))

	// Check body if query params missing
	if email == "" && userID == "" {
		var req struct {
			Email  string `json:"email"`
			UserID string `json:"userId"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		email = strings.TrimSpace(req.Email)
		userID = strings.TrimSpace(req.UserID)
	}

	if email == "" && userID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "MISSING_FILTER",
			"message": "email or userId is required to mark all as read",
		})
		return
	}

	updatedCount := 0
	if s.db != nil {
		res, err := s.db.ExecContext(r.Context(), `
			UPDATE notifications
			SET is_read = true
			WHERE ($1 != '' AND recipient_email ILIKE $1)
			   OR ($2 != '' AND user_id = $2)
		`, email, userID)
		if err == nil {
			n, _ := res.RowsAffected()
			updatedCount = int(n)
		}
	}

	memNotifsMu.Lock()
	for i := range memNotifs {
		if (email != "" && strings.EqualFold(memNotifs[i].RecipientEmail, email)) || (userID != "" && memNotifs[i].UserID == userID) {
			if !memNotifs[i].IsRead {
				updatedCount++
				memNotifs[i].IsRead = true
			}
		}
	}
	memNotifsMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"updatedCount": updatedCount,
	})
}
