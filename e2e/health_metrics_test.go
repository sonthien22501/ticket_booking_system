package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type ComponentHealth struct {
	Status  string                 `json:"status"`
	Details map[string]interface{} `json:"details,omitempty"`
}

type HealthResponse struct {
	Status        string                     `json:"status"`
	Service       string                     `json:"service"`
	Timestamp     string                     `json:"timestamp"`
	UptimeSeconds float64                    `json:"uptime_seconds"`
	Version       string                     `json:"version,omitempty"`
	Components    map[string]ComponentHealth `json:"components"`
}

type DBPoolMetrics struct {
	MaxOpenConnections int   `json:"max_open_connections"`
	OpenConnections    int   `json:"open_connections"`
	InUse              int   `json:"in_use"`
	Idle               int   `json:"idle"`
	WaitCount          int64 `json:"wait_count"`
	WaitDurationMs     int64 `json:"wait_duration_ms"`
}

type MemoryMetrics struct {
	AllocBytes      uint64  `json:"alloc_bytes"`
	TotalAllocBytes uint64  `json:"total_alloc_bytes"`
	SysBytes        uint64  `json:"sys_bytes"`
	HeapAllocBytes  uint64  `json:"heap_alloc_bytes"`
	HeapInuseBytes  uint64  `json:"heap_inuse_bytes"`
	NumGoroutines   int     `json:"num_goroutines"`
	NumGC           uint32  `json:"num_gc"`
	GCCPUFraction   float64 `json:"gc_cpu_fraction"`
}

type ServiceMetricsResponse struct {
	Service        string                 `json:"service"`
	Timestamp      string                 `json:"timestamp"`
	UptimeSeconds  float64                `json:"uptime_seconds"`
	Memory         MemoryMetrics          `json:"memory"`
	ConnectionPool DBPoolMetrics          `json:"connection_pool"`
	RedisPool      map[string]interface{} `json:"redis_pool,omitempty"`
	RabbitMQ       map[string]interface{} `json:"rabbitmq,omitempty"`
	HTTPMetrics    map[string]interface{} `json:"http_metrics,omitempty"`
}

func TestMicroserviceHealthEndpoints(t *testing.T) {
	services := []struct {
		name         string
		url          string
		expectedDeps []string
	}{
		{
			name:         "catalog-service",
			url:          "http://localhost:8081/health",
			expectedDeps: []string{"database", "redis"},
		},
		{
			name:         "inventory-service",
			url:          "http://localhost:8082/health",
			expectedDeps: []string{"database", "redis", "rabbitmq"},
		},
		{
			name:         "booking-service",
			url:          "http://localhost:8083/health",
			expectedDeps: []string{"database", "redis", "rabbitmq"},
		},
		{
			name:         "auth-service",
			url:          "http://localhost:8084/health",
			expectedDeps: []string{"database"},
		},
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, svc := range services {
		t.Run(svc.name, func(t *testing.T) {
			resp, err := client.Get(svc.url)
			if err != nil {
				t.Fatalf("Failed to call %s: %v", svc.url, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("Expected HTTP 200 for %s, got %d: %s", svc.url, resp.StatusCode, string(body))
			}

			var health HealthResponse
			if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
				t.Fatalf("Failed to decode health JSON from %s: %v", svc.url, err)
			}

			if health.Status != "UP" {
				t.Errorf("Expected status UP, got %q", health.Status)
			}
			if health.Service != svc.name && !(svc.name == "inventory-service" && health.Service == "seat-inventory-service") {
				t.Errorf("Expected service %q, got %q", svc.name, health.Service)
			}
			if health.Timestamp == "" {
				t.Errorf("Expected non-empty timestamp")
			}
			if health.UptimeSeconds <= 0 {
				t.Errorf("Expected positive uptime_seconds, got %f", health.UptimeSeconds)
			}
			if len(health.Components) == 0 {
				t.Fatalf("Expected components map to be populated for %s", svc.name)
			}

			for _, dep := range svc.expectedDeps {
				comp, ok := health.Components[dep]
				if !ok {
					t.Errorf("Missing expected dependency component %q in health check for %s", dep, svc.name)
					continue
				}
				if comp.Status != "UP" {
					t.Errorf("Expected component %q status UP, got %q for %s", dep, comp.Status, svc.name)
				}
			}
		})
	}
}

func TestMicroserviceMetricsEndpoints(t *testing.T) {
	services := []struct {
		name     string
		url      string
		hasRedis bool
		hasRabbit bool
	}{
		{name: "catalog-service", url: "http://localhost:8081/metrics", hasRedis: true, hasRabbit: false},
		{name: "inventory-service", url: "http://localhost:8082/metrics", hasRedis: true, hasRabbit: true},
		{name: "booking-service", url: "http://localhost:8083/metrics", hasRedis: true, hasRabbit: true},
		{name: "auth-service", url: "http://localhost:8084/metrics", hasRedis: false, hasRabbit: false},
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, svc := range services {
		t.Run(svc.name, func(t *testing.T) {
			resp, err := client.Get(svc.url)
			if err != nil {
				t.Fatalf("Failed to call %s: %v", svc.url, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("Expected HTTP 200 for %s, got %d: %s", svc.url, resp.StatusCode, string(body))
			}

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				t.Errorf("Expected application/json Content-Type, got %s", contentType)
			}

			var metrics ServiceMetricsResponse
			if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
				t.Fatalf("Failed to parse JSON metrics from %s: %v", svc.url, err)
			}

			if metrics.Service != svc.name && !(svc.name == "inventory-service" && metrics.Service == "seat-inventory-service") {
				t.Errorf("Expected service name %s, got %s", svc.name, metrics.Service)
			}
			if metrics.UptimeSeconds <= 0 {
				t.Errorf("Expected positive uptime_seconds, got %f", metrics.UptimeSeconds)
			}

			// Validate Memory metrics
			if metrics.Memory.AllocBytes == 0 {
				t.Errorf("Expected positive AllocBytes")
			}
			if metrics.Memory.NumGoroutines == 0 {
				t.Errorf("Expected positive NumGoroutines")
			}

			// Validate Connection Pool metrics (R23 core requirement)
			if metrics.ConnectionPool.OpenConnections < 0 {
				t.Errorf("Invalid OpenConnections: %d", metrics.ConnectionPool.OpenConnections)
			}

			if svc.hasRedis {
				if len(metrics.RedisPool) == 0 {
					t.Errorf("Expected non-empty RedisPool metrics for %s", svc.name)
				}
			}
			if svc.hasRabbit {
				if len(metrics.RabbitMQ) == 0 {
					t.Errorf("Expected non-empty RabbitMQ metrics for %s", svc.name)
				}
			}
		})
	}
}

func TestPrometheusMetricsFormat(t *testing.T) {
	endpoints := []string{
		"http://localhost:8081/metrics",
		"http://localhost:8082/metrics",
		"http://localhost:8083/metrics",
		"http://localhost:8084/metrics",
	}

	client := &http.Client{Timeout: 5 * time.Second}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, ep, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Accept", "text/plain")

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to execute request to %s: %v", ep, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected HTTP 200 from %s, got %d", ep, resp.StatusCode)
			}

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, "text/plain") {
				t.Errorf("Expected text/plain content type, got %s", contentType)
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("Failed to read body: %v", err)
			}
			body := string(bodyBytes)

			expectedSubstrings := []string{
				"# HELP go_mem_alloc_bytes",
				"# TYPE go_mem_alloc_bytes gauge",
				"# HELP go_goroutines",
				"# TYPE go_goroutines gauge",
				"# HELP db_pool_open_connections",
				"# TYPE db_pool_open_connections gauge",
				"# HELP db_pool_in_use",
				"# TYPE db_pool_idle",
			}

			for _, sub := range expectedSubstrings {
				if !strings.Contains(body, sub) {
					t.Errorf("Prometheus output from %s missing expected line %q", ep, sub)
				}
			}
		})
	}
}

func TestGatewayHealthAndMetricsRouting(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}

	routes := []struct {
		url          string
		expectedType string
	}{
		{url: "http://localhost:8080/health", expectedType: "application/json"},
		{url: "http://localhost:8080/metrics", expectedType: "application/json"},
		{url: "http://localhost:8080/api/catalog/health", expectedType: "application/json"},
		{url: "http://localhost:8080/api/catalog/metrics", expectedType: "application/json"},
		{url: "http://localhost:8080/api/inventory/health", expectedType: "application/json"},
		{url: "http://localhost:8080/api/inventory/metrics", expectedType: "application/json"},
		{url: "http://localhost:8080/api/bookings/health", expectedType: "application/json"},
		{url: "http://localhost:8080/api/bookings/metrics", expectedType: "application/json"},
		{url: "http://localhost:8080/api/auth/health", expectedType: "application/json"},
		{url: "http://localhost:8080/api/auth/metrics", expectedType: "application/json"},
	}

	for _, r := range routes {
		t.Run(r.url, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, r.url, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("X-Correlation-ID", "test-corr-actuator-1234")

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to call %s: %v", r.url, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("Expected HTTP 200 for %s, got %d: %s", r.url, resp.StatusCode, string(body))
			}

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, r.expectedType) {
				t.Errorf("Expected %s, got %s for %s", r.expectedType, contentType, r.url)
			}

			body, _ := io.ReadAll(resp.Body)
			if len(body) == 0 {
				t.Errorf("Expected non-empty response body for %s", r.url)
			}
		})
	}
}
