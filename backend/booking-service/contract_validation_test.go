package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContractValidation_Bookings(t *testing.T) {
	srv := &Server{
		httpClient: &http.Client{},
	}

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
		description    string
	}{
		{
			name:           "POST /bookings Violation A - Unknown field 'user' instead of 'userId'",
			payload:        `{"eventId":"evt-1","user":"usr-123","customerEmail":"test@test.com","ticketCount":1}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'user'",
		},
		{
			name:           "POST /bookings Violation A - Unknown field 'price' instead of 'totalAmount'",
			payload:        `{"eventId":"evt-1","userId":"usr-123","customerEmail":"test@test.com","price":100}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'price'",
		},
		{
			name:           "POST /bookings Violation B - Wrong field type for ticketCount (string instead of int)",
			payload:        `{"eventId":"evt-1","customerEmail":"test@test.com","ticketCount":"20"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for ticketCount should return 400 Bad Request",
		},
		{
			name:           "POST /bookings Violation B - Wrong field type for quantity (string instead of int)",
			payload:        `{"eventId":"evt-1","customerEmail":"test@test.com","quantity":"20"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for quantity should return 400 Bad Request",
		},
		{
			name:           "POST /bookings Violation C - Missing required eventId",
			payload:        `{"customerEmail":"test@test.com","ticketCount":1}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing eventId should return 400 Bad Request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/bookings", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.createBooking(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("%s: expected HTTP %d, got %d. Body: %s", tc.description, tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestContractValidation_CurrencyConvert(t *testing.T) {
	srv := &Server{}

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
		description    string
	}{
		{
			name:           "POST /currencies/convert Violation A - Unknown field 'price' instead of 'amount'",
			payload:        `{"price":100,"from":"USD","to":"EUR"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'price'",
		},
		{
			name:           "POST /currencies/convert Violation B - Wrong field type for amount (string instead of float)",
			payload:        `{"amount":"20","from":"USD","to":"EUR"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for amount should return 400 Bad Request",
		},
		{
			name:           "POST /currencies/convert Violation C - Missing required amount",
			payload:        `{"from":"USD","to":"EUR"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing amount should return 400 Bad Request",
		},
		{
			name:           "POST /currencies/convert Violation C - Missing required 'from' currency",
			payload:        `{"amount":100,"to":"EUR"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing 'from' currency should return 400 Bad Request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/currencies/convert", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.handleCurrencyConvert(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("%s: expected HTTP %d, got %d. Body: %s", tc.description, tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestContractValidation_ConciergeAdvise(t *testing.T) {
	srv := &Server{
		catalogServiceURL: "http://catalog-service:8081",
		httpClient:        &http.Client{},
	}

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
		description    string
	}{
		{
			name:           "POST /api/concierge/advise Violation A - Unknown field 'event' instead of 'eventId'",
			payload:        `{"event":"evt-101","prompt":"Recommend me seats"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'event'",
		},
		{
			name:           "POST /api/concierge/advise Violation B - Wrong field type for budgetMax (string instead of float)",
			payload:        `{"eventId":"evt-101","budgetMax":"150"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for budgetMax should return 400 Bad Request",
		},
		{
			name:           "POST /api/concierge/advise Violation C - Missing required eventId",
			payload:        `{"prompt":"Recommend me seats","budgetMax":150}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing eventId should return 400 Bad Request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/concierge/advise", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.handleConciergeAdvise(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("%s: expected HTTP %d, got %d. Body: %s", tc.description, tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestContractValidation_Waitlist(t *testing.T) {
	srv := &Server{
		httpClient: &http.Client{},
	}

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
		description    string
	}{
		{
			name:           "POST /waitlist Violation A - Unknown field 'user' instead of 'userId'",
			payload:        `{"eventId":"evt-101","user":"usr-123","quantity":2}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'user'",
		},
		{
			name:           "POST /waitlist Violation B - Wrong field type for quantity (string instead of int)",
			payload:        `{"eventId":"evt-101","userId":"usr-123","quantity":"20"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for quantity should return 400 Bad Request",
		},
		{
			name:           "POST /waitlist Violation C - Missing required eventId",
			payload:        `{"userId":"usr-123","quantity":2}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing eventId should return 400 Bad Request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/waitlist", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.createWaitlist(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("%s: expected HTTP %d, got %d. Body: %s", tc.description, tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}
