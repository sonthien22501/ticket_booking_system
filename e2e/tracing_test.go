package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// StructuredLogSchema represents the standardized JSON log structure emitted by all microservices
type StructuredLogSchema struct {
	Timestamp     string                 `json:"timestamp"`
	Level         string                 `json:"level"`
	Service       string                 `json:"service"`
	CorrelationID string                 `json:"correlationId"`
	Message       string                 `json:"message"`
	Method        string                 `json:"method,omitempty"`
	Path          string                 `json:"path,omitempty"`
	StatusCode    int                    `json:"statusCode,omitempty"`
	DurationMs    float64                `json:"durationMs,omitempty"`
	RemoteAddr    string                 `json:"remoteAddr,omitempty"`
	Fields        map[string]interface{} `json:"fields,omitempty"`
}

func TestTracePreservationCustomHeader(t *testing.T) {
	customID := fmt.Sprintf("custom-cid-test-%d", time.Now().UnixNano())

	req, err := http.NewRequest(http.MethodGet, "http://localhost:8080/api/events", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-Correlation-ID", customID)
	req.Header.Set("X-Rate-Limit-Bypass", "1")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected HTTP 200, got %d", resp.StatusCode)
	}

	resCorrID := resp.Header.Get("X-Correlation-ID")
	if resCorrID != customID {
		t.Errorf("Expected response X-Correlation-ID %q, got %q", customID, resCorrID)
	}

	resReqID := resp.Header.Get("X-Request-ID")
	if resReqID != customID {
		t.Errorf("Expected response X-Request-ID %q, got %q", customID, resReqID)
	}

	t.Logf("Verified custom correlation ID preservation: %s", resCorrID)
}

func TestTracePreservationRequestIDFallback(t *testing.T) {
	customReqID := fmt.Sprintf("custom-rid-test-%d", time.Now().UnixNano())

	req, err := http.NewRequest(http.MethodGet, "http://localhost:8080/api/events", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-Request-ID", customReqID)
	req.Header.Set("X-Rate-Limit-Bypass", "1")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	resCorrID := resp.Header.Get("X-Correlation-ID")
	if resCorrID != customReqID {
		t.Errorf("Expected X-Correlation-ID fallback to %q, got %q", customReqID, resCorrID)
	}

	t.Logf("Verified X-Request-ID fallback propagation: %s", resCorrID)
}

func TestTraceFallbackAutomaticGeneration(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8080/api/events", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-Rate-Limit-Bypass", "1")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	resCorrID := resp.Header.Get("X-Correlation-ID")
	resReqID := resp.Header.Get("X-Request-ID")

	if resCorrID == "" {
		t.Errorf("Expected non-empty generated X-Correlation-ID")
	}
	if resReqID == "" {
		t.Errorf("Expected non-empty generated X-Request-ID")
	}
	if resCorrID != resReqID {
		t.Errorf("Expected matching generated IDs, got X-Correlation-ID=%q and X-Request-ID=%q", resCorrID, resReqID)
	}

	t.Logf("Verified automatic trace ID generation: %s", resCorrID)
}

func TestDistributedTraceAcrossSaga(t *testing.T) {
	eventID := getBaseEvent(t)
	seats := getAvailableSeats(t, eventID)
	if len(seats) == 0 {
		t.Fatalf("No available seats found for event %s", eventID)
	}
	seatToBook := seats[len(seats)-1]

	customTraceID := fmt.Sprintf("trace-saga-e2e-%d", time.Now().UnixNano())

	bookingReq := map[string]interface{}{
		"eventId":       eventID,
		"tierId":        "tier-gen",
		"quantity":      1,
		"seats":         []string{seatToBook},
		"customerName":  "Trace Saga Tester",
		"customerEmail": "trace-saga@test.com",
		"paymentInfo": map[string]interface{}{
			"cardToken": "tok_visa",
		},
	}
	payloadBytes, _ := json.Marshal(bookingReq)

	req, err := http.NewRequest(http.MethodPost, "http://localhost:8080/api/bookings", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("Failed to create booking request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", customTraceID)
	req.Header.Set("X-Rate-Limit-Bypass", "1")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute booking: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected HTTP 200/201 for booking, got %d", resp.StatusCode)
	}

	resCorrID := resp.Header.Get("X-Correlation-ID")
	if resCorrID != customTraceID {
		t.Errorf("Expected response X-Correlation-ID %q, got %q", customTraceID, resCorrID)
	}

	// Verify distributed trace propagation in booking-service logs
	cmdBooking := exec.Command("docker", "logs", "ticket_booking")
	bookingLogs, err := cmdBooking.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to fetch booking logs: %v", err)
	}

	if !strings.Contains(string(bookingLogs), customTraceID) {
		t.Errorf("Expected booking-service logs to contain correlation ID %s", customTraceID)
	}

	// Verify distributed trace propagation in inventory-service logs
	cmdInv := exec.Command("docker", "logs", "ticket_inventory")
	invLogs, err := cmdInv.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to fetch inventory logs: %v", err)
	}

	if !strings.Contains(string(invLogs), customTraceID) {
		t.Errorf("Expected inventory-service logs to contain correlation ID %s", customTraceID)
	}

	t.Logf("Verified end-to-end distributed trace propagation across Gateway, Booking, and Inventory for ID %s", customTraceID)
}

func TestTracePreservationOnErrors(t *testing.T) {
	customErrTrace := fmt.Sprintf("trace-err-%d", time.Now().UnixNano())

	req, err := http.NewRequest(http.MethodGet, "http://localhost:8080/api/bookings/non-existent-booking-id-9999", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("X-Correlation-ID", customErrTrace)
	req.Header.Set("X-Rate-Limit-Bypass", "1")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to execute error request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", resp.StatusCode)
	}

	resCorrID := resp.Header.Get("X-Correlation-ID")
	if resCorrID != customErrTrace {
		t.Errorf("Expected error response to preserve X-Correlation-ID %q, got %q", customErrTrace, resCorrID)
	}

	t.Logf("Verified trace preservation on HTTP 404 error response: %s", resCorrID)
}

func TestStructuredJSONLogFormat(t *testing.T) {
	cmd := exec.Command("docker", "logs", "--tail", "20", "ticket_catalog")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to inspect catalog logs: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	foundJSONLog := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var entry StructuredLogSchema
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			if entry.Service == "catalog-service" && entry.Timestamp != "" && entry.Level != "" {
				foundJSONLog = true
				if entry.CorrelationID == "" {
					t.Errorf("Expected structured log entry to have non-empty correlationId: %s", line)
				}
				t.Logf("Validated structured log schema: level=%s, service=%s, cid=%s, msg=%s",
					entry.Level, entry.Service, entry.CorrelationID, entry.Message)
				break
			}
		}
	}

	if !foundJSONLog {
		t.Errorf("No valid structured JSON log entries found in ticket_catalog output")
	}
}
