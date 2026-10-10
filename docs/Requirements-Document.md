# Requirements Document
## Ticket Booking System

### Overview
An event ticket booking system built with a microservices architecture. It includes services for user management, highly concurrent seat inventory, and payments. The project includes both a backend of microservices and a React frontend.

### Functional Requirements
1. **Event Catalog**: Users must be able to view a list of upcoming events, including ticket tiers and pricing.
2. **Seat Selection**: Users must be able to select specific seats or request auto-allocation of seats for an event.
3. **Booking Process**: Users must be able to hold seats, process a simulated payment, and receive confirmed tickets.
4. **Concurrency**: The system must flawlessly handle scenarios where dozens of users attempt to purchase the exact same seat simultaneously.

### Non-Functional Requirements
1. **Architecture**: Microservices (Auth, Catalog, Booking, Inventory).
2. **Database**: PostgreSQL for relational data.
3. **Caching**: Redis for caching the event catalog to reduce DB load.
4. **Consistency**: RabbitMQ must be used to implement the Saga pattern for distributed transactions (e.g., reverting seat holds if payment fails).
5. **Containerization**: The entire application (DBs, Broker, Backend, Frontend) must be orchestratable via a single `docker-compose.yml`.

### Acceptance Criteria
- [x] All microservices start successfully using `docker-compose up`.
- [x] Concurrency Test: System must handle 50 concurrent requests for the exact same seat, resolving exactly 1 booking and 49 conflicts with zero overselling.
- [x] Saga Test: A booking with a failed payment automatically releases the held seats in the inventory service.
- [x] The React frontend compiles without errors, starts successfully, and correctly fetches the event catalog from the backend API.
