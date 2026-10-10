# Requirements Document: Distributed Ticket Booking System

## 1. Project Overview
An event ticket booking system built with a microservices architecture. It includes services for user management, highly concurrent seat inventory, and payments. The project includes both a backend of microservices and a React frontend. The system relies on an API Gateway to coordinate requests and uses distributed locking mechanisms to prevent seat overselling under heavy load.

## 2. Acceptance Criteria
- [x] All microservices start successfully using `docker-compose up`.
- [x] An automated integration test successfully creates a booking through the API Gateway, and a subsequent query confirms the seat inventory has accurately decreased.
- [x] The React frontend compiles without errors, starts successfully, and correctly fetches the event catalog from the backend API.
- [x] Concurrency Test: System must handle 50 concurrent requests for the exact same seat, resolving exactly 1 booking and 49 conflicts with zero overselling.

## 3. Mandatory Technical Requirements

**R1: The system must provide a React-based frontend for users.**  
Explanation: Implement a web frontend using React and TypeScript that consumes the backend APIs. Users should be able to view available events and book seats.

**R2: The system must provide a microservices architecture backend.**  
Explanation: Core business domains are strictly separated into autonomous services including at least an Event Catalog Service, a Seat Inventory Service, and a Booking Service.

**R3: The system must provide an API Gateway for request routing.** ✅ (Done successfully)  
Explanation: The API Gateway serves as the single entry point for external client traffic, routing requests to downstream microservices and handling cross-cutting concerns.

**R3.1: The system must implement robust Authentication and Authorization.**  
Explanation: Implement stateless authentication (e.g., JWT). The API Gateway must validate tokens, and a dedicated Auth/User Service should handle user registration and credential verification to ensure tickets are securely tied to user identities.

**R4: The system must use PostgreSQL as a database system for data storage.**  
Explanation: Relational database instances ensure ACID transactions and persistent state for event, seat, and booking records.

**R5: The system must be fully containerized using Docker and Docker Compose.** ✅ (Done successfully)  
Explanation: The entire system (all microservices, the frontend, databases, and message brokers) must be orchestratable via a single `docker-compose.yml` file.

**R6: The system must achieve automated integration test verification across the API Gateway.** ✅ (Done successfully)  
Explanation: Automated integration suites must execute end-to-end booking transactions via the gateway and assert that persistent seat inventory decrements accurately.

**R7: The system must pass a 50-client concurrent contention test without overselling.** ✅ (Done successfully)  
Explanation: Under an automated test dispatching 50 concurrent requests for the identical seat, the system must resolve exactly 1 successful booking and 49 conflict rejections (HTTP 409).

**R8: The system must implement distributed caching for read-heavy operations.** ✅ (Done successfully)  
Explanation: An in-memory data store (e.g., Redis) must be used to cache event catalogs and seat maps, reducing latency and database load during high-traffic spikes.

## 4. Functional Requirements

### 4.1 Must Requirements

**R9: The system must provide event catalog browsing functionality to users.** ✅ (Done successfully)  
Explanation: Users can retrieve and view all published events, schedules, and venue details via the frontend interface.

**R10: The system must provide real-time seat availability retrieval functionality to users.**  
Explanation: The system queries the Seat Inventory Service to present current seat states (available, held, reserved).

**R11: The system must provide seat reservation locking functionality to users.** ✅ (Done successfully)  
Explanation: The system temporarily holds a selected seat upon initiating checkout, preventing simultaneous reservation by other users via distributed Redis locks.

**R12: The system must prevent overbooking and race conditions on concurrent seat reservations.** ✅ (Done successfully)  
Explanation: Concurrency control mechanisms enforce transactional isolation so that a single physical seat cannot be allocated to multiple users.

**R13: The system must provide booking creation functionality to users.**  
Explanation: The Booking Service creates a pending booking record linked to the reserved seat and user identity.

**R14: The system must enforce idempotent API requests for financial transactions.**  
Explanation: The payment endpoint must require an idempotency key to prevent double-charging a user in the event of network retries or dropped connections.

**R14.1: The system must provide standardized API Documentation.**  
Explanation: All microservice APIs must be documented using the OpenAPI Specification (Swagger) to facilitate contract-driven development and frontend integration.

**R15: The system must implement the Saga Pattern for distributed transaction orchestration.**  
Explanation: Multi-service workflows (e.g., deducting inventory and capturing payment) must emit compensating transactions if any downstream step fails.

**R16: The system must provide booking confirmation status information to users.**  
Explanation: Displays definitive reservation results, unique booking reference identifiers, and payment status upon transaction completion.

**R17: The system must provide structured backend logging and distributed tracing.**  
Explanation: Centralized log output tracks transaction execution, correlation IDs, and distributed errors across all microservice instances.

**R18: The system must provide Dead Letter Queue (DLQ) functionality for asynchronous messaging.**  
Explanation: Unprocessable Kafka messages or failed compensating events must be routed to a DLQ for manual inspection and retry.

### 4.2 Should Requirements

**R19: The system should provide automatic seat release on reservation timeout.**  
Explanation: If payment is not completed within a designated time window (e.g., 10 minutes), the system automatically releases the held seat back to the general inventory.

**R20: The system should provide automatic waitlist promotion functionality.**  
Explanation: When a user cancels a confirmed ticket or abandons a checkout, the system automatically reserves the seat for the next user in a FIFO waitlist queue.

**R21: The system should provide booking cancellation functionality to users.**  
Explanation: Users may cancel pending or completed reservations, triggering inventory compensation workflows.

**R22: The system should provide asynchronous event messaging between microservices.**  
Explanation: Message brokers (like Kafka or RabbitMQ) decouple service dependencies, publishing domain events (e.g., `OrderPlaced`, `PaymentFailed`) across service boundaries.

**R23: The system should provide health check and metrics endpoints.**  
Explanation: Microservices expose actuator endpoints (e.g., `/health`, `/metrics`) for continuous monitoring of connection pool saturation, memory usage, and component status.

**R24: The system should provide dialog confirmation functionality to users.**  
Explanation: Modal dialogues prompt users to confirm seat selections and payment submissions before executing state changes.

### 4.3 Can Requirements

**R25: The system can provide visual interactive seat map visualization to users.**  
Explanation: Renders an interactive 2D floor grid allowing users to click and select specific seats directly.

**R26: The system can provide webhook notifications to external partners.**  
Explanation: Emits HTTP callbacks to third-party event organizers when an event reaches maximum capacity or triggers specific revenue thresholds.

**R27: The system can provide dynamic surge pricing for highly contested events.**  
Explanation: Adjusts real-time ticket pricing based on inventory scarcity and current booking velocity.

**R28: The system can provide dark mode interface toggle functionality to users.**  
Explanation: Allows switching between light and dark visual themes on the React frontend.

## 5. Non-Functional Requirements

**R29: The system shall guarantee strict inventory consistency under high load.** ✅ (Done successfully)  
Explanation: Distributed locking and transactional isolation ensure that seat allocations remain accurate and deterministic under burst traffic.

**R30: The system shall maintain low response latency for read operations.** ✅ (Done successfully)  
Explanation: Event catalog and seat layout queries must execute with sub-500ms response times under standard operating conditions.

**R31: The system shall enforce API rate limiting.**  
Explanation: The API Gateway restricts individual IP addresses or user accounts to a predefined number of reservation requests per minute to mitigate bot-driven scalping.

**R32: The system shall be usable on modern desktop browsers for users.**  
Explanation: The React frontend complies with modern web standards and renders correctly on current versions of Chromium, Firefox, and WebKit browsers.

**R33: The system shall provide service isolation and fault tolerance.**  
Explanation: Failure of an isolated downstream service (e.g., Event Catalog) must not cause cascading unhandled crashes in unrelated services (e.g., Seat Inventory).