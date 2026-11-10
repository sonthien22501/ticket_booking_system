package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestContractValidation_CreateEvent(t *testing.T) {
	srv := &Server{
		startTime:           srvStartTime(),
		inventoryServiceURL: "http://inventory-service:8082",
		httpClient:          &http.Client{},
	}

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
		description    string
	}{
		{
			name:           "Violation A - Unknown field identifier 'venue'",
			payload:        `{"title":"Rock Fest","venue":"Main Stadium","venueLocation":"Berlin"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject 'venue' (expected 'venueName')",
		},
		{
			name:           "Violation A - Unknown field identifier 'ticket_price'",
			payload:        `{"title":"Rock Fest","venueName":"Main Stadium","ticket_price":100}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unexpected root fields",
		},
		{
			name:           "Violation B - Wrong field type for tiers price (string instead of float)",
			payload:        `{"title":"Rock Fest","venueName":"Main Stadium","tiers":[{"id":"t1","name":"VIP","price":"100","capacity":50}]}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch should fail strict JSON decoding",
		},
		{
			name:           "Violation B - Wrong field type for category (int instead of string)",
			payload:        `{"title":"Rock Fest","venueName":"Main Stadium","category":12345}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for category should fail strict JSON decoding",
		},
		{
			name:           "Violation C - Missing required field 'title'",
			payload:        `{"venueName":"Main Stadium","venueLocation":"Berlin"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing required title should return 400 Bad Request",
		},
		{
			name:           "Violation C - Missing required field 'venueName'",
			payload:        `{"title":"Rock Fest","venueLocation":"Berlin"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing required venueName should return 400 Bad Request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.createEvent(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("%s: expected HTTP %d, got %d. Body: %s", tc.description, tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestContractValidation_EventStatusUpdate(t *testing.T) {
	srv := &Server{
		startTime: srvStartTime(),
	}

	tests := []struct {
		name           string
		method         string
		payload        string
		expectedStatus int
		description    string
	}{
		{
			name:           "PUT Violation A - Unknown field identifier 'state'",
			method:         http.MethodPut,
			payload:        `{"state":"ACTIVE"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'state'",
		},
		{
			name:           "PATCH Violation A - Unknown field identifier 'event_status'",
			method:         http.MethodPatch,
			payload:        `{"event_status":"ACTIVE"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'event_status'",
		},
		{
			name:           "PUT Violation B - Wrong field type (int instead of string)",
			method:         http.MethodPut,
			payload:        `{"status":123}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for status should return 400 Bad Request",
		},
		{
			name:           "PATCH Violation B - Wrong field type (bool instead of string)",
			method:         http.MethodPatch,
			payload:        `{"status":true}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for status should return 400 Bad Request",
		},
		{
			name:           "PUT Violation C - Missing required field 'status'",
			method:         http.MethodPut,
			payload:        `{}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing required status should return 400 Bad Request",
		},
		{
			name:           "PATCH Violation C - Invalid status enum value",
			method:         http.MethodPatch,
			payload:        `{"status":"NOT_A_VALID_STATUS"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Invalid enum status should return 400 Bad Request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/events/evt-test-101/status", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.handleEventStatusUpdate(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("%s: expected HTTP %d, got %d. Body: %s", tc.description, tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func srvStartTime() time.Time {
	return time.Now()
}
