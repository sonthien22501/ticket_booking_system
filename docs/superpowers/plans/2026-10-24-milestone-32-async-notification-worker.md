# Milestone 32: Asynchronous Multi-Channel Notification Microservice (Email/SMS Worker)

## Overview
Implement a resilient, asynchronous event-driven notification microservice that consumes domain events (`booking.confirmed`, `waitlist.promoted`, `ticket.transferred`, `booking.cancelled`) from RabbitMQ and delivers multi-channel communications (HTML Email with digital ticket access, and transactional SMS). Supports Dead Letter Exchange (DLX/DLQ), exponential retry policies, and persistent audit delivery logging in PostgreSQL.

---

## Architecture & Data Flow

```
[ Booking Service / Saga / Waitlist ]
                   │
                   ▼ (publish event)
          [ RabbitMQ: ticket_exchange ]
          ┌───────────────┴───────────────┐
          ▼                               ▼
[ notification_queue ]           [ notification_dlq ]
          │                               ▲
          ▼ (consume & dispatch)          │ (after max retries)
[ Notification Service Worker (:8085) ] ──┘
          │
          ├─────────────────────────┐
          ▼                         ▼
   [ Email Channel ]         [ SMS Channel ]
  (HTML ticket pass)        (SMS confirmation)
          │                         │
          └───────────┬─────────────┘
                      ▼
        [ notification_deliveries DB ]
```

---

## 1. Database Schema
- Table: `notification_deliveries`
  - `id VARCHAR(64) PRIMARY KEY`
  - `channel VARCHAR(32) NOT NULL` (e.g. `EMAIL`, `SMS`)
  - `recipient VARCHAR(255) NOT NULL`
  - `event_type VARCHAR(64) NOT NULL`
  - `subject VARCHAR(255)`
  - `body TEXT NOT NULL`
  - `status VARCHAR(32) NOT NULL DEFAULT 'DELIVERED'` (e.g. `QUEUED`, `DELIVERED`, `FAILED`, `RETRIED`)
  - `attempts INT DEFAULT 1`
  - `error_message TEXT`
  - `created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()`
- Indexes on `(recipient, created_at DESC)` and `status`.

---

## 2. Microservice Implementation
- Directory: `backend/notification-service/`
  - `main.go`: HTTP server on `:8085`, healthcheck (`/health`), delivery history (`/api/notifications/deliveries` & `/deliveries`), test dispatch trigger (`/api/notifications/dispatch-test` & `/dispatch-test`).
  - `worker.go`: RabbitMQ connection, queue declare with DLX/DLQ bindings, consumer loop, multi-channel dispatchers (`dispatchEmail`, `dispatchSMS`), exponential backoff retries, and DB persistence.
  - `worker_test.go`: RED-GREEN unit tests for email templating, SMS formatting, error handling, and delivery logging.
  - `Dockerfile`: Multi-stage Go build container.

---

## 3. Infrastructure & Gateway
- `docker-compose.yml`: Register `notification-service` container on port 8085 with dependencies on `postgres` and `rabbitmq`.
- `backend/api-gateway/nginx.conf`: Add upstream `notification_service` and route `/api/notifications/deliveries` and `/api/notifications/dispatch-test`.
- `infra/postgres/init.sql` & `scripts/seed_test_dataset.sql`: Add `notification_deliveries` table and seed records.

---

## 4. Frontend & Testing
- `frontend/src/types/index.ts`: Add `NotificationDeliveryLog`.
- `frontend/src/services/api.ts`: Add `fetchNotificationDeliveries`, `dispatchTestNotification`.
- `frontend/src/test/notification_worker.test.ts`: Vitest test suite.
