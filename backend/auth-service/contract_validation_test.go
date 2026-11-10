package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContractValidation_Register(t *testing.T) {
	srv := &Server{
		jwtSecret: []byte("test-jwt-secret"),
	}

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
		description    string
	}{
		{
			name:           "POST /auth/register Violation A - Unknown field 'username'",
			payload:        `{"username":"alice","email":"alice@example.com","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'username'",
		},
		{
			name:           "POST /auth/register Violation A - Unknown field 'pass'",
			payload:        `{"name":"Alice","email":"alice@example.com","pass":"password123"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'pass'",
		},
		{
			name:           "POST /auth/register Violation B - Wrong field type for email (number instead of string)",
			payload:        `{"name":"Alice","email":12345,"password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for email should return 400 Bad Request",
		},
		{
			name:           "POST /auth/register Violation B - Wrong field type for password (bool instead of string)",
			payload:        `{"name":"Alice","email":"alice@example.com","password":true}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for password should return 400 Bad Request",
		},
		{
			name:           "POST /auth/register Violation C - Missing required email",
			payload:        `{"name":"Alice","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing email should return 400 Bad Request",
		},
		{
			name:           "POST /auth/register Violation C - Missing required password",
			payload:        `{"name":"Alice","email":"alice@example.com"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing password should return 400 Bad Request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.handleRegister(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("%s: expected HTTP %d, got %d. Body: %s", tc.description, tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestContractValidation_Login(t *testing.T) {
	srv := &Server{
		jwtSecret: []byte("test-jwt-secret"),
	}

	tests := []struct {
		name           string
		payload        string
		expectedStatus int
		description    string
	}{
		{
			name:           "POST /auth/login Violation A - Unknown field 'username'",
			payload:        `{"username":"alice@example.com","password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'username'",
		},
		{
			name:           "POST /auth/login Violation A - Unknown field 'remember_me'",
			payload:        `{"email":"alice@example.com","password":"password123","remember_me":true}`,
			expectedStatus: http.StatusBadRequest,
			description:    "DisallowUnknownFields should reject unknown field 'remember_me'",
		},
		{
			name:           "POST /auth/login Violation B - Wrong field type for password (int instead of string)",
			payload:        `{"email":"alice@example.com","password":123456}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Type mismatch for password should return 400 Bad Request",
		},
		{
			name:           "POST /auth/login Violation C - Missing required email",
			payload:        `{"password":"password123"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing email should return 400 Bad Request",
		},
		{
			name:           "POST /auth/login Violation C - Missing required password",
			payload:        `{"email":"alice@example.com"}`,
			expectedStatus: http.StatusBadRequest,
			description:    "Missing password should return 400 Bad Request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.handleLogin(w, req)

			if w.Code != tc.expectedStatus {
				t.Fatalf("%s: expected HTTP %d, got %d. Body: %s", tc.description, tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}
