package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type contextKey string

const (
	CorrelationIDContextKey contextKey = "correlation_id"
	CorrelationIDHeader                = "X-Correlation-ID"
	RequestIDHeader                    = "X-Request-ID"
)

// LogEntry defines the standardized JSON schema for distributed logs across all microservices
type LogEntry struct {
	Timestamp     string                 `json:"timestamp"`
	Level         string                 `json:"level"`
	Service       string                 `json:"service"`
	CorrelationID string                 `json:"correlationId,omitempty"`
	Message       string                 `json:"message"`
	Method        string                 `json:"method,omitempty"`
	Path          string                 `json:"path,omitempty"`
	StatusCode    int                    `json:"statusCode,omitempty"`
	DurationMs    float64                `json:"durationMs,omitempty"`
	RemoteAddr    string                 `json:"remoteAddr,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Fields        map[string]interface{} `json:"fields,omitempty"`
}

var logMu sync.Mutex

// ContextWithCorrelationID returns a new context containing the correlation ID
func ContextWithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, CorrelationIDContextKey, id)
}

// CorrelationIDFromContext extracts the correlation ID from context
func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(CorrelationIDContextKey).(string); ok {
		return id
	}
	return ""
}

// CorrelationIDFromRequest extracts correlation ID from headers or context
func CorrelationIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	if id := r.Header.Get(CorrelationIDHeader); id != "" {
		return id
	}
	if id := r.Header.Get(RequestIDHeader); id != "" {
		return id
	}
	return CorrelationIDFromContext(r.Context())
}

// GenerateCorrelationID generates a unique 16-character pseudo-random correlation ID
func GenerateCorrelationID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("trc-%d-%x", time.Now().UnixNano(), b)
}

// StructuredLog writes a JSON-formatted log entry to standard output
func StructuredLog(level string, service string, correlationID string, message string, extra map[string]interface{}) {
	entry := LogEntry{
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Level:         level,
		Service:       service,
		CorrelationID: correlationID,
		Message:       message,
	}

	if extra != nil {
		// Copy map to avoid mutating caller data
		copied := make(map[string]interface{}, len(extra))
		for k, v := range extra {
			copied[k] = v
		}

		if method, ok := copied["method"].(string); ok {
			entry.Method = method
			delete(copied, "method")
		}
		if path, ok := copied["path"].(string); ok {
			entry.Path = path
			delete(copied, "path")
		}
		if status, ok := copied["statusCode"].(int); ok {
			entry.StatusCode = status
			delete(copied, "statusCode")
		}
		if dur, ok := copied["durationMs"].(float64); ok {
			entry.DurationMs = dur
			delete(copied, "durationMs")
		}
		if remote, ok := copied["remoteAddr"].(string); ok {
			entry.RemoteAddr = remote
			delete(copied, "remoteAddr")
		}
		if errStr, ok := copied["error"].(string); ok {
			entry.Error = errStr
			delete(copied, "error")
		}
		if len(copied) > 0 {
			entry.Fields = copied
		}
	}

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("[%s] [%s] %s: %s", level, service, correlationID, message)
		return
	}

	logMu.Lock()
	defer logMu.Unlock()
	fmt.Println(string(data))
}

// LogInfo writes an informational structured log entry
func LogInfo(ctx context.Context, service string, message string, extra ...map[string]interface{}) {
	var f map[string]interface{}
	if len(extra) > 0 {
		f = extra[0]
	}
	StructuredLog("INFO", service, CorrelationIDFromContext(ctx), message, f)
}

// LogWarn writes a warning structured log entry
func LogWarn(ctx context.Context, service string, message string, extra ...map[string]interface{}) {
	var f map[string]interface{}
	if len(extra) > 0 {
		f = extra[0]
	}
	StructuredLog("WARN", service, CorrelationIDFromContext(ctx), message, f)
}

// LogError writes an error structured log entry
func LogError(ctx context.Context, service string, message string, err error, extra ...map[string]interface{}) {
	f := make(map[string]interface{})
	if len(extra) > 0 && extra[0] != nil {
		for k, v := range extra[0] {
			f[k] = v
		}
	}
	if err != nil {
		f["error"] = err.Error()
	}
	StructuredLog("ERROR", service, CorrelationIDFromContext(ctx), message, f)
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *statusResponseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// TracingMiddleware intercepts HTTP requests to extract/generate correlation IDs,
// attach them to the request context, set response headers, and output structured access logs.
func TracingMiddleware(serviceName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := r.Header.Get(CorrelationIDHeader)
		if corrID == "" {
			corrID = r.Header.Get(RequestIDHeader)
		}
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		// Propagate to response headers
		w.Header().Set(CorrelationIDHeader, corrID)
		w.Header().Set(RequestIDHeader, corrID)

		ctx := ContextWithCorrelationID(r.Context(), corrID)
		start := time.Now()

		rw := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r.WithContext(ctx))

		durationMs := float64(time.Since(start).Microseconds()) / 1000.0

		level := "INFO"
		if rw.statusCode >= 500 {
			level = "ERROR"
		} else if rw.statusCode >= 400 {
			level = "WARN"
		}

		StructuredLog(level, serviceName, corrID, fmt.Sprintf("%s %s -> %d (%.2fms)", r.Method, r.URL.Path, rw.statusCode, durationMs), map[string]interface{}{
			"method":     r.Method,
			"path":       r.URL.Path,
			"query":      r.URL.RawQuery,
			"statusCode": rw.statusCode,
			"durationMs": durationMs,
			"remoteAddr": r.RemoteAddr,
			"bytesSent":  rw.bytesWritten,
		})
	})
}
