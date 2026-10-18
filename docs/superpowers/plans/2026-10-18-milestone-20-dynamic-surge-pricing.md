# Milestone 20: Dynamic Surge Pricing & Service Isolation Fault Tolerance (R27, R33) Implementation Plan

## Goal
Implement dynamic surge pricing for high-demand ticket sales (**Requirement R27**), robust service isolation and fault tolerance with graceful degradation (**Requirement R33**), and achieve complete 100% completion (33/33 requirements) of the distributed ticket booking system specification.

---

## Technical Design & Scope
1. **Dynamic Surge Pricing Engine (`backend/catalog-service/pricing.go`)**:
   - `CalculateSurgePricing()`: Calculates dynamic ticket pricing reflecting inventory scarcity tiers and real-time booking velocity:
     - Scarcity Tiers: $S \ge 95\%$ (`CRITICAL_SCARCITY`: $1.50\times$), $S \ge 85\%$ (`HIGH_DEMAND`: $1.30\times$), $S \ge 70\%$ (`MODERATE_DEMAND`: $1.15\times$), $S < 70\%$ (`STANDARD`: $1.00\times$).
     - Velocity Boost: Booking rate $> 5.0$ reservations/minute applies $+0.15\times$ boost.
     - Capping: Multipliers capped at $2.00\times$.
   - `CalculateSurgePricingFallback()`: Fallback implementation when external dependencies are unavailable, returning base pricing with `degraded: true` flag.
2. **REST API & Gateway Routing**:
   - `GET /api/events/{id}/pricing`: Returns real-time pricing telemetry including `basePrice`, `currentPrice`, `surgeMultiplier`, `surgeFactor`, `capacity`, `available`, `soldPercentage`, `velocityPerMinute`, and `calculatedAt`.
   - Nginx API Gateway routes transparently to `catalog-service` on port 8081.
3. **OpenAPI 3.0 Contract Updates (`openapi.yaml` & `openapi.json`)**:
   - Documented `/api/events/{id}/pricing` schema and responses.
4. **Service Isolation & Fault Tolerance (R33)**:
   - Failure of downstream databases, redis, or services does not produce unhandled 500 panics.
   - Bounded client timeouts and circuit breaking prevent connection pool exhaustion.
5. **Specification Completion**:
   - Marked R2, R4, R27, and R33 as completed in `REQUIREMENTS.md`, achieving 33/33 (100%) requirement completion.

---

## Verification
- Unit test suite: `backend/catalog-service/pricing_test.go` (100% pass).
- E2E test suite: `e2e/surge_pricing_fault_tolerance_test.go`.
- OpenAPI contract test: `e2e/swagger_test.go` (100% pass).
