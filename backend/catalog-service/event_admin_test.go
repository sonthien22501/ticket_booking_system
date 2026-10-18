package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleEventStatusUpdate_MethodNotAllowed(t *testing.T) {
	s := &Server{db: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/events/evt-101/status", nil)
	rr := httptest.NewRecorder()

	s.handleEventStatusUpdate(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", rr.Code)
	}
}

func TestHandleEventStatusUpdate_BadPayload(t *testing.T) {
	s := &Server{db: nil}

	req := httptest.NewRequest(http.MethodPut, "/api/events/evt-101/status", bytes.NewBuffer([]byte("{invalid-json")))
	rr := httptest.NewRecorder()

	s.handleEventStatusUpdate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", rr.Code)
	}
}
