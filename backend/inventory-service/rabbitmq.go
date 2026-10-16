package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var ErrUnprocessable = errors.New("unprocessable message")

type UnprocessableError struct {
	Err    error
	Reason string
}

func (e *UnprocessableError) Error() string {
	return fmt.Sprintf("%s: %v", e.Reason, e.Err)
}

func (e *UnprocessableError) Unwrap() error {
	return e.Err
}

func IsUnprocessable(err error) bool {
	if err == nil {
		return false
	}
	var ue *UnprocessableError
	if errors.As(err, &ue) {
		return true
	}
	return errors.Is(err, ErrUnprocessable)
}

type DLQMessage struct {
	MessageID          string                 `json:"messageId"`
	OriginalExchange   string                 `json:"originalExchange"`
	OriginalRoutingKey string                 `json:"originalRoutingKey"`
	DeathReason        string                 `json:"deathReason"`
	ErrorMessage       string                 `json:"errorMessage"`
	RetryCount         int                    `json:"retryCount"`
	Payload            string                 `json:"payload"`
	PayloadJSON        map[string]interface{} `json:"payloadJson,omitempty"`
	Timestamp          time.Time              `json:"timestamp"`
}

type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

type HandlerWithHeaders func(body []byte, headers amqp.Table) error

const (
	SagaExchange    = "saga_exchange"
	SagaDLX         = "saga_dead_letter_exchange"
	SagaDLQ         = "saga_dead_letter_queue"
	DLQRoutingKey   = "saga.dead_letter"
	DefaultMaxRetry = 3
)

func InitRabbitMQ(url string) (*RabbitMQ, error) {
	var conn *amqp.Connection
	var err error

	for i := 1; i <= 30; i++ {
		conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		log.Printf("Failed to connect to RabbitMQ (attempt %d/30): %v", i, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("could not connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("could not open channel: %w", err)
	}

	// 1. Declare primary Saga Topic Exchange
	err = ch.ExchangeDeclare(
		SagaExchange, // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("could not declare saga_exchange: %w", err)
	}

	// 2. Declare Dead Letter Exchange (DLX)
	err = ch.ExchangeDeclare(
		SagaDLX, // name
		"topic", // type
		true,    // durable
		false,   // auto-deleted
		false,   // internal
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("could not declare dead letter exchange: %w", err)
	}

	// 3. Declare Dead Letter Queue (DLQ)
	dlq, err := ch.QueueDeclare(
		SagaDLQ,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("could not declare dead letter queue: %w", err)
	}

	// 4. Bind DLQ to DLX with wildcard routing key "#" (catches all dead-lettered events)
	err = ch.QueueBind(
		dlq.Name,
		"#",
		SagaDLX,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("could not bind DLQ to DLX: %w", err)
	}

	log.Printf("RabbitMQ topology initialized with DLX (%s) and DLQ (%s)", SagaDLX, SagaDLQ)

	return &RabbitMQ{
		conn:    conn,
		channel: ch,
	}, nil
}

func (r *RabbitMQ) Close() {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		r.conn.Close()
	}
}

func (r *RabbitMQ) declareQueueSafe(queueName string, args amqp.Table) (amqp.Queue, error) {
	testCh, err := r.conn.Channel()
	if err != nil {
		return amqp.Queue{}, err
	}

	q, err := testCh.QueueDeclare(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		args,  // DLQ arguments
	)
	if err == nil {
		testCh.Close()
		return q, nil
	}

	testCh.Close()

	delCh, err := r.conn.Channel()
	if err != nil {
		return amqp.Queue{}, fmt.Errorf("failed to open recovery channel: %w", err)
	}
	defer delCh.Close()

	log.Printf("Upgrading queue %s with Dead Letter Exchange configuration...", queueName)
	_, _ = delCh.QueueDelete(queueName, false, false, false)

	return delCh.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		args,
	)
}

func (r *RabbitMQ) Publish(routingKey string, body []byte) error {
	return r.PublishWithHeaders(SagaExchange, routingKey, body, nil)
}

func (r *RabbitMQ) PublishWithHeaders(exchange, routingKey string, body []byte, headers amqp.Table) error {
	if headers == nil {
		headers = amqp.Table{}
	}

	corrID := ""
	if cid, ok := headers["x-correlation-id"].(string); ok && cid != "" {
		corrID = cid
	} else if tid, ok := headers["x-trace-id"].(string); ok && tid != "" {
		corrID = tid
	} else {
		corrID = GenerateCorrelationID()
	}
	headers["x-correlation-id"] = corrID

	StructuredLog("INFO", "inventory-service", corrID, fmt.Sprintf("RabbitMQ Published to %s [%s]", exchange, routingKey), map[string]interface{}{
		"exchange":   exchange,
		"routingKey": routingKey,
		"bodyBytes":  len(body),
	})

	return r.channel.Publish(
		exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			DeliveryMode:  amqp.Persistent,
			CorrelationId: corrID,
			Headers:       headers,
			Body:          body,
			Timestamp:     time.Now(),
		})
}

func (r *RabbitMQ) PublishToDLQ(routingKey string, body []byte, reason string, errMsg string, retryCount int) error {
	headers := amqp.Table{
		"x-death-reason":         reason,
		"x-error-message":        errMsg,
		"x-original-routing-key": routingKey,
		"x-original-exchange":    SagaExchange,
		"x-retry-count":          int32(retryCount),
		"x-failed-at":            time.Now().Format(time.RFC3339),
	}
	return r.PublishWithHeaders(SagaDLX, DLQRoutingKey, body, headers)
}

func (r *RabbitMQ) ConsumeWithRetry(queueName, routingKey string, maxRetries int, handler HandlerWithHeaders) error {
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetry
	}

	args := amqp.Table{
		"x-dead-letter-exchange":    SagaDLX,
		"x-dead-letter-routing-key": DLQRoutingKey,
	}

	q, err := r.declareQueueSafe(queueName, args)
	if err != nil {
		return fmt.Errorf("failed to declare queue %s with DLX: %w", queueName, err)
	}

	err = r.channel.QueueBind(
		q.Name,
		routingKey,
		SagaExchange,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue %s to exchange %s: %w", q.Name, SagaExchange, err)
	}

	msgs, err := r.channel.Consume(
		q.Name,
		"",    // consumer
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to consume from %s: %w", q.Name, err)
	}

	go func() {
		for d := range msgs {
			// Extract correlation ID from message properties or headers
			corrID := d.CorrelationId
			if corrID == "" && d.Headers != nil {
				if cid, ok := d.Headers["x-correlation-id"].(string); ok && cid != "" {
					corrID = cid
				} else if tid, ok := d.Headers["x-trace-id"].(string); ok && tid != "" {
					corrID = tid
				}
			}
			if corrID == "" {
				corrID = GenerateCorrelationID()
			}
			if d.Headers == nil {
				d.Headers = amqp.Table{}
			}
			d.Headers["x-correlation-id"] = corrID

			// Extract retry count from headers
			retryCount := 0
			if d.Headers != nil {
				if rc, ok := d.Headers["x-retry-count"]; ok {
					if v, ok := rc.(int32); ok {
						retryCount = int(v)
					} else if v, ok := rc.(int); ok {
						retryCount = v
					} else if v, ok := rc.(int64); ok {
						retryCount = int(v)
					}
				}
			}

			StructuredLog("INFO", "inventory-service", corrID, fmt.Sprintf("RabbitMQ Consuming from queue %s [%s]", queueName, routingKey), map[string]interface{}{
				"queue":      queueName,
				"routingKey": routingKey,
				"retryCount": retryCount,
			})

			handlerErr := handler(d.Body, d.Headers)
			if handlerErr == nil {
				_ = d.Ack(false)
				continue
			}

			if IsUnprocessable(handlerErr) {
				StructuredLog("WARN", "inventory-service", corrID, fmt.Sprintf("[DLQ ROUTE - Poison/Unprocessable] %s on queue %s: %v", routingKey, queueName, handlerErr), map[string]interface{}{
					"error":  handlerErr.Error(),
					"reason": "UNPROCESSABLE_PAYLOAD",
				})
				_ = r.PublishToDLQ(routingKey, d.Body, "UNPROCESSABLE_PAYLOAD", handlerErr.Error(), retryCount)
				_ = d.Ack(false)
				continue
			}

			if retryCount < maxRetries {
				nextRetry := retryCount + 1
				StructuredLog("WARN", "inventory-service", corrID, fmt.Sprintf("[RETRY %d/%d] Message on %s failed (%v), retrying...", nextRetry, maxRetries, routingKey, handlerErr), map[string]interface{}{
					"error":      handlerErr.Error(),
					"retryCount": nextRetry,
				})

				time.Sleep(100 * time.Millisecond)

				newHeaders := amqp.Table{}
				for k, v := range d.Headers {
					newHeaders[k] = v
				}
				newHeaders["x-retry-count"] = int32(nextRetry)
				newHeaders["x-last-error"] = handlerErr.Error()
				newHeaders["x-correlation-id"] = corrID

				_ = r.PublishWithHeaders(SagaExchange, routingKey, d.Body, newHeaders)
				_ = d.Ack(false)
			} else {
				StructuredLog("ERROR", "inventory-service", corrID, fmt.Sprintf("[DLQ ROUTE - Retries Exhausted (%d/%d)] Message on %s routed to DLQ: %v", retryCount, maxRetries, routingKey, handlerErr), map[string]interface{}{
					"error":  handlerErr.Error(),
					"reason": "MAX_RETRIES_EXCEEDED",
				})
				_ = r.PublishToDLQ(routingKey, d.Body, "MAX_RETRIES_EXCEEDED", handlerErr.Error(), retryCount)
				_ = d.Ack(false)
			}
		}
	}()

	return nil
}

func (r *RabbitMQ) Consume(queueName, routingKey string, handler func([]byte)) error {
	return r.ConsumeWithRetry(queueName, routingKey, DefaultMaxRetry, func(body []byte, headers amqp.Table) error {
		handler(body)
		return nil
	})
}

func (r *RabbitMQ) PeekDLQ(limit int) ([]DLQMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	ch, err := r.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("channel error: %w", err)
	}
	defer ch.Close()

	q, err := ch.QueueInspect(SagaDLQ)
	if err != nil {
		return nil, fmt.Errorf("inspect queue error: %w", err)
	}

	var messages []DLQMessage
	var deliveries []amqp.Delivery

	for i := 0; i < limit && i < q.Messages; i++ {
		msg, ok, err := ch.Get(SagaDLQ, false)
		if err != nil || !ok {
			break
		}
		deliveries = append(deliveries, msg)

		dlqMsg := extractDLQMessage(msg)
		messages = append(messages, dlqMsg)
	}

	for _, d := range deliveries {
		_ = d.Nack(false, true)
	}

	if messages == nil {
		messages = []DLQMessage{}
	}
	return messages, nil
}

func (r *RabbitMQ) PurgeDLQ() (int, error) {
	ch, err := r.conn.Channel()
	if err != nil {
		return 0, err
	}
	defer ch.Close()

	return ch.QueuePurge(SagaDLQ, false)
}

func (r *RabbitMQ) ReprocessDLQ() (int, error) {
	ch, err := r.conn.Channel()
	if err != nil {
		return 0, err
	}
	defer ch.Close()

	q, err := ch.QueueInspect(SagaDLQ)
	if err != nil {
		return 0, err
	}

	reprocessed := 0
	for i := 0; i < q.Messages; i++ {
		msg, ok, err := ch.Get(SagaDLQ, false)
		if err != nil || !ok {
			break
		}

		origRoutingKey := msg.RoutingKey
		if val, ok := msg.Headers["x-original-routing-key"].(string); ok && val != "" {
			origRoutingKey = val
		}
		origExchange := SagaExchange
		if val, ok := msg.Headers["x-original-exchange"].(string); ok && val != "" {
			origExchange = val
		}

		headers := amqp.Table{}
		if msg.Headers != nil {
			for k, v := range msg.Headers {
				headers[k] = v
			}
		}
		headers["x-retry-count"] = int32(0)
		headers["x-reprocessed-at"] = time.Now().Format(time.RFC3339)

		err = ch.Publish(
			origExchange,
			origRoutingKey,
			false,
			false,
			amqp.Publishing{
				ContentType:  msg.ContentType,
				DeliveryMode: amqp.Persistent,
				Headers:      headers,
				Body:         msg.Body,
			},
		)
		if err == nil {
			_ = msg.Ack(false)
			reprocessed++
		} else {
			_ = msg.Nack(false, true)
		}
	}
	return reprocessed, nil
}

func (r *RabbitMQ) GetDLQStats() (map[string]interface{}, error) {
	ch, err := r.conn.Channel()
	if err != nil {
		return nil, err
	}
	defer ch.Close()

	q, err := ch.QueueInspect(SagaDLQ)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"queueName":          q.Name,
		"messageCount":       q.Messages,
		"consumerCount":      q.Consumers,
		"deadLetterExchange": SagaDLX,
	}, nil
}

func extractDLQMessage(msg amqp.Delivery) DLQMessage {
	deathReason := "UNSPECIFIED"
	origRoutingKey := msg.RoutingKey
	origExchange := msg.Exchange
	retryCount := 0
	errMsg := ""

	if val, ok := msg.Headers["x-death-reason"].(string); ok {
		deathReason = val
	}
	if val, ok := msg.Headers["x-original-routing-key"].(string); ok {
		origRoutingKey = val
	}
	if val, ok := msg.Headers["x-original-exchange"].(string); ok {
		origExchange = val
	}
	if val, ok := msg.Headers["x-error-message"].(string); ok {
		errMsg = val
	}
	if val, ok := msg.Headers["x-retry-count"]; ok {
		if rc, ok := val.(int32); ok {
			retryCount = int(rc)
		} else if rc, ok := val.(int); ok {
			retryCount = rc
		} else if rc, ok := val.(int64); ok {
			retryCount = int(rc)
		}
	}

	if xDeath, ok := msg.Headers["x-death"].([]interface{}); ok && len(xDeath) > 0 {
		if deathTable, ok := xDeath[0].(amqp.Table); ok {
			if r, ok := deathTable["reason"].(string); ok && deathReason == "UNSPECIFIED" {
				deathReason = fmt.Sprintf("RABBITMQ_NACK_%s", strings.ToUpper(r))
			}
			if ex, ok := deathTable["exchange"].(string); ok && origExchange == "" {
				origExchange = ex
			}
			if rk, ok := deathTable["routing-keys"].([]interface{}); ok && len(rk) > 0 {
				if rkStr, ok := rk[0].(string); ok && origRoutingKey == "" {
					origRoutingKey = rkStr
				}
			}
		}
	}

	var payloadJSON map[string]interface{}
	_ = json.Unmarshal(msg.Body, &payloadJSON)

	return DLQMessage{
		MessageID:          msg.MessageId,
		OriginalExchange:   origExchange,
		OriginalRoutingKey: origRoutingKey,
		DeathReason:        deathReason,
		ErrorMessage:       errMsg,
		RetryCount:         retryCount,
		Payload:            string(msg.Body),
		PayloadJSON:        payloadJSON,
		Timestamp:          msg.Timestamp,
	}
}
