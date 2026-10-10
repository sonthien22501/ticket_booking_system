# Technical Documentation
## Ticket Booking System

### 1. Introduction
The Ticket Booking System is a highly concurrent, microservices-based application designed to handle large spikes in traffic for event ticketing. It guarantees zero overselling using distributed locks and ensures data consistency across services via the Saga orchestration pattern.

### 2. Tech Stack
- **Backend**: Go (Golang) v1.22
- **Frontend**: React, TypeScript, Vite
- **Database**: PostgreSQL 16
- **Caching & Locking**: Redis 7
- **Message Broker**: RabbitMQ 3
- **Gateway**: NGINX / Go API Gateway
- **Containerization**: Docker & Docker Compose

### 3. Installation & Setup
To run this project locally, ensure you have Docker and Docker Compose installed.

1. **Clone the repository**
   ```bash
   git clone <repo-url>
   cd ticket_booking_system
   ```
2. **Start the Infrastructure and Services**
   ```bash
   docker compose up -d --build
   ```
3. **Verify Health**
   All services expose a `/health` endpoint.
   - Gateway: `http://localhost:8080/health`
   - Frontend: `http://localhost:3000`

### 4. API Endpoints Overview

All backend traffic is routed through the API Gateway at `http://localhost:8080/api`.

#### Catalog Service
- `GET /api/events` - Retrieve a paginated list of events (Cached in Redis)
- `GET /api/events/:id` - Get specific event details and ticket tiers

#### Inventory Service (Internal & Saga Driven)
- `POST /api/inventory/reserve` - Hold seats for a booking (Requires Redis Lock)
- `POST /api/inventory/commit` - Finalize a seat reservation
- `POST /api/inventory/release` - Release a held seat

#### Booking Service
- `POST /api/bookings` - Create a new booking (Triggers the Saga Orchestrator)
- `GET /api/bookings` - List user bookings
- `GET /api/bookings/:id` - Retrieve a specific booking and its generated tickets

### 5. Testing
The system includes an automated E2E integration test simulating high concurrency.
```bash
# Run backend tests
cd backend/inventory-service
go test ./...
```
