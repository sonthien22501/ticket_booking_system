package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (s *Server) startSagaWorker(rmq *RabbitMQ) {
	// 1. Consume Reserve commands
	rmq.ConsumeWithRetry("inventory_reserve_queue", "inventory.reserve", 3, func(body []byte, headers amqp.Table) error {
		log.Printf("Received inventory.reserve command")

		var msg map[string]interface{}
		if err := json.Unmarshal(body, &msg); err != nil {
			log.Printf("Unprocessable reserve message: invalid JSON: %v", err)
			return &UnprocessableError{Err: err, Reason: "MALFORMED_JSON"}
		}

		sagaID, _ := msg["saga_id"].(string)
		if sagaID == "" {
			log.Printf("Unprocessable reserve message: missing saga_id")
			return &UnprocessableError{Err: fmt.Errorf("missing saga_id"), Reason: "MISSING_SAGA_ID"}
		}

		// Support test simulation of worker errors for retry & DLQ verification
		if simErr, ok := headers["x-simulate-error"].(string); ok && simErr != "" {
			log.Printf("Simulating inventory worker failure: %s", simErr)
			return fmt.Errorf("simulated error: %s", simErr)
		}

		// Create a mock HTTP request to internal handler
		req := httptest.NewRequest(http.MethodPost, "/inventory/reserve", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		// Call the handler
		s.handleReserve(rr, req)

		var resp map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &resp)

		if rr.Code == http.StatusOK || rr.Code == http.StatusCreated {
			// Success
			resp["saga_id"] = sagaID
			out, _ := json.Marshal(resp)
			rmq.Publish("inventory.reserved", out)
			log.Printf("Published inventory.reserved for saga %s", sagaID)
		} else {
			// Failed (e.g. seat taken)
			resp["saga_id"] = sagaID
			out, _ := json.Marshal(resp)
			rmq.Publish("inventory.reserve_failed", out)
			log.Printf("Published inventory.reserve_failed for saga %s: %s", sagaID, rr.Body.String())
		}
		return nil
	})

	// 2. Consume Commit commands
	rmq.ConsumeWithRetry("inventory_commit_queue", "inventory.commit", 3, func(body []byte, headers amqp.Table) error {
		log.Printf("Received inventory.commit command")

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
		rr := httptest.NewRecorder()

		s.handleCommit(rr, req)

		if rr.Code == http.StatusOK {
			msg["status"] = "COMMITTED"
			out, _ := json.Marshal(msg)
			rmq.Publish("inventory.committed", out)
			log.Printf("Published inventory.committed for saga %s", sagaID)
		} else {
			msg["error"] = rr.Body.String()
			out, _ := json.Marshal(msg)
			rmq.Publish("inventory.commit_failed", out)
			log.Printf("Published inventory.commit_failed for saga %s", sagaID)
		}
		return nil
	})

	// 3. Consume Release commands
	rmq.ConsumeWithRetry("inventory_release_queue", "inventory.release", 3, func(body []byte, headers amqp.Table) error {
		log.Printf("Received inventory.release command")

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
		rr := httptest.NewRecorder()

		s.handleRelease(rr, req)

		msg["status"] = "RELEASED"
		out, _ := json.Marshal(msg)
		rmq.Publish("inventory.released", out)
		log.Printf("Published inventory.released for saga %s", sagaID)
		return nil
	})

	// 4. Consume Cancel commands
	rmq.ConsumeWithRetry("inventory_cancel_queue", "inventory.cancel", 3, func(body []byte, headers amqp.Table) error {
		log.Printf("Received inventory.cancel command")

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
		rr := httptest.NewRecorder()

		s.handleCancel(rr, req)

		if rr.Code == http.StatusOK {
			msg["status"] = "CANCELLED"
			out, _ := json.Marshal(msg)
			rmq.Publish("inventory.cancelled", out)
			log.Printf("Published inventory.cancelled for saga %s", sagaID)
		} else {
			msg["error"] = rr.Body.String()
			out, _ := json.Marshal(msg)
			rmq.Publish("inventory.cancel_failed", out)
			log.Printf("Published inventory.cancel_failed for saga %s: %s", sagaID, rr.Body.String())
		}
		return nil
	})
}
