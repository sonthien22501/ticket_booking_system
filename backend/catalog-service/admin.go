package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

type UpdateEventStatusRequest struct {
	Status string `json:"status"`
}

func (s *Server) handleEventStatusUpdate(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPut && r.Method != http.MethodPatch && r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/events/")
	path = strings.TrimPrefix(path, "/events/")
	path = strings.TrimSuffix(path, "/status")
	eventID := strings.TrimSpace(path)

	var req UpdateEventStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid request payload: %v"}`, err), http.StatusBadRequest)
		return
	}

	status := strings.ToUpper(strings.TrimSpace(req.Status))
	if status != "ACTIVE" && status != "PUBLISHED" && status != "PAUSED" && status != "COMPLETED" && status != "CANCELLED" {
		http.Error(w, `{"error":"Invalid status. Allowed values: ACTIVE, PUBLISHED, PAUSED, COMPLETED, CANCELLED"}`, http.StatusBadRequest)
		return
	}

	if s.db != nil && eventID != "" {
		res, err := s.db.Exec(`UPDATE events SET status = $1, updated_at = NOW() WHERE id = $2`, status, eventID)
		if err != nil {
			log.Printf("Failed to update event status: %v", err)
			http.Error(w, `{"error":"Failed to update event status"}`, http.StatusInternalServerError)
			return
		}
		rowsAffected, _ := res.RowsAffected()
		if rowsAffected == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Event not found", "eventId": eventID})
			return
		}

		if s.rdb != nil {
			ctx := context.Background()
			_ = s.rdb.Del(ctx, fmt.Sprintf("event_id:%s", eventID))
			iter := s.rdb.Scan(ctx, 0, "events_list:*", 0).Iterator()
			for iter.Next(ctx) {
				_ = s.rdb.Del(ctx, iter.Val())
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"eventId": eventID,
		"status":  status,
		"updated": true,
	})
}
