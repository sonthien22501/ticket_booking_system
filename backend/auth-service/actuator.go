package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync/atomic"
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
	HTTPMetrics    map[string]interface{} `json:"http_metrics,omitempty"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	components := make(map[string]ComponentHealth)
	overallHealthy := true

	// Check Database
	if s.db != nil {
		start := time.Now()
		if err := s.db.PingContext(ctx); err != nil {
			overallHealthy = false
			components["database"] = ComponentHealth{
				Status: "DOWN",
				Details: map[string]interface{}{
					"error": err.Error(),
				},
			}
		} else {
			dbStats := s.db.Stats()
			components["database"] = ComponentHealth{
				Status: "UP",
				Details: map[string]interface{}{
					"ping_ms":          float64(time.Since(start).Microseconds()) / 1000.0,
					"open_connections": dbStats.OpenConnections,
					"in_use":           dbStats.InUse,
					"idle":             dbStats.Idle,
				},
			}
		}
	} else {
		components["database"] = ComponentHealth{
			Status: "UP",
			Details: map[string]interface{}{
				"status": "standalone/mock",
			},
		}
	}

	uptime := time.Since(s.startTime).Seconds()
	if uptime < 0.001 {
		uptime = 0.001
	}

	status := "UP"
	httpStatus := http.StatusOK
	if !overallHealthy {
		status = "DOWN"
		httpStatus = http.StatusServiceUnavailable
	}

	resp := HealthResponse{
		Status:        status,
		Service:       "auth-service",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		UptimeSeconds: uptime,
		Version:       "1.0.0",
		Components:    components,
	}

	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	atomic.AddInt64(&s.reqCount, 1)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	uptime := time.Since(s.startTime).Seconds()
	if uptime < 0.001 {
		uptime = 0.001
	}

	var dbPool DBPoolMetrics
	if s.db != nil {
		stats := s.db.Stats()
		dbPool = DBPoolMetrics{
			MaxOpenConnections: stats.MaxOpenConnections,
			OpenConnections:    stats.OpenConnections,
			InUse:              stats.InUse,
			Idle:               stats.Idle,
			WaitCount:          stats.WaitCount,
			WaitDurationMs:     stats.WaitDuration.Milliseconds(),
		}
	}

	format := r.URL.Query().Get("format")
	accept := r.Header.Get("Accept")

	if format == "prometheus" || strings.Contains(accept, "text/plain") {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		var sb strings.Builder
		sb.WriteString("# HELP go_mem_alloc_bytes Number of bytes allocated and still in use.\n")
		sb.WriteString("# TYPE go_mem_alloc_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("go_mem_alloc_bytes{service=\"auth-service\"} %d\n", mem.Alloc))

		sb.WriteString("# HELP go_mem_total_alloc_bytes Total number of bytes allocated, even if freed.\n")
		sb.WriteString("# TYPE go_mem_total_alloc_bytes counter\n")
		sb.WriteString(fmt.Sprintf("go_mem_total_alloc_bytes{service=\"auth-service\"} %d\n", mem.TotalAlloc))

		sb.WriteString("# HELP go_mem_sys_bytes Number of bytes obtained from system.\n")
		sb.WriteString("# TYPE go_mem_sys_bytes gauge\n")
		sb.WriteString(fmt.Sprintf("go_mem_sys_bytes{service=\"auth-service\"} %d\n", mem.Sys))

		sb.WriteString("# HELP go_goroutines Number of goroutines that currently exist.\n")
		sb.WriteString("# TYPE go_goroutines gauge\n")
		sb.WriteString(fmt.Sprintf("go_goroutines{service=\"auth-service\"} %d\n", runtime.NumGoroutine()))

		sb.WriteString("# HELP go_num_gc Number of completed GC cycles.\n")
		sb.WriteString("# TYPE go_num_gc counter\n")
		sb.WriteString(fmt.Sprintf("go_num_gc{service=\"auth-service\"} %d\n", mem.NumGC))

		sb.WriteString("# HELP db_pool_max_open The maximum number of open connections to the database.\n")
		sb.WriteString("# TYPE db_pool_max_open gauge\n")
		sb.WriteString(fmt.Sprintf("db_pool_max_open{service=\"auth-service\"} %d\n", dbPool.MaxOpenConnections))

		sb.WriteString("# HELP db_pool_open_connections The number of established connections both in use and idle.\n")
		sb.WriteString("# TYPE db_pool_open_connections gauge\n")
		sb.WriteString(fmt.Sprintf("db_pool_open_connections{service=\"auth-service\"} %d\n", dbPool.OpenConnections))

		sb.WriteString("# HELP db_pool_in_use The number of connections currently in use.\n")
		sb.WriteString("# TYPE db_pool_in_use gauge\n")
		sb.WriteString(fmt.Sprintf("db_pool_in_use{service=\"auth-service\"} %d\n", dbPool.InUse))

		sb.WriteString("# HELP db_pool_idle The number of idle connections.\n")
		sb.WriteString("# TYPE db_pool_idle gauge\n")
		sb.WriteString(fmt.Sprintf("db_pool_idle{service=\"auth-service\"} %d\n", dbPool.Idle))

		sb.WriteString("# HELP db_pool_wait_count Total number of connections waited for.\n")
		sb.WriteString("# TYPE db_pool_wait_count counter\n")
		sb.WriteString(fmt.Sprintf("db_pool_wait_count{service=\"auth-service\"} %d\n", dbPool.WaitCount))

		sb.WriteString("# HELP http_requests_total Total number of HTTP requests processed.\n")
		sb.WriteString("# TYPE http_requests_total counter\n")
		sb.WriteString(fmt.Sprintf("http_requests_total{service=\"auth-service\"} %d\n", atomic.LoadInt64(&s.reqCount)))

		sb.WriteString("# HELP process_uptime_seconds Total uptime of the service process.\n")
		sb.WriteString("# TYPE process_uptime_seconds gauge\n")
		sb.WriteString(fmt.Sprintf("process_uptime_seconds{service=\"auth-service\"} %.2f\n", uptime))

		w.Write([]byte(sb.String()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := ServiceMetricsResponse{
		Service:       "auth-service",
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		UptimeSeconds: uptime,
		Memory: MemoryMetrics{
			AllocBytes:      mem.Alloc,
			TotalAllocBytes: mem.TotalAlloc,
			SysBytes:        mem.Sys,
			HeapAllocBytes:  mem.HeapAlloc,
			HeapInuseBytes:  mem.HeapInuse,
			NumGoroutines:   runtime.NumGoroutine(),
			NumGC:           mem.NumGC,
			GCCPUFraction:   mem.GCCPUFraction,
		},
		ConnectionPool: dbPool,
		HTTPMetrics: map[string]interface{}{
			"requests_total": atomic.LoadInt64(&s.reqCount),
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}
