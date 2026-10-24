package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Supported delivery channels
const (
	ChannelEmail = "EMAIL"
	ChannelSMS   = "SMS"
	ChannelPush  = "PUSH"
)

// Delivery statuses
const (
	StatusQueued    = "QUEUED"
	StatusDelivered = "DELIVERED"
	StatusFailed    = "FAILED"
	StatusRetried   = "RETRIED"
)

// RabbitMQ topology constants
const (
	ExchangeName     = "ticket_exchange"
	DLXName          = "ticket_notifications_dlx"
	QueueName        = "ticket_notifications_queue"
	DLQName          = "ticket_notifications_dlq"
	MaxDeliveryTries = 3
)

// NotificationEvent represents an incoming event payload
type NotificationEvent struct {
	EventType     string    `json:"eventType"`
	BookingRef    string    `json:"bookingRef,omitempty"`
	EventTitle    string    `json:"eventTitle,omitempty"`
	Recipient     string    `json:"recipient"`
	CustomerName  string    `json:"customerName,omitempty"`
	Seats         []string  `json:"seats,omitempty"`
	TotalPrice    float64   `json:"totalPrice,omitempty"`
	Currency      string    `json:"currency,omitempty"`
	TicketPassURL string    `json:"ticketPassUrl,omitempty"`
	Timestamp     time.Time `json:"timestamp,omitempty"`
}

// NotificationDelivery represents an audited message delivery record
type NotificationDelivery struct {
	ID           string    `json:"id"`
	Channel      string    `json:"channel"`
	Recipient    string    `json:"recipient"`
	EventType    string    `json:"eventType"`
	Subject      string    `json:"subject"`
	Body         string    `json:"body"`
	Status       string    `json:"status"`
	Attempts     int       `json:"attempts"`
	ErrorMessage string    `json:"errorMessage,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// NotificationWorker coordinates message consumption and multi-channel dispatch
type NotificationWorker struct {
	db         *sql.DB
	rmqConn    *amqp.Connection
	rmqChan    *amqp.Channel
	mu         sync.RWMutex
	deliveries []NotificationDelivery
}

// NewNotificationWorker creates an instance of the worker
func NewNotificationWorker(db *sql.DB, rmqConn *amqp.Connection) *NotificationWorker {
	now := time.Now().UTC()
	initialSeed := []NotificationDelivery{
		{
			ID:        "del-seed-001",
			Channel:   ChannelEmail,
			Recipient: "user@tickets.com",
			EventType: "booking.confirmed",
			Subject:   "Your Tickets are Confirmed: Neon Symphony 2026",
			Body:      "<h1>Booking Confirmed</h1><p>Dear Alex Morgan, your reservation is confirmed!</p>",
			Status:    StatusDelivered,
			Attempts:  1,
			CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			ID:        "del-seed-002",
			Channel:   ChannelSMS,
			Recipient: "+15551234567",
			EventType: "waitlist.promoted",
			Subject:   "SMS Alert",
			Body:      "TicketVerse: Hold available for Cyberpunk Symphony! Claim your seat within 15 min.",
			Status:    StatusDelivered,
			Attempts:  1,
			CreatedAt: now.Add(-30 * time.Minute),
		},
	}

	return &NotificationWorker{
		db:         db,
		rmqConn:    rmqConn,
		deliveries: initialSeed,
	}
}

// formatEmailMessage generates rich HTML body and descriptive subject line
func formatEmailMessage(evt NotificationEvent) (string, string) {
	currency := evt.Currency
	if currency == "" {
		currency = "USD"
	}
	seatsStr := strings.Join(evt.Seats, ", ")
	if seatsStr == "" {
		seatsStr = "General Admission"
	}

	switch evt.EventType {
	case "booking.confirmed":
		subject := fmt.Sprintf("Booking Confirmed: %s (Ref: %s)", evt.EventTitle, evt.BookingRef)
		htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><style>body { font-family: sans-serif; background: #0f172a; color: #f8fafc; }</style></head>
<body>
  <h2>Order Confirmed! 🎉</h2>
  <p>Dear <strong>%s</strong>,</p>
  <p>Your tickets for <strong>%s</strong> are confirmed and cryptographically signed.</p>
  <ul>
    <li>Booking Ref: <code>%s</code></li>
    <li>Reserved Seats: <strong>%s</strong></li>
    <li>Total Amount: <strong>%.2f %s</strong></li>
  </ul>
  <p><a href="%s" style="display:inline-block;padding:10px 20px;background:#f59e0b;color:#000;text-decoration:none;border-radius:8px;font-weight:bold;">View Digital Passbook QR Pass</a></p>
</body>
</html>`, evt.CustomerName, evt.EventTitle, evt.BookingRef, seatsStr, evt.TotalPrice, currency, evt.TicketPassURL)
		return subject, htmlBody

	case "waitlist.promoted":
		subject := fmt.Sprintf("Priority Hold Available: %s", evt.EventTitle)
		htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html><body>
  <h2>You're off the waitlist! 🎟️</h2>
  <p>Dear %s, a seat opened up for <strong>%s</strong>.</p>
  <p>You have <strong>15 minutes</strong> to claim your priority reservation before it passes to the next person in line.</p>
</body></html>`, evt.CustomerName, evt.EventTitle)
		return subject, htmlBody

	case "ticket.transferred":
		subject := fmt.Sprintf("Ticket Transfer Confirmation: %s", evt.EventTitle)
		htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html><body>
  <h2>Ticket Transferred 🤝</h2>
  <p>Dear %s, ticket for <strong>%s</strong> has been transferred successfully.</p>
  <p>Reference: %s. Previous ticket barcodes have been deactivated.</p>
</body></html>`, evt.CustomerName, evt.EventTitle, evt.BookingRef)
		return subject, htmlBody

	case "booking.cancelled":
		fallthrough
	default:
		subject := fmt.Sprintf("Ticket Notice: %s", evt.EventTitle)
		htmlBody := fmt.Sprintf(`<p>Hello %s, notice regarding your booking %s for %s.</p>`, evt.CustomerName, evt.BookingRef, evt.EventTitle)
		return subject, htmlBody
	}
}

// formatSMSMessage generates character-bounded SMS messages
func formatSMSMessage(evt NotificationEvent) string {
	switch evt.EventType {
	case "booking.confirmed":
		return fmt.Sprintf("TicketVerse: Confirmed! %s. Ref: %s. Seats: %s. Access pass at gate.",
			evt.EventTitle, evt.BookingRef, strings.Join(evt.Seats, ","))
	case "waitlist.promoted":
		return fmt.Sprintf("TicketVerse: Waitlist hold available for %s! Claim reservation within 15 mins: ref %s",
			evt.EventTitle, evt.BookingRef)
	case "ticket.transferred":
		return fmt.Sprintf("TicketVerse: Ticket transferred for %s (Ref: %s). New digital entry code generated.",
			evt.EventTitle, evt.BookingRef)
	default:
		return fmt.Sprintf("TicketVerse: Update for %s (Ref: %s).", evt.EventTitle, evt.BookingRef)
	}
}

// processEvent handles formatting, mock gateway delivery, and persistent audit logging
func (w *NotificationWorker) processEvent(evt NotificationEvent, channel string) (*NotificationDelivery, error) {
	var subject, body string

	if channel == ChannelSMS {
		subject = "Transactional SMS"
		body = formatSMSMessage(evt)
	} else {
		channel = ChannelEmail
		subject, body = formatEmailMessage(evt)
	}

	deliveryID := fmt.Sprintf("del-%d-%s", time.Now().UnixNano()%10000000, strings.ToLower(channel))
	delivery := NotificationDelivery{
		ID:        deliveryID,
		Channel:   channel,
		Recipient: evt.Recipient,
		EventType: evt.EventType,
		Subject:   subject,
		Body:      body,
		Status:    StatusDelivered,
		Attempts:  1,
		CreatedAt: time.Now().UTC(),
	}

	// Persist to Postgres if connected
	if w.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err := w.db.ExecContext(ctx, `
			INSERT INTO notification_deliveries (
				id, channel, recipient, event_type, subject, body, status, attempts, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		`, delivery.ID, delivery.Channel, delivery.Recipient, delivery.EventType, delivery.Subject, delivery.Body, delivery.Status, delivery.Attempts)
		if err != nil {
			log.Printf("Warning: failed to persist delivery to postgres: %v", err)
		}
	}

	// Store in memory
	w.mu.Lock()
	w.deliveries = append([]NotificationDelivery{delivery}, w.deliveries...)
	w.mu.Unlock()

	log.Printf("[Worker] Successfully dispatched %s to %s for event %s (ID: %s)", channel, evt.Recipient, evt.EventType, delivery.ID)
	return &delivery, nil
}

// GetDeliveries retrieves audited message records
func (w *NotificationWorker) GetDeliveries() []NotificationDelivery {
	if w.db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		rows, err := w.db.QueryContext(ctx, `
			SELECT id, channel, recipient, event_type, COALESCE(subject, ''), body, status, attempts, COALESCE(error_message, ''), created_at
			FROM notification_deliveries
			ORDER BY created_at DESC
			LIMIT 50
		`)
		if err == nil {
			defer rows.Close()
			var list []NotificationDelivery
			for rows.Next() {
				var d NotificationDelivery
				if err := rows.Scan(&d.ID, &d.Channel, &d.Recipient, &d.EventType, &d.Subject, &d.Body, &d.Status, &d.Attempts, &d.ErrorMessage, &d.CreatedAt); err == nil {
					list = append(list, d)
				}
			}
			if len(list) > 0 {
				return list
			}
		}
	}

	w.mu.RLock()
	defer w.mu.RUnlock()
	res := make([]NotificationDelivery, len(w.deliveries))
	copy(res, w.deliveries)
	return res
}

// StartConsumer establishes RabbitMQ queue bindings and consumes messages
func (w *NotificationWorker) StartConsumer(ctx context.Context) error {
	if w.rmqConn == nil {
		log.Println("[Worker] No active RabbitMQ connection, running in autonomous HTTP mode")
		return nil
	}

	ch, err := w.rmqConn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open rmq channel: %w", err)
	}
	w.rmqChan = ch

	// Declare Dead Letter Exchange & Queue
	_ = ch.ExchangeDeclare(DLXName, "direct", true, false, false, false, nil)
	_, _ = ch.QueueDeclare(DLQName, true, false, false, false, nil)
	_ = ch.QueueBind(DLQName, "notification.failed", DLXName, false, nil)

	// Declare Main Exchange
	_ = ch.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil)

	// Declare Main Queue with DLX
	queueArgs := amqp.Table{
		"x-dead-letter-exchange":    DLXName,
		"x-dead-letter-routing-key": "notification.failed",
	}
	q, err := ch.QueueDeclare(QueueName, true, false, false, false, queueArgs)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind key events
	events := []string{"booking.confirmed", "waitlist.promoted", "ticket.transferred", "booking.cancelled"}
	for _, e := range events {
		_ = ch.QueueBind(q.Name, e, ExchangeName, false, nil)
	}

	msgs, err := ch.Consume(q.Name, "notification-worker", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to consume messages: %w", err)
	}

	go func() {
		log.Printf("[Worker] RabbitMQ Consumer listening on queue '%s'", q.Name)
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					return
				}

				var evt NotificationEvent
				if err := json.Unmarshal(msg.Body, &evt); err != nil {
					log.Printf("[Worker] Malformed message, rejecting to DLQ: %v", err)
					_ = msg.Nack(false, false)
					continue
				}

				// Dispatch to Email
				_, _ = w.processEvent(evt, ChannelEmail)

				// If recipient looks like phone number, dispatch SMS
				if strings.HasPrefix(evt.Recipient, "+") {
					_, _ = w.processEvent(evt, ChannelSMS)
				}

				_ = msg.Ack(false)
			}
		}
	}()

	return nil
}
