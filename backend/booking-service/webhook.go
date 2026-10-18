package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// WebhookSubscription defines a partner webhook callback registration
type WebhookSubscription struct {
	ID        string    `json:"id"`
	EventID   string    `json:"eventId"` // Specific eventId or "*" for all events
	TargetURL string    `json:"targetUrl"`
	Secret    string    `json:"secret,omitempty"`
	Events    []string  `json:"events"` // e.g. ["event.capacity_reached", "event.revenue_threshold", "order.placed"]
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// WebhookDelivery defines an audit record of a dispatched webhook notification
type WebhookDelivery struct {
	ID             string    `json:"id"`
	SubscriptionID string    `json:"subscriptionId"`
	EventID        string    `json:"eventId"`
	EventType      string    `json:"eventType"`
	TargetURL      string    `json:"targetUrl"`
	StatusCode     int       `json:"statusCode"`
	DurationMs     int64     `json:"durationMs"`
	Success        bool      `json:"success"`
	ErrorMessage   string    `json:"errorMessage,omitempty"`
	PayloadJSON    string    `json:"payloadJson"`
	CreatedAt      time.Time `json:"createdAt"`
}

// WebhookPayload sent to partner endpoint
type WebhookPayload struct {
	EventID   string                 `json:"eventId"`
	EventType string                 `json:"eventType"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// computeHMACSignature signs the payload using HMAC-SHA256
func computeHMACSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}

// generateRandomID generates a unique hex-based ID with prefix
func generateRandomID(prefix string) string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(bytes))
}

// WebhookManager coordinates partner webhook subscriptions and event delivery
type WebhookManager struct {
	db            *sql.DB
	rmq           *RabbitMQ
	httpClient    *http.Client
	subscriptions map[string]*WebhookSubscription
	deliveries    []*WebhookDelivery
	mu            sync.RWMutex
}

// NewWebhookManager initializes the webhook manager
func NewWebhookManager(db *sql.DB, rmq *RabbitMQ) *WebhookManager {
	return &WebhookManager{
		db:  db,
		rmq: rmq,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		subscriptions: make(map[string]*WebhookSubscription),
		deliveries:    make([]*WebhookDelivery, 0),
	}
}

// CreateSubscription registers a new partner webhook subscription
func (wm *WebhookManager) CreateSubscription(sub *WebhookSubscription) (*WebhookSubscription, error) {
	if sub.TargetURL == "" {
		return nil, errors.New("targetUrl is required")
	}
	if len(sub.Events) == 0 {
		return nil, errors.New("at least one event must be subscribed")
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	if sub.ID == "" {
		sub.ID = generateRandomID("sub")
	}
	if sub.EventID == "" {
		sub.EventID = "*"
	}
	if sub.Secret == "" {
		sub.Secret = generateRandomID("sec")
	}
	sub.Active = true
	now := time.Now().UTC()
	sub.CreatedAt = now
	sub.UpdatedAt = now

	// Clone to store
	stored := *sub
	wm.subscriptions[sub.ID] = &stored
	return &stored, nil
}

// GetSubscription retrieves a subscription by ID
func (wm *WebhookManager) GetSubscription(id string) (*WebhookSubscription, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	sub, ok := wm.subscriptions[id]
	if !ok {
		return nil, fmt.Errorf("subscription %s not found", id)
	}
	copy := *sub
	return &copy, nil
}

// ListSubscriptions returns active subscriptions optionally filtered by eventId
func (wm *WebhookManager) ListSubscriptions(eventID string) []*WebhookSubscription {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	results := make([]*WebhookSubscription, 0, len(wm.subscriptions))
	for _, sub := range wm.subscriptions {
		if eventID != "" && sub.EventID != "*" && sub.EventID != eventID {
			continue
		}
		copy := *sub
		results = append(results, &copy)
	}
	return results
}

// DeleteSubscription removes a webhook subscription
func (wm *WebhookManager) DeleteSubscription(id string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if _, ok := wm.subscriptions[id]; !ok {
		return fmt.Errorf("subscription %s not found", id)
	}
	delete(wm.subscriptions, id)
	return nil
}

// ListDeliveries retrieves audit delivery records
func (wm *WebhookManager) ListDeliveries(subID string, limit int) []*WebhookDelivery {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	results := make([]*WebhookDelivery, 0)
	for i := len(wm.deliveries) - 1; i >= 0 && len(results) < limit; i-- {
		d := wm.deliveries[i]
		if subID != "" && d.SubscriptionID != subID {
			continue
		}
		copy := *d
		results = append(results, &copy)
	}
	return results
}

// DispatchEvent sends an HTTP webhook notification to all matching partner subscriptions
// and publishes domain events asynchronously to RabbitMQ (R22 & R26)
func (wm *WebhookManager) DispatchEvent(eventID, eventType string, data map[string]interface{}) ([]*WebhookDelivery, error) {
	// 1. Asynchronous domain event publishing to RabbitMQ (R22)
	if wm.rmq != nil && wm.rmq.channel != nil {
		go func() {
			routingKey := fmt.Sprintf("domain.event.%s", strings.ReplaceAll(eventType, ".", "_"))
			eventMsg := map[string]interface{}{
				"eventId":   eventID,
				"eventType": eventType,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"data":      data,
			}
			payloadBytes, _ := json.Marshal(eventMsg)
			_ = wm.rmq.channel.Publish(
				SagaExchange,
				routingKey,
				false,
				false,
				amqp.Publishing{
					ContentType:  "application/json",
					DeliveryMode: amqp.Persistent,
					Timestamp:    time.Now().UTC(),
					Body:         payloadBytes,
				},
			)
		}()
	}

	// 2. HTTP Webhook dispatching to registered partners (R26)
	wm.mu.RLock()
	matchingSubs := make([]*WebhookSubscription, 0)
	for _, sub := range wm.subscriptions {
		if !sub.Active {
			continue
		}
		if sub.EventID != "*" && sub.EventID != eventID {
			continue
		}
		eventMatched := false
		for _, e := range sub.Events {
			if e == "*" || e == eventType {
				eventMatched = true
				break
			}
		}
		if eventMatched {
			copy := *sub
			matchingSubs = append(matchingSubs, &copy)
		}
	}
	wm.mu.RUnlock()

	deliveries := make([]*WebhookDelivery, 0, len(matchingSubs))
	payload := WebhookPayload{
		EventID:   eventID,
		EventType: eventType,
		Timestamp: time.Now().UTC(),
		Data:      data,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	for _, sub := range matchingSubs {
		delivery := wm.sendWebhook(sub, eventType, payloadBytes)
		deliveries = append(deliveries, delivery)
	}

	return deliveries, nil
}

// sendWebhook executes the HTTP POST request to partner with HMAC signature
func (wm *WebhookManager) sendWebhook(sub *WebhookSubscription, eventType string, payloadBytes []byte) *WebhookDelivery {
	deliveryID := generateRandomID("dlv")
	sig := computeHMACSignature(sub.Secret, payloadBytes)
	startTime := time.Now()

	req, err := http.NewRequest(http.MethodPost, sub.TargetURL, bytes.NewReader(payloadBytes))
	if err != nil {
		d := &WebhookDelivery{
			ID:             deliveryID,
			SubscriptionID: sub.ID,
			EventID:        sub.EventID,
			EventType:      eventType,
			TargetURL:      sub.TargetURL,
			StatusCode:     0,
			DurationMs:     time.Since(startTime).Milliseconds(),
			Success:        false,
			ErrorMessage:   err.Error(),
			PayloadJSON:    string(payloadBytes),
			CreatedAt:      time.Now().UTC(),
		}
		wm.recordDelivery(d)
		return d
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TicketVerse-Webhook-Dispatcher/1.0")
	req.Header.Set("X-Webhook-ID", deliveryID)
	req.Header.Set("X-Webhook-Event", eventType)
	req.Header.Set("X-Webhook-Timestamp", fmt.Sprintf("%d", startTime.Unix()))
	req.Header.Set("X-Webhook-Signature", sig)

	resp, err := wm.httpClient.Do(req)
	duration := time.Since(startTime).Milliseconds()

	delivery := &WebhookDelivery{
		ID:             deliveryID,
		SubscriptionID: sub.ID,
		EventID:        sub.EventID,
		EventType:      eventType,
		TargetURL:      sub.TargetURL,
		DurationMs:     duration,
		PayloadJSON:    string(payloadBytes),
		CreatedAt:      time.Now().UTC(),
	}

	if err != nil {
		delivery.StatusCode = 0
		delivery.Success = false
		delivery.ErrorMessage = err.Error()
		log.Printf("[Webhook] Delivery %s to %s failed: %v", deliveryID, sub.TargetURL, err)
	} else {
		defer resp.Body.Close()
		delivery.StatusCode = resp.StatusCode
		delivery.Success = (resp.StatusCode >= 200 && resp.StatusCode < 300)
		if !delivery.Success {
			delivery.ErrorMessage = fmt.Sprintf("HTTP status %d", resp.StatusCode)
		}
	}

	wm.recordDelivery(delivery)
	return delivery
}

func (wm *WebhookManager) recordDelivery(d *WebhookDelivery) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.deliveries = append(wm.deliveries, d)
	// Cap audit log length to 500
	if len(wm.deliveries) > 500 {
		wm.deliveries = wm.deliveries[len(wm.deliveries)-500:]
	}
}

// HTTP Handlers on Server

func (s *Server) handleWebhookSubscriptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.webhookManager == nil {
		s.webhookManager = NewWebhookManager(s.db, s.rmq)
	}

	switch r.Method {
	case http.MethodPost:
		var sub WebhookSubscription
		if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_JSON", "message": err.Error()})
			return
		}
		created, err := s.webhookManager.CreateSubscription(&sub)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "SUBSCRIPTION_FAILED", "message": err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)

	case http.MethodGet:
		eventID := r.URL.Query().Get("eventId")
		subs := s.webhookManager.ListSubscriptions(eventID)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(subs)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "METHOD_NOT_ALLOWED"})
	}
}

func (s *Server) handleWebhookSubscriptionByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.webhookManager == nil {
		s.webhookManager = NewWebhookManager(s.db, s.rmq)
	}

	// Extract subscription ID from path
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	subID := parts[len(parts)-1]

	switch r.Method {
	case http.MethodGet:
		sub, err := s.webhookManager.GetSubscription(subID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "NOT_FOUND", "message": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(sub)

	case http.MethodDelete:
		err := s.webhookManager.DeleteSubscription(subID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "NOT_FOUND", "message": err.Error()})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"deleted": true, "id": subID})

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "METHOD_NOT_ALLOWED"})
	}
}

func (s *Server) handleWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.webhookManager == nil {
		s.webhookManager = NewWebhookManager(s.db, s.rmq)
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "METHOD_NOT_ALLOWED"})
		return
	}

	subID := r.URL.Query().Get("subscriptionId")
	deliveries := s.webhookManager.ListDeliveries(subID, 50)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(deliveries)
}

func (s *Server) handleWebhookTestDispatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.webhookManager == nil {
		s.webhookManager = NewWebhookManager(s.db, s.rmq)
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "METHOD_NOT_ALLOWED"})
		return
	}

	var req struct {
		EventID   string                 `json:"eventId"`
		EventType string                 `json:"eventType"`
		Data      map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "INVALID_JSON", "message": err.Error()})
		return
	}
	if req.EventType == "" {
		req.EventType = "event.capacity_reached"
	}
	if req.EventID == "" {
		req.EventID = "*"
	}
	if req.Data == nil {
		req.Data = make(map[string]interface{})
	}

	deliveries, err := s.webhookManager.DispatchEvent(req.EventID, req.EventType, req.Data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "DISPATCH_FAILED", "message": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"dispatched": len(deliveries),
		"deliveries": deliveries,
	})
}
