package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"
)

type RateLimitResponse struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	RetryAfter int    `json:"retryAfter"`
}

func TestBookingRateLimitingAntiBot(t *testing.T) {
	// Give previous tests a brief moment to settle any burst tokens
	time.Sleep(1500 * time.Millisecond)

	const totalRequests = 25
	var wg sync.WaitGroup
	wg.Add(totalRequests)

	statusCodeCounts := make(map[int]int)
	var mu sync.Mutex

	var rateLimitedBody RateLimitResponse
	var got429 bool

	for i := 0; i < totalRequests; i++ {
		go func() {
			defer wg.Done()

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get(gatewayURL + "/bookings")
			if err != nil {
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			statusCodeCounts[resp.StatusCode]++
			if resp.StatusCode == http.StatusTooManyRequests && !got429 {
				got429 = true
				json.NewDecoder(resp.Body).Decode(&rateLimitedBody)
			}
			mu.Unlock()
		}()
	}

	wg.Wait()

	t.Logf("Rate limit distribution: %+v", statusCodeCounts)

	if statusCodeCounts[http.StatusTooManyRequests] == 0 {
		t.Fatalf("Expected at least one HTTP 429 Too Many Requests response, got none")
	}

	if rateLimitedBody.Error != "RATE_LIMIT_EXCEEDED" {
		t.Errorf("Expected error code 'RATE_LIMIT_EXCEEDED', got %s", rateLimitedBody.Error)
	}

	if rateLimitedBody.RetryAfter <= 0 {
		t.Errorf("Expected positive retryAfter value, got %d", rateLimitedBody.RetryAfter)
	}

	t.Logf("Successfully verified anti-bot rate limiting with %d blocked requests", statusCodeCounts[http.StatusTooManyRequests])
}

func TestRateLimitRecovery(t *testing.T) {
	// Wait 2 seconds for token bucket to replenish
	time.Sleep(2 * time.Second)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(gatewayURL + "/bookings")
	if err != nil {
		t.Fatalf("Recovery request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 OK after rate limit cooldown, got %d", resp.StatusCode)
	}
}

func TestAuthRateLimiting(t *testing.T) {
	// Wait 1.5 seconds to settle
	time.Sleep(1500 * time.Millisecond)

	payload := map[string]string{
		"email":    "user@tickets.com",
		"password": "wrongpassword",
	}
	body, _ := json.Marshal(payload)

	const totalRequests = 30
	var wg sync.WaitGroup
	wg.Add(totalRequests)

	statusCodeCounts := make(map[int]int)
	var mu sync.Mutex

	for i := 0; i < totalRequests; i++ {
		go func() {
			defer wg.Done()

			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Post(gatewayURL+"/auth/login", "application/json", bytes.NewReader(body))
			if err != nil {
				return
			}
			defer resp.Body.Close()

			mu.Lock()
			statusCodeCounts[resp.StatusCode]++
			mu.Unlock()
		}()
	}

	wg.Wait()

	t.Logf("Auth rate limit distribution: %+v", statusCodeCounts)

	if statusCodeCounts[http.StatusTooManyRequests] == 0 {
		t.Errorf("Expected HTTP 429 on rapid auth attempts to prevent credential stuffing, got %+v", statusCodeCounts)
	}
}
