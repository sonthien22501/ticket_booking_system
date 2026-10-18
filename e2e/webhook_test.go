package e2e

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type WebhookSubscriptionResponse struct {
	ID        string    `json:"id"`
	EventID   string    `json:"eventId"`
	TargetURL string    `json:"targetUrl"`
	Secret    string    `json:"secret,omitempty"`
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type WebhookDeliveryResponse struct {
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

func computeTestHMAC(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}

func TestWebhookSubscriptionLifecycleE2E(t *testing.T) {
	// 1. Create a webhook subscription
	createBody := map[string]interface{}{
		"eventId":   "evt-live-concert",
		"targetUrl": "https://partner-portal.example.com/api/v1/callbacks",
		"secret":    "webhook-secret-xyz-456",
		"events":    []string{"event.capacity_reached", "event.revenue_threshold"},
	}
	bodyBytes, _ := json.Marshal(createBody)

	resp, err := http.Post(gatewayURL+"/webhooks/subscriptions", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("Create webhook subscription request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 201 Created, got %d: %s", resp.StatusCode, string(raw))
	}

	var created WebhookSubscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to decode subscription response: %v", err)
	}

	if created.ID == "" {
		t.Errorf("Expected subscription ID to be generated, got empty")
	}
	if !created.Active {
		t.Errorf("Expected subscription to be active")
	}

	// 2. Query subscription by ID
	getResp, err := http.Get(fmt.Sprintf("%s/webhooks/subscriptions/%s", gatewayURL, created.ID))
	if err != nil {
		t.Fatalf("Get subscription failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", getResp.StatusCode)
	}

	var fetched WebhookSubscriptionResponse
	_ = json.NewDecoder(getResp.Body).Decode(&fetched)
	if fetched.ID != created.ID {
		t.Errorf("Expected fetched ID %s, got %s", created.ID, fetched.ID)
	}

	// 3. List subscriptions
	listResp, err := http.Get(gatewayURL + "/webhooks/subscriptions")
	if err != nil {
		t.Fatalf("List subscriptions failed: %v", err)
	}
	defer listResp.Body.Close()

	var subsList []WebhookSubscriptionResponse
	_ = json.NewDecoder(listResp.Body).Decode(&subsList)
	found := false
	for _, s := range subsList {
		if s.ID == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Created subscription %s not found in subscriptions list", created.ID)
	}

	// 4. Delete subscription
	reqDel, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/webhooks/subscriptions/%s", gatewayURL, created.ID), nil)
	delResp, err := http.DefaultClient.Do(reqDel)
	if err != nil {
		t.Fatalf("Delete subscription request failed: %v", err)
	}
	defer delResp.Body.Close()

	if delResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK on deletion, got %d", delResp.StatusCode)
	}

	// 5. Verify deleted returns 404
	afterDelResp, err := http.Get(fmt.Sprintf("%s/webhooks/subscriptions/%s", gatewayURL, created.ID))
	if err != nil {
		t.Fatalf("Check deleted subscription failed: %v", err)
	}
	defer afterDelResp.Body.Close()
	if afterDelResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found after deletion, got %d", afterDelResp.StatusCode)
	}
}

func TestWebhookPartnerDispatchAndHMACVerificationE2E(t *testing.T) {
	partnerSecret := "hmac-secret-partner-integration-2026"
	var mu sync.Mutex
	receivedCallbacks := make([]map[string]interface{}, 0)
	receivedSignatures := make([]string, 0)
	rawBodies := make([][]byte, 0)

	// Mock partner callback receiver server
	mockPartner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig := r.Header.Get("X-Webhook-Signature")
		body, _ := io.ReadAll(r.Body)

		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)

		mu.Lock()
		receivedSignatures = append(receivedSignatures, sig)
		receivedCallbacks = append(receivedCallbacks, payload)
		rawBodies = append(rawBodies, body)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ACK"}`))
	}))
	defer mockPartner.Close()

	// Register subscription pointing to mock partner server
	subBody := map[string]interface{}{
		"eventId":   "evt-championship-2026",
		"targetUrl": mockPartner.URL,
		"secret":    partnerSecret,
		"events":    []string{"event.capacity_reached", "event.revenue_threshold"},
	}
	subJSON, _ := json.Marshal(subBody)
	resp, err := http.Post(gatewayURL+"/webhooks/subscriptions", "application/json", bytes.NewReader(subJSON))
	if err != nil {
		t.Fatalf("Failed to create partner subscription: %v", err)
	}
	defer resp.Body.Close()

	var sub WebhookSubscriptionResponse
	_ = json.NewDecoder(resp.Body).Decode(&sub)
	defer func() {
		// Clean up
		reqDel, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/webhooks/subscriptions/%s", gatewayURL, sub.ID), nil)
		_, _ = http.DefaultClient.Do(reqDel)
	}()

	// 1. Dispatch event.capacity_reached
	dispatchBody1 := map[string]interface{}{
		"eventId":   "evt-championship-2026",
		"eventType": "event.capacity_reached",
		"data": map[string]interface{}{
			"remainingSeats": 0,
			"soldOut":        true,
			"totalCapacity":  25000,
		},
	}
	dispatchJSON1, _ := json.Marshal(dispatchBody1)
	dResp1, err := http.Post(gatewayURL+"/webhooks/dispatch", "application/json", bytes.NewReader(dispatchJSON1))
	if err != nil {
		t.Fatalf("Dispatch capacity event failed: %v", err)
	}
	defer dResp1.Body.Close()

	if dResp1.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK on dispatch, got %d", dResp1.StatusCode)
	}

	// 2. Dispatch event.revenue_threshold
	dispatchBody2 := map[string]interface{}{
		"eventId":   "evt-championship-2026",
		"eventType": "event.revenue_threshold",
		"data": map[string]interface{}{
			"revenue":   1500000.0,
			"threshold": 1000000.0,
			"currency":  "USD",
		},
	}
	dispatchJSON2, _ := json.Marshal(dispatchBody2)
	dResp2, err := http.Post(gatewayURL+"/webhooks/dispatch", "application/json", bytes.NewReader(dispatchJSON2))
	if err != nil {
		t.Fatalf("Dispatch revenue event failed: %v", err)
	}
	defer dResp2.Body.Close()

	if dResp2.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK on revenue dispatch, got %d", dResp2.StatusCode)
	}

	// Verify mock partner received both callbacks
	mu.Lock()
	defer mu.Unlock()

	if len(receivedCallbacks) != 2 {
		t.Fatalf("Expected 2 callbacks received, got %d", len(receivedCallbacks))
	}

	// Verify HMAC signatures match
	for i, body := range rawBodies {
		expectedSig := computeTestHMAC(partnerSecret, body)
		if receivedSignatures[i] != expectedSig {
			t.Errorf("Callback #%d: expected signature %s, got %s", i, expectedSig, receivedSignatures[i])
		}
	}

	// Verify audit deliveries query
	delivResp, err := http.Get(fmt.Sprintf("%s/webhooks/deliveries?subscriptionId=%s", gatewayURL, sub.ID))
	if err != nil {
		t.Fatalf("Failed to query deliveries: %v", err)
	}
	defer delivResp.Body.Close()

	var deliveries []WebhookDeliveryResponse
	_ = json.NewDecoder(delivResp.Body).Decode(&deliveries)
	if len(deliveries) != 2 {
		t.Errorf("Expected 2 audit delivery records, got %d", len(deliveries))
	}
	for _, d := range deliveries {
		if !d.Success {
			t.Errorf("Expected delivery to succeed, got failure: %s", d.ErrorMessage)
		}
		if d.StatusCode != http.StatusOK {
			t.Errorf("Expected HTTP 200 status code in delivery log, got %d", d.StatusCode)
		}
	}
}
