package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
)

func (s *Server) startSagaWorker(rmq *RabbitMQ) {
	// Consume Reserve commands
	rmq.Consume("inventory_reserve_queue", "inventory.reserve", func(body []byte) {
		log.Printf("Received inventory.reserve command")
		
		var msg map[string]interface{}
		json.Unmarshal(body, &msg)
		sagaID, _ := msg["saga_id"].(string)

		// Create a mock HTTP request
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
			// Failed
			resp["saga_id"] = sagaID
			out, _ := json.Marshal(resp)
			rmq.Publish("inventory.reserve_failed", out)
			log.Printf("Published inventory.reserve_failed for saga %s: %s", sagaID, rr.Body.String())
		}
	})

	// Consume Commit commands
	rmq.Consume("inventory_commit_queue", "inventory.commit", func(body []byte) {
		log.Printf("Received inventory.commit command")
		
		var msg map[string]interface{}
		json.Unmarshal(body, &msg)
		sagaID, _ := msg["saga_id"].(string)

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
	})

	// Consume Release commands
	rmq.Consume("inventory_release_queue", "inventory.release", func(body []byte) {
		log.Printf("Received inventory.release command")
		
		var msg map[string]interface{}
		json.Unmarshal(body, &msg)
		sagaID, _ := msg["saga_id"].(string)

		req := httptest.NewRequest(http.MethodPost, "/inventory/release", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		s.handleRelease(rr, req)

		msg["status"] = "RELEASED"
		out, _ := json.Marshal(msg)
		rmq.Publish("inventory.released", out)
		log.Printf("Published inventory.released for saga %s", sagaID)
	})
}
