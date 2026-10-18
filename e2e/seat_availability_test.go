package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

type SSEMessage struct {
	Event string
	Data  string
}

func readSSEEvents(scanner *bufio.Scanner, out chan<- SSEMessage, stop <-chan struct{}) {
	var currentEvent string
	var currentData strings.Builder

	for {
		select {
		case <-stop:
			return
		default:
			if !scanner.Scan() {
				return
			}
			line := scanner.Text()
			if strings.HasPrefix(line, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			} else if strings.HasPrefix(line, "data:") {
				currentData.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			} else if line == "" {
				// Event boundary
				if currentEvent != "" || currentData.Len() > 0 {
					out <- SSEMessage{
						Event: currentEvent,
						Data:  currentData.String(),
					}
					currentEvent = ""
					currentData.Reset()
				}
			}
		}
	}
}

func TestRealTimeSeatAvailabilitySnapshot(t *testing.T) {
	resp, err := http.Get("http://localhost:8080/api/inventory/evt-101/seats/realtime")
	if err != nil {
		t.Fatalf("Failed to fetch real-time seats: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for realtime seats, got %d", resp.StatusCode)
	}

	cc := resp.Header.Get("Cache-Control")
	if !strings.Contains(cc, "no-cache") && !strings.Contains(cc, "no-store") {
		t.Errorf("Expected Cache-Control to contain no-cache or no-store, got: %s", cc)
	}

	var body struct {
		EventID string `json:"eventId"`
		Summary struct {
			Total     int `json:"total"`
			Available int `json:"available"`
			Held      int `json:"held"`
			Booked    int `json:"booked"`
		} `json:"summary"`
		Seats []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
			Row    string `json:"row"`
			Number int    `json:"number"`
		} `json:"seats"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Failed to decode realtime seat response: %v", err)
	}

	if body.EventID != "evt-101" {
		t.Errorf("Expected eventId evt-101, got %s", body.EventID)
	}
	if len(body.Seats) == 0 {
		t.Fatalf("Expected non-empty seats array for evt-101")
	}
	if body.Summary.Total != len(body.Seats) {
		t.Errorf("Summary total %d mismatch seat array len %d", body.Summary.Total, len(body.Seats))
	}
}

func TestRealTimeSeatStatusFilter(t *testing.T) {
	resp, err := http.Get("http://localhost:8080/api/inventory/evt-101/seats?status=AVAILABLE")
	if err != nil {
		t.Fatalf("Failed to query available seats: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", resp.StatusCode)
	}

	var body struct {
		Seats []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"seats"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	for _, s := range body.Seats {
		if s.Status != "AVAILABLE" {
			t.Errorf("Seat %s has status %s, expected AVAILABLE only", s.ID, s.Status)
		}
	}
}

func TestRealTimeSeatAvailabilityStreamSSE(t *testing.T) {
	req, err := http.NewRequest("GET", "http://localhost:8080/api/inventory/evt-101/seats/stream", nil)
	if err != nil {
		t.Fatalf("Failed to create SSE request: %v", err)
	}
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to connect to SSE stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK for SSE stream, got %d", resp.StatusCode)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Expected Content-Type text/event-stream, got %s", ct)
	}

	eventsChan := make(chan SSEMessage, 20)
	stopChan := make(chan struct{})
	defer close(stopChan)

	scanner := bufio.NewScanner(resp.Body)
	go readSSEEvents(scanner, eventsChan, stopChan)

	// Step 1: Assert initial_state event is received promptly
	select {
	case msg := <-eventsChan:
		if msg.Event != "initial_state" && msg.Event != "message" {
			t.Logf("Received event: %s data: %s", msg.Event, msg.Data)
		}
		if !strings.Contains(msg.Data, "evt-101") {
			t.Errorf("Expected initial state to contain eventId evt-101, got: %s", msg.Data)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Timed out waiting for initial_state SSE event")
	}

	// Step 2: Trigger seat reservation to observe real-time push event
	targetSeat := "B1"
	reservePayload := map[string]interface{}{
		"eventId":     "evt-101",
		"tierId":      "tier-vip",
		"seats":       []string{targetSeat},
		"holdSeconds": 10,
	}
	bodyBytes, _ := json.Marshal(reservePayload)
	resResp, err := http.Post("http://localhost:8080/api/inventory/reserve", "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		t.Fatalf("Failed to post reservation: %v", err)
	}
	defer resResp.Body.Close()

	if resResp.StatusCode != http.StatusOK && resResp.StatusCode != http.StatusCreated {
		t.Fatalf("Reservation failed with status %d", resResp.StatusCode)
	}

	var resData struct {
		ReservationID string `json:"reservationId"`
	}
	_ = json.NewDecoder(resResp.Body).Decode(&resData)

	// Step 3: Assert SSE receives seat_update event for targetSeat with status HOLD
	receivedUpdate := false
	timeout := time.After(5 * time.Second)

	for !receivedUpdate {
		select {
		case msg := <-eventsChan:
			if msg.Event == "seat_update" && strings.Contains(msg.Data, targetSeat) && strings.Contains(msg.Data, "HOLD") {
				receivedUpdate = true
			}
		case <-timeout:
			t.Fatalf("Timed out waiting for real-time SSE seat_update event for seat %s (HOLD)", targetSeat)
		}
	}

	// Step 4: Release reservation and assert real-time push event for AVAILABLE
	if resData.ReservationID != "" {
		relPayload := map[string]string{
			"reservationId": resData.ReservationID,
		}
		relBytes, _ := json.Marshal(relPayload)
		relResp, err := http.Post("http://localhost:8080/api/inventory/release", "application/json", bytes.NewReader(relBytes))
		if err == nil {
			relResp.Body.Close()
		}

		receivedRelease := false
		relTimeout := time.After(5 * time.Second)
		for !receivedRelease {
			select {
			case msg := <-eventsChan:
				if msg.Event == "seat_update" && strings.Contains(msg.Data, targetSeat) && strings.Contains(msg.Data, "AVAILABLE") {
					receivedRelease = true
				}
			case <-relTimeout:
				t.Fatalf("Timed out waiting for real-time SSE seat_update event for seat %s (AVAILABLE)", targetSeat)
			}
		}
	}
}
