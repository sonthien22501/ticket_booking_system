package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (s *Server) startSagaWorker(rmq *RabbitMQ) {
	// 1. Consume Reserve commands
	rmq.ConsumeWithRetry("inventory_reserve_queue", "inventory.reserve", 3, func(body []byte, headers amqp.Table) error {
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		StructuredLog("INFO", "inventory-service", corrID, "Received inventory.reserve command", nil)

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			StructuredLog("WARN", "inventory-service", corrID, fmt.Sprintf("Unprocessable reserve message: invalid JSON: %v", err), nil)
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}

		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			StructuredLog("WARN", "inventory-service", corrID, "Unprocessable reserve message: missing saga_id", nil)
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}

		// Support test simulation of worker errors for retry & DLQ verification
		if simErr, ok := headers["x-simulate-error"].(string); ok && simErr != "" {
			StructuredLog("WARN", "inventory-service", corrID, fmt.Sprintf("Simulating inventory worker failure: %s", simErr), nil)
			return fmt.Errorf("simulated error: %s", simErr)
		}

		// Create a mock HTTP request to internal handler with correlation ID preserved
		req := httptest.NewRequest(http.MethodPost, "/inventory/reserve", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Correlation-ID", corrID)
		req.Header.Set("X-Request-ID", corrID)
		rr := httptest.NewRecorder()

		// Call the handler
		s.handleReserve(rr, req)

		var resp map[string]interface{}
		_ = json.Unmarshal(rr.Body.Bytes(), &resp)
		if resp == nil {
			resp = make(map[string]interface{})
		}

		outHeaders := amqp.Table{"x-correlation-id": corrID}
		if rr.Code == http.StatusOK || rr.Code == http.StatusCreated {
			// Success
			resp["saga_id"] = sagaID
			out, _ := json.Marshal(resp)
			_ = rmq.PublishWithHeaders(SagaExchange, "inventory.reserved", out, outHeaders)
			StructuredLog("INFO", "inventory-service", corrID, fmt.Sprintf("Published inventory.reserved for saga %s", sagaID), nil)
		} else {
			// Failed (e.g. seat taken)
			resp["saga_id"] = sagaID
			out, _ := json.Marshal(resp)
			_ = rmq.PublishWithHeaders(SagaExchange, "inventory.reserve_failed", out, outHeaders)
			StructuredLog("INFO", "inventory-service", corrID, fmt.Sprintf("Published inventory.reserve_failed for saga %s: %s", sagaID, rr.Body.String()), nil)
		}
		return nil
	})

	// 2. Consume Commit commands
	rmq.ConsumeWithRetry("inventory_commit_queue", "inventory.commit", 3, func(body []byte, headers amqp.Table) error {
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		StructuredLog("INFO", "inventory-service", corrID, "Received inventory.commit command", nil)

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}

		if simErr, ok := headers["x-simulate-error"].(string); ok && simErr != "" {
			return fmt.Errorf("simulated error: %s", simErr)
		}

		req := httptest.NewRequest(http.MethodPost, "/inventory/commit", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Correlation-ID", corrID)
		req.Header.Set("X-Request-ID", corrID)
		rr := httptest.NewRecorder()

		s.handleCommit(rr, req)

		outHeaders := amqp.Table{"x-correlation-id": corrID}
		if rr.Code == http.StatusOK {
			msg["status"] = "COMMITTED"
			out, _ := json.Marshal(msg)
			_ = rmq.PublishWithHeaders(SagaExchange, "inventory.committed", out, outHeaders)
			StructuredLog("INFO", "inventory-service", corrID, fmt.Sprintf("Published inventory.committed for saga %s", sagaID), nil)
		} else {
			msg["error"] = rr.Body.String()
			out, _ := json.Marshal(msg)
			_ = rmq.PublishWithHeaders(SagaExchange, "inventory.commit_failed", out, outHeaders)
			StructuredLog("INFO", "inventory-service", corrID, fmt.Sprintf("Published inventory.commit_failed for saga %s", sagaID), nil)
		}
		return nil
	})

	// 3. Consume Release commands
	rmq.ConsumeWithRetry("inventory_release_queue", "inventory.release", 3, func(body []byte, headers amqp.Table) error {
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		StructuredLog("INFO", "inventory-service", corrID, "Received inventory.release command", nil)

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}

		if simErr, ok := headers["x-simulate-error"].(string); ok && simErr != "" {
			return fmt.Errorf("simulated error: %s", simErr)
		}

		req := httptest.NewRequest(http.MethodPost, "/inventory/release", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Correlation-ID", corrID)
		req.Header.Set("X-Request-ID", corrID)
		rr := httptest.NewRecorder()

		s.handleRelease(rr, req)

		msg["status"] = "RELEASED"
		out, _ := json.Marshal(msg)
		outHeaders := amqp.Table{"x-correlation-id": corrID}
		_ = rmq.PublishWithHeaders(SagaExchange, "inventory.released", out, outHeaders)
		StructuredLog("INFO", "inventory-service", corrID, fmt.Sprintf("Published inventory.released for saga %s", sagaID), nil)
		return nil
	})

	// 4. Consume Cancel commands
	rmq.ConsumeWithRetry("inventory_cancel_queue", "inventory.cancel", 3, func(body []byte, headers amqp.Table) error {
		corrID, _ := headers["x-correlation-id"].(string)
		if corrID == "" {
			corrID = GenerateCorrelationID()
		}

		StructuredLog("INFO", "inventory-service", corrID, "Received inventory.cancel command", nil)

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}
		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}

		if simErr, ok := headers["x-simulate-error"].(string); ok && simErr != "" {
			return fmt.Errorf("simulated error: %s", simErr)
		}

		req := httptest.NewRequest(http.MethodPost, "/inventory/cancel", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Correlation-ID", corrID)
		req.Header.Set("X-Request-ID", corrID)
		rr := httptest.NewRecorder()

		s.handleCancel(rr, req)

		outHeaders := amqp.Table{"x-correlation-id": corrID}
		if rr.Code == http.StatusOK {
			msg["status"] = "CANCELLED"
			out, _ := json.Marshal(msg)
			_ = rmq.PublishWithHeaders(SagaExchange, "inventory.cancelled", out, outHeaders)
			StructuredLog("INFO", "inventory-service", corrID, fmt.Sprintf("Published inventory.cancelled for saga %s", sagaID), nil)
		} else {
			msg["error"] = rr.Body.String()
			out, _ := json.Marshal(msg)
			_ = rmq.PublishWithHeaders(SagaExchange, "inventory.cancel_failed", out, outHeaders)
			StructuredLog("INFO", "inventory-service", corrID, fmt.Sprintf("Published inventory.cancel_failed for saga %s: %s", sagaID, rr.Body.String()), nil)
		}
		return nil
	})
}
