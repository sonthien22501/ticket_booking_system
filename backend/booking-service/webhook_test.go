package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestWebhookHMACSignature(t *testing.T) {
	secret := "partner-test-secret-1234"
	payload := []byte(`{"eventId":"evt-101","eventType":"event.capacity_reached"}`)

	sig := computeHMACSignature(secret, payload)
	expectedPrefix := "sha256="
	if len(sig) <= len(expectedPrefix) || sig[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("expected signature to start with 'sha256=', got %s", sig)
	}

	// Verify manually
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedHex := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if sig != expectedHex {
		t.Errorf("expected signature %s, got %s", expectedHex, sig)
	}
}

func TestWebhookSubscriptionCRUD(t *testing.T) {
	wm := NewWebhookManager(nil, nil)

	// 1. Create subscription
	sub, err := wm.CreateSubscription(&WebhookSubscription{
		EventID:   "evt-gala-2026",
		TargetURL: "https://partner.example.com/webhook",
		Secret:    "secret-abc",
		Events:    []string{"event.capacity_reached", "event.revenue_threshold"},
	})
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}
	if sub.ID == "" {
		t.Fatalf("expected generated subscription ID, got empty")
	}
	if !sub.Active {
		t.Errorf("expected subscription to be active by default")
	}

	// 2. Get by ID
	fetched, err := wm.GetSubscription(sub.ID)
	if err != nil {
		t.Fatalf("failed to get subscription: %v", err)
	}
	if fetched.TargetURL != sub.TargetURL {
		t.Errorf("expected target url %s, got %s", sub.TargetURL, fetched.TargetURL)
	}

	// 3. List
	subs := wm.ListSubscriptions("")
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}

	// 4. Delete
	err = wm.DeleteSubscription(sub.ID)
	if err != nil {
		t.Fatalf("failed to delete subscription: %v", err)
	}

	// 5. Verify deleted
	_, err = wm.GetSubscription(sub.ID)
	if err == nil {
		t.Errorf("expected error when getting deleted subscription, got nil")
	}
}

func TestWebhookDispatchCapacityAndRevenue(t *testing.T) {
	var mu sync.Mutex
	receivedEvents := make([]map[string]interface{}, 0)
	receivedSignatures := make([]string, 0)

	partnerSecret := "super-secure-partner-key-999"

	// Mock partner receiver
	mockPartnerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sigHeader := r.Header.Get("X-Webhook-Signature")
		bodyBytes, _ := io.ReadAll(r.Body)

		var payload map[string]interface{}
		_ = json.Unmarshal(bodyBytes, &payload)

		mu.Lock()
		receivedSignatures = append(receivedSignatures, sigHeader)
		receivedEvents = append(receivedEvents, payload)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer mockPartnerServer.Close()

	wm := NewWebhookManager(nil, nil)

	// Subscribe to capacity and revenue events
	sub, err := wm.CreateSubscription(&WebhookSubscription{
		EventID:   "evt-summer-festival",
		TargetURL: mockPartnerServer.URL,
		Secret:    partnerSecret,
		Events:    []string{"event.capacity_reached", "event.revenue_threshold"},
	})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	// 1. Dispatch event.capacity_reached
	deliveries1, err := wm.DispatchEvent("evt-summer-festival", "event.capacity_reached", map[string]interface{}{
		"totalCapacity": 1000,
		"soldCount":     1000,
		"status":        "SOLD_OUT",
	})
	if err != nil {
		t.Fatalf("dispatch capacity failed: %v", err)
	}
	if len(deliveries1) != 1 || !deliveries1[0].Success {
		t.Fatalf("expected 1 successful delivery, got %v", deliveries1)
	}

	// 2. Dispatch event.revenue_threshold
	deliveries2, err := wm.DispatchEvent("evt-summer-festival", "event.revenue_threshold", map[string]interface{}{
		"thresholdAmount": 50000.0,
		"currentRevenue":  52400.0,
		"currency":        "USD",
	})
	if err != nil {
		t.Fatalf("dispatch revenue failed: %v", err)
	}
	if len(deliveries2) != 1 || !deliveries2[0].Success {
		t.Fatalf("expected 1 successful delivery, got %v", deliveries2)
	}

	// Assertions on partner receiver
	mu.Lock()
	defer mu.Unlock()

	if len(receivedEvents) != 2 {
		t.Fatalf("expected partner to receive 2 events, got %d", len(receivedEvents))
	}

	if receivedEvents[0]["eventType"] != "event.capacity_reached" {
		t.Errorf("expected first event to be capacity_reached, got %v", receivedEvents[0]["eventType"])
	}
	if receivedEvents[1]["eventType"] != "event.revenue_threshold" {
		t.Errorf("expected second event to be revenue_threshold, got %v", receivedEvents[1]["eventType"])
	}

	if len(receivedSignatures) != 2 || receivedSignatures[0] == "" {
		t.Errorf("expected non-empty HMAC signature headers, got %v", receivedSignatures)
	}

	// Verify delivery audit logs
	allDeliveries := wm.ListDeliveries(sub.ID, 10)
	if len(allDeliveries) != 2 {
		t.Errorf("expected 2 audit delivery records, got %d", len(allDeliveries))
	}
}

func TestWebhookFilterByEventType(t *testing.T) {
	partnerCallCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		partnerCallCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	wm := NewWebhookManager(nil, nil)

	// Subscribe ONLY to capacity_reached
	_, err := wm.CreateSubscription(&WebhookSubscription{
		EventID:   "evt-tech-conf",
		TargetURL: mockServer.URL,
		Secret:    "key-1",
		Events:    []string{"event.capacity_reached"},
	})
	if err != nil {
		t.Fatalf("failed to create subscription: %v", err)
	}

	// Dispatch event.revenue_threshold -> should NOT be delivered
	deliveries, err := wm.DispatchEvent("evt-tech-conf", "event.revenue_threshold", map[string]interface{}{
		"currentRevenue": 10000.0,
	})
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if len(deliveries) != 0 {
		t.Errorf("expected 0 deliveries for un-subscribed event type, got %d", len(deliveries))
	}
	if partnerCallCount != 0 {
		t.Errorf("expected partner not to be called, got call count %d", partnerCallCount)
	}

	// Dispatch event.capacity_reached -> SHOULD be delivered
	deliveries, err = wm.DispatchEvent("evt-tech-conf", "event.capacity_reached", map[string]interface{}{
		"soldCount": 500,
	})
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	if len(deliveries) != 1 {
		t.Errorf("expected 1 delivery, got %d", len(deliveries))
	}
	if partnerCallCount != 1 {
		t.Errorf("expected partner to be called once, got %d", partnerCallCount)
	}
}
