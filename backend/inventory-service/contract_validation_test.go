package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContractValidation_Reserve(t *testing.T) {
	srv := &Server{}

	tests := []struct {
		name           string
		method         string
		payload        string
		expectedStatus int
		description    string
	}{
		{
			name:           "POST Violation A - Unknown field 'user' instead of 'userId'",
			method:         http.MethodPost,
			payload:        `{"eventId":"evt-1","user":"usr-123","ticketCount":2}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'user'",
		},
		{
			name:           "POST Violation A - Unknown field 'count' instead of 'ticketCount'",
			method:         http.MethodPost,
			payload:        `{"eventId":"evt-1","count":2}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'count'",
		},
		{
			name:           "PATCH Violation A - Unknown field 'user'",
			method:         http.MethodPatch,
			payload:        `{"eventId":"evt-1","user":"usr-123","ticketCount":2}`,
			expectedStatus: http.StatusBadRequest,
			description:    "PATCH should reject unknown field 'user'",
		},
		{
			name:           "POST Violation B - Wrong field type for ticketCount (string instead of int)",
			method:         http.MethodPost,
			payload:        `{"eventId":"evt-1","ticketCount":"2"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for ticketCount should return 400 Bad Request",
		},
		{
			name:           "PATCH Violation B - Wrong field type for ticketCount (string instead of int)",
			method:         http.MethodPatch,
			payload:        `{"eventId":"evt-1","ticketCount":"2"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for ticketCount on PATCH should return 400 Bad Request",
		},
		{
			name:           "POST Violation C - Missing required eventId",
			method:         http.MethodPost,
			payload:        `{"ticketCount":2,"seats":["A1","A2"]}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing eventId should return 400 Bad Request",
		},
		{
			name:           "PATCH Violation C - Missing required eventId",
			method:         http.MethodPatch,
			payload:        `{"ticketCount":2}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing eventId on PATCH should return 400 Bad Request",
		},
		{
			name:           "Method Check - GET not allowed on /inventory/reserve",
			method:         http.MethodGet,
			payload:        `{"eventId":"evt-1"}`,
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "GET on /inventory/reserve should return 405 Method Not Allowed",
		},
		{
			name:           "Method Check - DELETE not allowed on /inventory/reserve",
			method:         http.MethodDelete,
			payload:        `{"eventId":"evt-1"}`,
			expectedStatus: http.StatusMethodNotAllowed,
			description:    "DELETE on /inventory/reserve should return 405 Method Not Allowed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/inventory/reserve", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.handleReserve(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("%s: expected HTTP %d, got %d. Body: %s", tc.description, tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestMethodNotAllowed_InventoryEndpoints(t *testing.T) {
	srv := &Server{}

	endpoints := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		path    string
		method  string
	}{
		{"GET /inventory/commit", srv.handleCommit, "/inventory/commit", http.MethodGet},
		{"DELETE /inventory/commit", srv.handleCommit, "/inventory/commit", http.MethodDelete},
		{"GET /inventory/release", srv.handleRelease, "/inventory/release", http.MethodGet},
		{"DELETE /inventory/release", srv.handleRelease, "/inventory/release", http.MethodDelete},
		{"GET /inventory/cancel", srv.handleCancel, "/inventory/cancel", http.MethodGet},
		{"DELETE /inventory/cancel", srv.handleCancel, "/inventory/cancel", http.MethodDelete},
		{"GET /inventory/provision", srv.handleProvision, "/inventory/provision", http.MethodGet},
	}

	for _, tc := range endpoints {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()

			tc.handler(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405 Method Not Allowed on %s, got %d", tc.name, w.Code)
			}
		})
	}
}
