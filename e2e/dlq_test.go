package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

type DLQStatsResponse struct {
	QueueName          string `json:"queueName"`
	MessageCount       int    `json:"messageCount"`
	ConsumerCount      int    `json:"consumerCount"`
	DeadLetterExchange string `json:"deadLetterExchange"`
}

type DLQMessageItem struct {
	MessageID          string                 `json:"messageId"`
	OriginalExchange   string                 `json:"originalExchange"`
	OriginalRoutingKey string                 `json:"originalRoutingKey"`
	DeathReason        string                 `json:"deathReason"`
	ErrorMessage       string                 `json:"errorMessage"`
	RetryCount         int                    `json:"retryCount"`
	Payload            string                 `json:"payload"`
	PayloadJSON        map[string]interface{} `json:"payloadJson"`
	Timestamp          time.Time              `json:"timestamp"`
}

type DLQMessagesResponse struct {
	QueueName string           `json:"queueName"`
	Count     int              `json:"count"`
	Messages  []DLQMessageItem `json:"messages"`
}

func purgeDLQ(t *testing.T) {
	req, _ := http.NewRequest(http.MethodDelete, "http://localhost:8080/api/dlq/messages", nil)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to purge DLQ: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to purge DLQ, got status %d: %s", resp.StatusCode, string(body))
	}
}

func TestDLQTopologyAndStats(t *testing.T) {
	purgeDLQ(t)

	resp, err := http.Get("http://localhost:8080/api/dlq/stats")
	if err != nil {
		t.Fatalf("Failed to query DLQ stats: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200 OK for DLQ stats, got %d: %s", resp.StatusCode, string(body))
	}

	var stats DLQStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode DLQ stats response: %v", err)
	}

	if stats.QueueName != "saga_dead_letter_queue" {
		t.Errorf("Expected queueName 'saga_dead_letter_queue', got '%s'", stats.QueueName)
	}
	if stats.DeadLetterExchange != "saga_dead_letter_exchange" {
		t.Errorf("Expected deadLetterExchange 'saga_dead_letter_exchange', got '%s'", stats.DeadLetterExchange)
	}

	t.Logf("DLQ topology verified: Queue=%s, DLX=%s, CurrentMessages=%d", stats.QueueName, stats.DeadLetterExchange, stats.MessageCount)
}

func TestDLQPoisonMessageRejection(t *testing.T) {
	purgeDLQ(t)

	// Simulate injection of a malformed/corrupted non-JSON poison message
	poisonPayload := map[string]interface{}{
		"type":       "poison",
		"routingKey": "inventory.reserve",
		"payload":    "THIS_IS_CORRUPTED_NON_JSON_DATA_!!!",
	}
	payloadBytes, _ := json.Marshal(poisonPayload)

	resp, err := http.Post("http://localhost:8080/api/dlq/simulate", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("Failed to simulate poison message: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200 OK for simulate endpoint, got %d: %s", resp.StatusCode, string(body))
	}

	// Allow consumer worker brief moment to process and route to DLQ
	time.Sleep(300 * time.Millisecond)

	// Query DLQ messages
	respMsgs, err := http.Get("http://localhost:8080/api/dlq/messages")
	if err != nil {
		t.Fatalf("Failed to fetch DLQ messages: %v", err)
	}
	defer respMsgs.Body.Close()

	var dlqResp DLQMessagesResponse
	if err := json.NewDecoder(respMsgs.Body).Decode(&dlqResp); err != nil {
		t.Fatalf("Failed to decode DLQ messages: %v", err)
	}

	if dlqResp.Count != 1 {
		t.Fatalf("Expected exactly 1 dead-lettered poison message, found %d", dlqResp.Count)
	}

	deadMsg := dlqResp.Messages[0]
	if deadMsg.DeathReason != "UNPROCESSABLE_PAYLOAD" {
		t.Errorf("Expected deathReason 'UNPROCESSABLE_PAYLOAD', got '%s'", deadMsg.DeathReason)
	}
	if deadMsg.OriginalRoutingKey != "inventory.reserve" {
		t.Errorf("Expected original routing key 'inventory.reserve', got '%s'", deadMsg.OriginalRoutingKey)
	}
	if deadMsg.Payload != "THIS_IS_CORRUPTED_NON_JSON_DATA_!!!" {
		t.Errorf("Expected payload to match corrupted string, got '%s'", deadMsg.Payload)
	}

	t.Logf("Poison message successfully routed to DLQ: Reason=%s, RoutingKey=%s, Error=%s",
		deadMsg.DeathReason, deadMsg.OriginalRoutingKey, deadMsg.ErrorMessage)
}

func TestDLQMaxRetriesExceeded(t *testing.T) {
	purgeDLQ(t)

	// Inject a retriable message that triggers simulated transient failures
	simPayload := map[string]interface{}{
		"type":         "retry_fail",
		"routingKey":   "inventory.reserve",
		"payload":      map[string]interface{}{"saga_id": "saga-e2e-retry-test-999", "tierId": "tier-vip"},
		"errorMessage": "Transient database timeout",
	}
	payloadBytes, _ := json.Marshal(simPayload)

	resp, err := http.Post("http://localhost:8080/api/dlq/simulate", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("Failed to simulate retry failure: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected 200 OK for simulate endpoint, got %d: %s", resp.StatusCode, string(body))
	}

	// 3 retries with 100ms backoff each = ~400ms. Wait 800ms to allow all retries to exhaust.
	time.Sleep(800 * time.Millisecond)

	respMsgs, err := http.Get("http://localhost:8080/api/dlq/messages")
	if err != nil {
		t.Fatalf("Failed to fetch DLQ messages: %v", err)
	}
	defer respMsgs.Body.Close()

	var dlqResp DLQMessagesResponse
	if err := json.NewDecoder(respMsgs.Body).Decode(&dlqResp); err != nil {
		t.Fatalf("Failed to decode DLQ messages: %v", err)
	}

	if dlqResp.Count != 1 {
		t.Fatalf("Expected exactly 1 message in DLQ after retry exhaustion, found %d", dlqResp.Count)
	}

	msg := dlqResp.Messages[0]
	if msg.DeathReason != "MAX_RETRIES_EXCEEDED" {
		t.Errorf("Expected deathReason 'MAX_RETRIES_EXCEEDED', got '%s'", msg.DeathReason)
	}
	if msg.RetryCount != 3 {
		t.Errorf("Expected retryCount 3, got %d", msg.RetryCount)
	}
	if msg.PayloadJSON["saga_id"] != "saga-e2e-retry-test-999" {
		t.Errorf("Expected saga_id 'saga-e2e-retry-test-999', got '%v'", msg.PayloadJSON["saga_id"])
	}

	t.Logf("Message exceeded max retries and moved to DLQ: RetryCount=%d, DeathReason=%s, ErrorMessage=%s",
		msg.RetryCount, msg.DeathReason, msg.ErrorMessage)
}

func TestDLQPeekAndInspection(t *testing.T) {
	purgeDLQ(t)

	// Inject 2 test messages directly to DLQ
	for i := 1; i <= 2; i++ {
		simPayload := map[string]interface{}{
			"type":         "direct_dlq",
			"routingKey":   fmt.Sprintf("inventory.order.%d", i),
			"payload":      map[string]interface{}{"orderNumber": i, "status": "FAILED"},
			"errorMessage": fmt.Sprintf("Inspection test failure %d", i),
		}
		pBytes, _ := json.Marshal(simPayload)
		resp, err := http.Post("http://localhost:8080/api/dlq/simulate", "application/json", bytes.NewReader(pBytes))
		if err != nil {
			t.Fatalf("Failed to inject DLQ message %d: %v", i, err)
		}
		resp.Body.Close()
	}

	time.Sleep(200 * time.Millisecond)

	// First peek
	resp1, err := http.Get("http://localhost:8080/api/dlq/messages")
	if err != nil {
		t.Fatalf("Peek 1 failed: %v", err)
	}
	defer resp1.Body.Close()
	var dlq1 DLQMessagesResponse
	json.NewDecoder(resp1.Body).Decode(&dlq1)

	if dlq1.Count != 2 {
		t.Fatalf("Expected 2 messages on first peek, got %d", dlq1.Count)
	}

	// Second peek: Assert messages were NOT removed by peek operation
	resp2, err := http.Get("http://localhost:8080/api/dlq/messages")
	if err != nil {
		t.Fatalf("Peek 2 failed: %v", err)
	}
	defer resp2.Body.Close()
	var dlq2 DLQMessagesResponse
	json.NewDecoder(resp2.Body).Decode(&dlq2)

	if dlq2.Count != 2 {
		t.Fatalf("Peek must be non-destructive! Expected 2 messages remaining, got %d", dlq2.Count)
	}

	t.Logf("Non-destructive DLQ inspection verified: 2 messages inspected and preserved in queue")
}

func TestDLQReprocessAndRedrive(t *testing.T) {
	purgeDLQ(t)

	// Inject a message into DLQ
	simPayload := map[string]interface{}{
		"type":         "direct_dlq",
		"routingKey":   "inventory.release",
		"payload":      map[string]interface{}{"saga_id": "saga-reprocess-test-456", "reservationId": "res-456"},
		"errorMessage": "Simulated redrive test",
	}
	pBytes, _ := json.Marshal(simPayload)
	resp, _ := http.Post("http://localhost:8080/api/dlq/simulate", "application/json", bytes.NewReader(pBytes))
	resp.Body.Close()

	time.Sleep(200 * time.Millisecond)

	// Call POST /api/dlq/retry
	retryResp, err := http.Post("http://localhost:8080/api/dlq/retry", "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to trigger DLQ retry: %v", err)
	}
	defer retryResp.Body.Close()

	if retryResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(retryResp.Body)
		t.Fatalf("Expected 200 OK for DLQ retry, got %d: %s", retryResp.StatusCode, string(body))
	}

	var result map[string]interface{}
	json.NewDecoder(retryResp.Body).Decode(&result)

	reprocessed, _ := result["reprocessed"].(float64)
	if int(reprocessed) != 1 {
		t.Fatalf("Expected 1 message reprocessed, got %v", result["reprocessed"])
	}

	time.Sleep(200 * time.Millisecond)

	// Assert DLQ is now empty
	msgsResp, _ := http.Get("http://localhost:8080/api/dlq/messages")
	defer msgsResp.Body.Close()
	var after DLQMessagesResponse
	json.NewDecoder(msgsResp.Body).Decode(&after)

	if after.Count != 0 {
		t.Fatalf("Expected DLQ to be empty after reprocessing, found %d messages", after.Count)
	}

	t.Logf("DLQ message redrive passed: 1 message successfully re-published to saga_exchange and cleared from DLQ")
}
