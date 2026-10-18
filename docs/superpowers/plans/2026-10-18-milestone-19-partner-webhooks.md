# Milestone 19: Webhook Notifications to External Partners & Asynchronous Domain Events (R26, R22) Implementation Plan

## Goal
Implement external partner webhook notification capabilities (**Requirement R26**) and asynchronous domain event messaging between microservices (**Requirement R22**). External event organizers and partners can register callback URLs with custom secrets to receive signed HTTP POST notifications with HMAC-SHA256 signatures when an event reaches maximum capacity (`event.capacity_reached`), crosses revenue thresholds (`event.revenue_threshold`), or creates new bookings (`order.placed`). All domain events are also broadcast over RabbitMQ (`SagaExchange`) to decouple microservice boundaries.

---

## Technical Design & Scope
1. **Webhook Subsystem & Manager (`backend/booking-service/webhook.go`)**:
   - `WebhookSubscription`: Models target URL, secret, subscribed events (`event.capacity_reached`, `event.revenue_threshold`, `order.placed`), active state, and timestamps.
   - `WebhookDelivery`: Immutable audit log of every callback attempt containing response status code, round-trip latency (`durationMs`), payload JSON, success flag, and error messages.
   - `computeHMACSignature()`: Computes `X-Webhook-Signature: sha256=<hex>` using partner's secret key for tamper-proof verification.
   - `DispatchEvent()`: Dispatches signed HTTP POST callbacks concurrently to matching partner endpoints with configurable timeouts, recording delivery audit logs, while simultaneously emitting persistent messages to RabbitMQ topic exchange `SagaExchange` under routing key `domain.event.<type>`.
2. **REST API Endpoints**:
   - `POST /api/webhooks/subscriptions`: Register new partner webhook.
   - `GET /api/webhooks/subscriptions`: List subscriptions with optional `?eventId=` filter.
   - `GET /api/webhooks/subscriptions/{id}`: Retrieve single subscription.
   - `DELETE /api/webhooks/subscriptions/{id}`: Delete subscription.
   - `GET /api/webhooks/deliveries`: Query audit delivery history (`?subscriptionId=`, `?limit=`).
   - `POST /api/webhooks/dispatch`: Test dispatch endpoint for automated verification and debugging.
3. **API Gateway Routing (`backend/api-gateway/nginx.conf`)**:
   - Proxies `/api/webhooks` and `/webhooks` to `booking-service` on port 8083 with rate limiting and tracing header injection.
4. **OpenAPI 3.0 Contract Updates (`openapi.yaml` & `openapi.json`)**:
   - Documented all webhook routes, parameters, request bodies, and responses.
5. **Automated Verification**:
   - In-memory unit test suite: `backend/booking-service/webhook_test.go` (100% pass).
   - E2E integration test suite: `e2e/webhook_test.go`.

---

## Verification
- Unit test suite: `go test -v -run "TestWebhook.*" ./backend/booking-service` (100% pass).
- Gateway contract test: `go test -run "TestSwagger" ./e2e` (100% pass).
