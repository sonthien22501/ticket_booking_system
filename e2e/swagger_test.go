package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const gatewayBaseURL = "http://localhost:8080"

func TestSwaggerUIDelivery(t *testing.T) {
	endpoints := []string{
		gatewayBaseURL + "/docs/",
		gatewayBaseURL + "/api/docs/",
		gatewayBaseURL + "/docs",
		gatewayBaseURL + "/api/docs",
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("stopped after 5 redirects")
			}
			return nil
		},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := client.Get(endpoint)
			if err != nil {
				t.Fatalf("Failed to GET %s: %v", endpoint, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected HTTP 200 for %s, got %d", endpoint, resp.StatusCode)
			}

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, "text/html") {
				t.Errorf("Expected Content-Type to contain text/html for %s, got %s", endpoint, contentType)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read body from %s: %v", endpoint, err)
			}

			bodyStr := string(body)
			if !strings.Contains(bodyStr, "swagger-ui") {
				t.Errorf("Expected response body to contain 'swagger-ui' for %s", endpoint)
			}
			if !strings.Contains(bodyStr, "openapi.yaml") {
				t.Errorf("Expected response body to contain 'openapi.yaml' for %s", endpoint)
			}
		})
	}
}

func TestOpenAPISpecDelivery(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Test openapi.yaml
	t.Run("openapi.yaml", func(t *testing.T) {
		resp, err := client.Get(gatewayBaseURL + "/docs/openapi.yaml")
		if err != nil {
			t.Fatalf("Failed to GET openapi.yaml: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected HTTP 200 for openapi.yaml, got %d", resp.StatusCode)
		}

		cType := resp.Header.Get("Content-Type")
		if !strings.Contains(cType, "yaml") && !strings.Contains(cType, "text/plain") {
			t.Errorf("Expected YAML content type for openapi.yaml, got %s", cType)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read openapi.yaml body: %v", err)
		}

		var doc map[string]interface{}
		if err := yaml.Unmarshal(body, &doc); err != nil {
			t.Fatalf("Delivered openapi.yaml is not valid YAML: %v", err)
		}

		ver, ok := doc["openapi"].(string)
		if !ok || !strings.HasPrefix(ver, "3.0") {
			t.Errorf("Expected openapi version 3.0.x, got %v", doc["openapi"])
		}
	})

	// 2. Test openapi.json
	t.Run("openapi.json", func(t *testing.T) {
		resp, err := client.Get(gatewayBaseURL + "/docs/openapi.json")
		if err != nil {
			t.Fatalf("Failed to GET openapi.json: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected HTTP 200 for openapi.json, got %d", resp.StatusCode)
		}

		cType := resp.Header.Get("Content-Type")
		if !strings.Contains(cType, "application/json") {
			t.Errorf("Expected application/json content type for openapi.json, got %s", cType)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read openapi.json body: %v", err)
		}

		var doc map[string]interface{}
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("Delivered openapi.json is not valid JSON: %v", err)
		}

		ver, ok := doc["openapi"].(string)
		if !ok || !strings.HasPrefix(ver, "3.0") {
			t.Errorf("Expected openapi version 3.0.x, got %v", doc["openapi"])
		}
	})
}

func TestOpenAPISchemaIntegrityAndCoverage(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(gatewayBaseURL + "/docs/openapi.json")
	if err != nil {
		t.Fatalf("Failed to fetch openapi.json: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected HTTP 200, got %d", resp.StatusCode)
	}

	var spec struct {
		OpenAPI string `json:"openapi"`
		Info    struct {
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"info"`
		Paths      map[string]map[string]interface{} `json:"paths"`
		Components struct {
			Schemas         map[string]interface{} `json:"schemas"`
			SecuritySchemes map[string]interface{} `json:"securitySchemes"`
		} `json:"components"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		t.Fatalf("Failed to parse openapi.json: %v", err)
	}

	// 1. Verify Basic Info
	if !strings.HasPrefix(spec.OpenAPI, "3.0") {
		t.Errorf("Expected OpenAPI 3.0.x, got %s", spec.OpenAPI)
	}
	if !strings.Contains(spec.Info.Title, "Distributed Ticket Booking") {
		t.Errorf("Unexpected API Title: %s", spec.Info.Title)
	}

	// 2. Verify Path Coverage across 6 subsystems
	requiredPaths := []string{
		// Auth
		"/api/auth/register",
		"/api/auth/login",
		"/api/auth/verify",
		"/api/auth/me",
		"/api/auth/users",
		// Catalog / Events
		"/api/events",
		"/api/events/{id}",
		// Inventory
		"/api/inventory/{eventId}",
		"/api/inventory/{eventId}/seats",
		"/api/inventory/reserve",
		"/api/inventory/commit",
		"/api/inventory/release",
		"/api/inventory/cancel",
		"/api/inventory/reservations/{id}",
		"/api/inventory/janitor/sweep",
		// Bookings
		"/api/bookings",
		"/api/bookings/{id}",
		"/api/bookings/{id}/cancel",
		// Waitlist
		"/api/waitlist",
		"/api/waitlist/{id}",
		"/api/waitlist/{id}/claim",
		"/api/waitlist/{id}/cancel",
		// DLQ
		"/api/dlq/stats",
		"/api/dlq/messages",
		"/api/dlq/retry",
		"/api/dlq/simulate",
		// Gateway
		"/health",
	}

	for _, p := range requiredPaths {
		if _, exists := spec.Paths[p]; !exists {
			t.Errorf("Missing required OpenAPI path: %s", p)
		}
	}

	// 3. Verify Component Schemas Coverage
	requiredSchemas := []string{
		"User", "RegisterRequest", "LoginRequest", "AuthResponse", "UserClaims",
		"Event", "EventTier", "EventListResponse",
		"SeatItem", "InventoryResponse", "ReserveSeatsRequest", "ReservationResponse",
		"Booking", "CreateBookingRequest", "BookingTicket", "CancelBookingResponse",
		"WaitlistEntry", "JoinWaitlistRequest", "WaitlistStatusResponse", "ClaimWaitlistResponse",
		"DLQMessage", "DLQStatsResponse", "DLQRetryResponse",
		"ErrorResponse",
	}

	for _, s := range requiredSchemas {
		if _, exists := spec.Components.Schemas[s]; !exists {
			t.Errorf("Missing required OpenAPI component schema: %s", s)
		}
	}
}

func TestOpenAPISecurityAndIdempotencyHeaders(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(gatewayBaseURL + "/docs/openapi.json")
	if err != nil {
		t.Fatalf("Failed to fetch openapi.json: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected HTTP 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read body: %v", err)
	}

	var spec map[string]interface{}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// 1. Verify BearerAuth security scheme
	components, ok := spec["components"].(map[string]interface{})
	if !ok {
		t.Fatalf("Missing components object")
	}
	secSchemes, ok := components["securitySchemes"].(map[string]interface{})
	if !ok {
		t.Fatalf("Missing securitySchemes object in components")
	}
	bearerAuth, ok := secSchemes["BearerAuth"].(map[string]interface{})
	if !ok {
		t.Fatalf("Missing BearerAuth in securitySchemes")
	}
	if bearerAuth["type"] != "http" || bearerAuth["scheme"] != "bearer" {
		t.Errorf("BearerAuth misconfigured: expected type http and scheme bearer, got %v", bearerAuth)
	}

	// 2. Verify Idempotency-Key & X-Correlation-ID headers on mutating endpoints
	paths, ok := spec["paths"].(map[string]interface{})
	if !ok {
		t.Fatalf("Missing paths object")
	}

	mutatingEndpoints := []struct {
		path   string
		method string
	}{
		{"/api/bookings", "post"},
		{"/api/inventory/reserve", "post"},
		{"/api/waitlist", "post"},
	}

	for _, ep := range mutatingEndpoints {
		pathItem, exists := paths[ep.path].(map[string]interface{})
		if !exists {
			t.Errorf("Path %s not found in spec", ep.path)
			continue
		}
		op, exists := pathItem[ep.method].(map[string]interface{})
		if !exists {
			t.Errorf("Method %s not found on path %s", ep.method, ep.path)
			continue
		}

		params, ok := op["parameters"].([]interface{})
		if !ok || len(params) == 0 {
			t.Errorf("Expected parameters defined for %s %s", strings.ToUpper(ep.method), ep.path)
			continue
		}

		hasIdempotencyKey := false
		hasCorrelationID := false

		for _, p := range params {
			paramMap, isMap := p.(map[string]interface{})
			if !isMap {
				continue
			}
			name, _ := paramMap["name"].(string)
			in, _ := paramMap["in"].(string)

			if strings.EqualFold(name, "Idempotency-Key") && in == "header" {
				hasIdempotencyKey = true
			}
			if strings.EqualFold(name, "X-Correlation-ID") && in == "header" {
				hasCorrelationID = true
			}
		}

		if !hasIdempotencyKey {
			t.Errorf("Missing Idempotency-Key header parameter on %s %s", strings.ToUpper(ep.method), ep.path)
		}
		if !hasCorrelationID {
			t.Errorf("Missing X-Correlation-ID header parameter on %s %s", strings.ToUpper(ep.method), ep.path)
		}
	}
}
