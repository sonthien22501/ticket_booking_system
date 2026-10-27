# Plan: Tier 5 Extended Feature & External API Mash-Up Verification Suite (M25–M33)

## Context & Goal
Expand the automated E2E verification harness (`scripts/verify_e2e.py`), dataset loader (`scripts/seed_dataset.sh`), and dataset documentation (`docs/TEST_DATASET_GUIDE.md`) so that a single command automatically verifies **100% of all milestones including M25–M33** (Promotional Vouchers, Multi-Currency & PDF Invoicing, Waitlist & Ticket Transfer, In-App Notifications & iCalendar `.ics`, VIP Add-ons, Verified Buyer Reviews, Multi-Facet Geolocation Search & Floorplans, Async Multi-Channel Notification Worker, External API Mash-Up Hub, OpenAPI 3.0.3 Contract & Swagger UI, and Runtime AI Event Concierge).

---

## Step 1: Extend `scripts/verify_e2e.py` with `run_tier_5_extended_features(ctx: TestContext)`
- **Files**: `scripts/verify_e2e.py`
- **Changes**:
  1. Add `"5"` to `--tier` CLI choices (`choices=["all", "1", "2", "3", "4", "5"]`).
  2. Implement `run_tier_5_extended_features(ctx: TestContext)` covering:
     - **5.1 Promotional Voucher Validation (M25)**: `POST /api/vouchers/validate` with `{"code": "EARLYBIRD20", "subtotal": 100}` -> assert HTTP 200, `valid: True`, `discountAmount == 20`.
     - **5.2 Multi-Currency Live FX Rates & PDF Tax Invoice (M26 / M33)**:
       - `GET /api/currencies/rates` -> assert HTTP 200 and presence of `USD`, `EUR`, `GBP`, `JPY`.
       - `GET /api/bookings/bkg-demo-001/invoice` -> assert HTTP 200 and valid invoice JSON.
     - **5.3 Waitlist Queue & Calendar `.ics` Export (M27 / M28)**:
       - `GET /api/waitlist?eventId=evt-106` -> assert HTTP 200 and non-empty waitlist entries.
       - `GET /api/bookings/bkg-demo-001/calendar.ics` -> assert HTTP 200 and `"BEGIN:VCALENDAR"` in response body.
     - **5.4 VIP Add-Ons & Verified Buyer Reviews (M29 / M30)**:
       - `GET /api/events/evt-101/addons` -> assert HTTP 200 and list of add-ons (`len >= 1`).
       - `GET /api/events/evt-101/reviews` -> assert HTTP 200 and `averageRating` / `reviews` present.
     - **5.5 Geolocation Multi-Facet Search & Venue Floorplans (M31)**:
       - `GET /api/events/search?city=Berlin` -> assert HTTP 200 and matching events returned.
       - `GET /api/events/evt-101/floorplan` -> assert HTTP 200 and `sections` present.
     - **5.6 External API Mash-Up Hub — Open-Meteo Weather & Apple iTunes Preview (M33)**:
       - `GET /api/events/evt-101/mashup` -> assert HTTP 200, `weather` object with `temperatureCelsius`, `condition`, and `source` in `("open-meteo-live", "deterministic-fallback")`, plus `mediaPreview` object with `trackName`, `previewUrl`, and `source` in `("itunes-live", "curated-fallback")`.
       - Also test `GET /api/events/evt-105/mashup` (Tokyo Cyberpunk) -> assert HTTP 200 and valid weather/media preview.
     - **5.7 OpenAPI 3.0.3 Contract & Interactive Swagger Docs (M33)**:
       - `GET /api/openapi.json` -> assert HTTP 200, `openapi` starting with `"3.0"`, and `/api/events/{id}/mashup` & `/api/concierge/advise` in `paths`.
       - `GET /api/docs` -> assert HTTP 200 and `"swagger-ui"` in HTML body.
     - **5.8 Runtime AI Event Concierge Synthesis (M33)**:
       - `POST /api/concierge/advise` with `{"prompt": "I want an electric concert experience in Berlin", "budget": 180, "currency": "EUR", "EventID": "evt-101"}` -> assert HTTP 200, `recommendedEventId`, `recommendedTier`, `convertedPrice`, `currency == "EUR"`, `weatherAdvisory`, and `suggestedVoucher`.
  3. Wire `if args.tier in ("all", "5"): run_tier_5_extended_features(ctx)` into `main()`.

---

## Step 2: Update `scripts/seed_dataset.sh` Health & Endpoint Checks
- **Files**: `scripts/seed_dataset.sh`
- **Changes**:
  - Add database count for `Notification Deliveries: ` (`SELECT 'Deliveries:      ' || count(*) FROM notification_deliveries`).
  - Add Gateway endpoint verification checks in `seed_dataset.sh` for:
    - `External API Mash-Up (Berlin)` (`/api/events/evt-101/mashup`)
    - `External API Mash-Up (Tokyo)` (`/api/events/evt-105/mashup`)
    - `OpenAPI 3.0 Contract` (`/api/openapi.json`)
    - `Interactive Swagger UI` (`/api/docs`)
    - `Geo Search & Facets` (`/api/events/search?city=Berlin`)
    - `Venue Floorplan` (`/api/events/evt-101/floorplan`)
    - `VIP Add-ons` (`/api/events/evt-101/addons`)
    - `Verified Buyer Reviews` (`/api/events/evt-101/reviews`)
    - `Notification Deliveries` (`/api/notifications/deliveries`)
  - Also test `POST /api/concierge/advise` in `seed_dataset.sh` so running `./scripts/seed_dataset.sh` immediately confirms the AI Concierge is live.

---

## Step 3: Update `docs/TEST_DATASET_GUIDE.md` with M29–M33 & External API Runbook
- **Files**: `docs/TEST_DATASET_GUIDE.md`
- **Changes**:
  - Add Section 9: **External API Mash-Up Hub, OpenAPI 3.0 & Runtime AI Concierge (M33)** with copy-pasteable `curl` commands for `/api/events/evt-101/mashup`, `/api/events/evt-105/mashup`, `/api/concierge/advise`, `/api/openapi.json`, and `/api/docs`.
  - Add Section 10: **VIP Add-Ons, Verified Reviews, Floorplans & Async Notification Worker (M29–M32)** commands.
  - Update the **Automated Regression & Concurrency Verification** section to reflect all 5 tiers (19 total E2E assertions).

---

## Step 4: Verification (RED -> GREEN)
1. Run `./scripts/seed_dataset.sh` and verify all database counts and 15+ Gateway endpoint checks pass.
2. Run `python3 scripts/verify_e2e.py --tier 5` and `python3 scripts/verify_e2e.py` (all tiers 1–5) using the silent-on-success wrapper and verify 100% PASS.
