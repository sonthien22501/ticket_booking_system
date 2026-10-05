# Project Requirements: Ticket Booking System

## Overview
An event ticket booking system built with a microservices architecture. It includes services for user management, highly concurrent seat inventory, and payments. The project includes both a backend of microservices and a React frontend.

## Requirements

### R1. Backend Microservices
Implement a microservices backend comprising at least an Event Catalog Service, a Seat Inventory Service, and a Booking Service. Use an API Gateway to route external requests. 

### R2. Frontend Interface
Implement a web frontend using React and TypeScript that consumes the backend APIs. Users should be able to view available events and book seats.

### R3. Containerization
The entire system (all microservices, the frontend, and any necessary databases or message brokers) must be containerized using Docker and orchestratable via a single `docker-compose.yml` file.

## Acceptance Criteria
- [x] All microservices start successfully using `docker-compose up`.
- [x] An automated integration test successfully creates a booking through the API Gateway, and a subsequent query confirms the seat inventory has accurately decreased.
- [x] The React frontend compiles without errors, starts successfully, and correctly fetches the event catalog from the backend API.
- [x] Concurrency Test: System must handle 50 concurrent requests for the exact same seat, resolving exactly 1 booking and 49 conflicts with zero overselling.
