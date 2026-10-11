# Architecture & Design Document
## Ticket Booking System

This document outlines the architectural design of the Ticket Booking System, which leverages a Microservices architecture to ensure high availability, scalability, and robust fault tolerance.

### 1. High-Level Architecture

The system is composed of several independent microservices communicating via an API Gateway and asynchronous message brokers. 

```mermaid
flowchart TD
    Client[Web / Mobile Client] --> Gateway[API Gateway / NGINX]
    
    subgraph Microservices
        Gateway --> Auth[Auth Service]
        Gateway --> Catalog[Catalog Service]
        Gateway --> Booking[Booking Service]
        Gateway --> Inventory[Inventory Service]
    end
    
    subgraph Databases
        Auth --> DB[(PostgreSQL)]
        Catalog --> DB
        Booking --> DB
        Inventory --> DB
    end
    
    subgraph Message_Broker
        Booking -- Publish --> RMQ((RabbitMQ))
        Inventory -- Subscribe --> RMQ
    end
    
    subgraph Caching_Locking
        Catalog -- Cache --> Redis[(Redis)]
        Inventory -- Distributed Lock --> Redis
    end
```

### 2. The Saga Pattern (Distributed Transactions)

To ensure data consistency across multiple services without distributed locks blocking the entire system, we use the Saga Orchestration Pattern for the checkout and booking flow.

```mermaid
sequenceDiagram
    participant Client
    participant BookingSvc as Booking Service (Orchestrator)
    participant RMQ as RabbitMQ
    participant InvSvc as Inventory Service
    participant DB as PostgreSQL

    Client->>BookingSvc: POST /api/bookings (Create)
    BookingSvc->>DB: Save Booking (Status: PENDING)
    BookingSvc->>RMQ: Publish Event (inventory.reserve)
    
    RMQ->>InvSvc: Consume (inventory.reserve)
    InvSvc->>DB: Lock Seats (FOR UPDATE SKIP LOCKED)
    InvSvc->>RMQ: Publish Event (inventory.reserved)
    
    RMQ->>BookingSvc: Consume (inventory.reserved)
    BookingSvc->>BookingSvc: Process Payment (Simulated)
    
    alt Payment Successful
        BookingSvc->>RMQ: Publish Event (inventory.commit)
        RMQ->>InvSvc: Consume (inventory.commit)
        InvSvc->>DB: Finalize Seat Status
        InvSvc->>RMQ: Publish Event (inventory.committed)
        RMQ->>BookingSvc: Consume (inventory.committed)
        BookingSvc->>DB: Update Booking (Status: CONFIRMED)
        BookingSvc-->>Client: 201 Created (Booking Details)
    else Payment Failed
        BookingSvc->>RMQ: Publish Event (inventory.release)
        RMQ->>InvSvc: Consume (inventory.release)
        InvSvc->>DB: Release Seats
        InvSvc->>RMQ: Publish Event (inventory.released)
        RMQ->>BookingSvc: Consume (inventory.released)
        BookingSvc->>DB: Update Booking (Status: CANCELLED)
        BookingSvc-->>Client: 402 Payment Required
    end
```

### 3. Concurrency & Inventory Locking

Handling hundreds of users attempting to book the exact same seat requires a robust locking strategy to prevent overselling while maintaining performance.

```mermaid
flowchart LR
    Request1[User A: Book Seat 1] --> Lock{Redis Distributed Lock\nSETNX}
    Request2[User B: Book Seat 1] --> Lock
    
    Lock -- Success --> DB[PostgreSQL FOR UPDATE SKIP LOCKED]
    Lock -- Fail (Retry) --> Retry[Wait 100ms and Retry]
    Retry --> Lock
    
    DB -- Seat Available --> Reserve[Reserve Seat]
    DB -- Seat Taken --> Error[Return 409 Conflict]
```
