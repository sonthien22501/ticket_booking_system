# ADR 001: Use PostgreSQL Row-Level Locking for Concurrency

**Date:** 2026-10-05
**Status:** Accepted

## Context
In an event ticket booking system, hundreds of users may attempt to book the same specific seat at the exact same millisecond. We need a way to ensure that a seat cannot be double-booked (zero overselling) while maintaining high throughput.

## Considered Options
1. **Redis Distributed Locks / Atomic Counters:** Fast, but requires adding another piece of infrastructure (Redis) and handling complex cache-invalidation edge cases.
2. **PostgreSQL Row-Level Locking (`SELECT FOR UPDATE`):** Relies on the database's native ACID compliance to lock the specific seat row during the transaction. 

## Decision
We chose to use **PostgreSQL Row-Level Locking (`SELECT FOR UPDATE`)**.

## Consequences
*   **Pros:** Dramatically simplifies the architecture by removing the strict dependency on Redis for consistency. The database acts as the single source of truth. It successfully passed the 50-thread concurrent contention test.
*   **Cons:** Puts more load on the primary PostgreSQL instance during massive traffic spikes. If performance degrades in production, we may need to reconsider Option 1 as a caching layer.
