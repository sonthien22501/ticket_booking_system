# User Guide
## Ticket Booking System

Welcome to the Ticket Booking System! This guide will help you navigate the system as an end user and provide examples of how to interact with the API directly if you are a developer testing the system.

### 1. Browsing Events
When you access the platform (Frontend at `http://localhost:3000`), the home page will display a catalog of all available events. This catalog is optimized using Redis caching to ensure lightning-fast load times even during traffic spikes.

**Developer API:**
```bash
curl http://localhost:8080/api/events
```

### 2. Making a Booking
To purchase tickets, select an event, choose your ticket tier and quantity (or specific seats), and proceed to checkout.

Behind the scenes, the system uses a **Saga Pattern**:
1. It temporarily "locks" your seats.
2. It attempts to process your payment.
3. If successful, it generates your final Tickets.
4. If the payment fails, it immediately unlocks the seats for other users to purchase.

**Developer API (Simulating a Booking):**
```bash
curl -X POST http://localhost:8080/api/bookings \
  -H "Content-Type: application/json" \
  -d '{
    "eventId": "evt-001",
    "tierId": "tier-vip",
    "quantity": 2,
    "customerEmail": "test@example.com",
    "paymentInfo": { "cardToken": "tok_visa" }
  }'
```

*Note: To simulate a failed payment (which triggers the Saga rollback), change the `cardToken` to `"tok_fail"`.*

### 3. Viewing Your Tickets
Once a booking is confirmed, you will receive a Booking Reference and a list of generated Tickets, each with a unique `ticketCode` that can be scanned at the event venue.

**Developer API:**
```bash
curl http://localhost:8080/api/bookings?email=test@example.com
```
