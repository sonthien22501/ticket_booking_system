# Milestone 33: External API Mash-Up Hub, OpenAPI 3.0 Contracts & Runtime AI Concierge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a production-grade External API Mash-Up Hub (combining internal Catalog & Booking microservices with Open-Meteo Weather, Apple iTunes Artist Audio Previews, and Frankfurter ECB Live Currency FX), formal OpenAPI 3.0 specification & interactive Swagger UI (`/api/openapi.json` & `/api/docs`), a Runtime AI Event Concierge (`POST /api/concierge/advise`), and the complete EnIA Prompting & Orchestration Engineering Diary (`docs/PROMPT_AND_ORCHESTRATION_DIARY.md`).

**Architecture:**
1. **`catalog-service` (`backend/catalog-service/mashup.go` + `openapi.go`)**:
   - Exposes `GET /events/:id/mashup` (and `/api/events/:id/mashup`) orchestrating internal PostgreSQL event/venue data with external **Open-Meteo Forecast API** (`https://api.open-meteo.com/v1/forecast`) and **Apple iTunes Search API** (`https://itunes.apple.com/search`), backed by Redis caching (`mashup:event:<id>`, TTL 30m) and a 2.5s `context.WithTimeout` circuit breaker with deterministic fallback (`X-Mashup-Degraded: true`).
   - Exposes `GET /openapi.json` (and `/api/openapi.json`) and interactive Swagger UI at `GET /docs` (and `/api/docs`).
2. **`booking-service` (`backend/booking-service/currency.go` + `concierge.go`)**:
   - Upgrades `GET /currencies/rates` (and `/api/currencies/rates`) to compose live **Frankfurter ECB FX rates** (`https://api.frankfurter.app/latest?from=USD`) with internal regional VAT/tax rules and automatic fallback (`source: "ecb-live" | "fallback-static"`).
   - Exposes `POST /concierge/advise` (and `/api/concierge/advise`) combining internal event/tier/add-on telemetry + external weather & FX data with a Runtime LLM synthesizer (supports `OPENAI_API_KEY` / `GEMINI_API_KEY` or deterministic rule-based heuristic fallback when no key is set) with strict contract validation.
3. **`api-gateway` (`backend/api-gateway/nginx.conf`) & `frontend` (`VenueSeatMap.tsx`, `api.ts`, `types/index.ts`)**:
   - Routes `/api/openapi.json`, `/api/docs`, `/api/events/:id/mashup`, and `/api/concierge/advise` through Nginx Gateway `:8080`.
   - Renders the **Live Mash-Up Intelligence Bar** (Weather badge, 30s iTunes track preview player, API source indicators) and **AI Event Concierge Drawer** inside `VenueSeatMap.tsx`.

**Tech Stack:** Go 1.22 (`net/http`, `context`, `redis/go-redis/v9`), Nginx Reverse Proxy, OpenAPI 3.0.3, React 18 + TypeScript + Vite + Vitest.

**Spec:** `docs/ARCHITECTURE.md` (Milestone 33 / EnIA Course Rubric)

## Global Constraints
- Zero API keys required for default operation: Open-Meteo, iTunes Search, and Frankfurter ECB are 100% keyless public APIs, and all 3 must degrade gracefully with HTTP 200 and `fallbackUsed: true` when offline or timed out (>2.5s).
- Zero breaking changes to existing endpoints (`GET /api/currencies/rates` must preserve all existing fields `base`, `rates`, `symbols`, `taxRates`, `decimals`, `currencies`, `updatedAt` while adding `source` and `fallbackUsed`).
- Strict contract validation on `POST /api/concierge/advise`: empty `eventId` must return HTTP `400 Bad Request` with structured RFC 7807-style error JSON (`{"code":"VALIDATION_ERROR","error":"eventId is required","fallbackUsed":false}`).
- All Go tests (`go test -timeout 30s ./...`) and Frontend Vitest + TypeScript checks (`npx --no-install vitest run`, `npx --no-install -p typescript tsc --noEmit`) must pass.

## Review Focus
1. **External API Timeout / Offline Resilience**: When Open-Meteo, iTunes, or Frankfurter returns HTTP 500 or times out, `GET /api/events/:id/mashup` and `GET /api/currencies/rates` must still return HTTP 200 with deterministic fallback data and `fallbackUsed: true`.
2. **Contract Validation on Concierge Endpoint**: `POST /api/concierge/advise` with missing `eventId` or malformed JSON must return HTTP 400 with `code: "VALIDATION_ERROR"`.
3. **Backward Compatibility of Currency Rates**: Existing tests in `backend/booking-service/currency_invoice_test.go` must continue to pass without modification.
4. **OpenAPI 3.0 Spec Completeness**: `GET /api/openapi.json` must return valid JSON containing `openapi: "3.0.3"`, `info.title`, and paths for `/api/events`, `/api/events/{id}/mashup`, `/api/bookings`, `/api/currencies/rates`, and `/api/concierge/advise`.
5. **City Coordinate Resolution**: Unknown venue cities in `catalog-service` must resolve to sensible default coordinates (e.g. New York `40.7128, -74.0060`) rather than panicking or failing the mashup call.

---

### Task 1: Catalog Service External Mash-Up (`Open-Meteo` + `iTunes Search`) & OpenAPI 3.0 Spec Server

**Files:**
- Create: `backend/catalog-service/mashup.go`
- Create: `backend/catalog-service/openapi.go`
- Create: `backend/catalog-service/mashup_test.go`
- Modify: `backend/catalog-service/main.go`
- Create: `docs/openapi.yaml`

**Interfaces:**
- Produces in `backend/catalog-service/mashup.go`:
  - `type WeatherForecast struct { TemperatureC float64; TemperatureF float64; Condition string; WeatherCode int; PrecipitationProb int; WindSpeedKmh float64; IsOutdoorFriendly bool; Advisory string; Source string }`
  - `type MediaTrackPreview struct { TrackName string; ArtistName string; AlbumName string; PreviewURL string; ArtworkURL string; Genre string; ReleaseDate string; Source string }`
  - `type EventMashupResponse struct { EventID string; EventTitle string; VenueName string; VenueCity string; Latitude float64; Longitude float64; Weather WeatherForecast; MediaPreview MediaTrackPreview; FallbackUsed bool; Cached bool; FetchedAt string }`
  - `func (s *Server) handleEventMashup(w http.ResponseWriter, r *http.Request, eventID string)`
  - `func resolveCityCoords(location string) (city string, lat float64, lon float64)`
  - `func fetchOpenMeteoWeather(ctx context.Context, client *http.Client, baseURL string, lat, lon float64) (WeatherForecast, bool)`
  - `func fetchITunesPreview(ctx context.Context, client *http.Client, baseURL string, query string) (MediaTrackPreview, bool)`
- Produces in `backend/catalog-service/openapi.go`:
  - `func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request)`
  - `func (s *Server) handleSwaggerDocs(w http.ResponseWriter, r *http.Request)`

- [x] **Step 1: Write failing tests in `backend/catalog-service/mashup_test.go`**
  Test:
  1. `TestResolveCityCoords` (verifies San Francisco, New York, London, Tokyo, and unknown fallback).
  2. `TestFetchOpenMeteoWeather_LiveAndFallback` (uses `httptest.NewServer` for mock Open-Meteo 200 OK JSON and 500 error fallback).
  3. `TestFetchITunesPreview_LiveAndFallback` (uses `httptest.NewServer` for mock iTunes JSON and timeout/error fallback).
  4. `TestOpenAPISpecAndDocsEndpoints` (verifies `GET /api/openapi.json` returns 200 with `"openapi":"3.0.3"` and `GET /api/docs` returns 200 HTML with Swagger UI).

- [x] **Step 2: Run `go test` in `backend/catalog-service` to verify RED failure**
  Run: `cd backend/catalog-service && go test -timeout 30s ./...`

- [x] **Step 3: Implement `backend/catalog-service/mashup.go`, `backend/catalog-service/openapi.go`, `docs/openapi.yaml`, and wire routes in `backend/catalog-service/main.go`**
  - In `handleEventByID` (`backend/catalog-service/main.go`), add suffix check for `/mashup` calling `s.handleEventMashup(w, r, eventID)`.
  - In `main()` (`backend/catalog-service/main.go`), register `/openapi.json`, `/api/openapi.json`, `/docs`, and `/api/docs`.
  - Create `docs/openapi.yaml` documenting the full system + Mash-Up contracts.

- [x] **Step 4: Run `go test` and `gofmt` in `backend/catalog-service` to verify GREEN**
  Run: `cd backend/catalog-service && gofmt -s -w . && go test -timeout 30s ./...`

---

### Task 2: Booking Service Live Frankfurter ECB FX & Runtime AI Event Concierge (`POST /api/concierge/advise`)

**Files:**
- Modify: `backend/booking-service/currency.go`
- Create: `backend/booking-service/concierge.go`
- Create: `backend/booking-service/concierge_mashup_test.go`
- Modify: `backend/booking-service/main.go`
- Modify: `backend/api-gateway/nginx.conf`

**Interfaces:**
- Updates in `backend/booking-service/currency.go`:
  - Add `Source string` (`json:"source"`) and `FallbackUsed bool` (`json:"fallbackUsed"`) to `CurrencyRatesResponse`.
  - Add `fetchFrankfurterRates(ctx context.Context, client *http.Client, baseURL string) (map[string]float64, bool)` with 2.0s timeout and in-memory 15-minute cache so live ECB rates update `currencyRegistry` safely while falling back to static rates if offline.
- Produces in `backend/booking-service/concierge.go`:
  - `type ConciergeAdviceRequest struct { EventID string; EventTitle string; Category string; VenueName string; VenueLocation string; Currency string; BudgetMax float64; WeatherCondition string; RainProbability int; Preferences string }`
  - `type ConciergeAdviceResponse struct { EventID string; Headline string; WeatherAdvisory string; RecommendedTier string; RecommendedAddonName string; RecommendedAddonID string; LocalizedBudgetNote string; InsiderTips []string; ComposedSources []string; Engine string; FallbackUsed bool; GeneratedAt string }`
  - `func (s *Server) handleConciergeAdvise(w http.ResponseWriter, r *http.Request)`

- [x] **Step 1: Write failing tests in `backend/booking-service/concierge_mashup_test.go`**
  Test:
  1. `TestFetchFrankfurterRates_SuccessAndFallback` (mock `httptest.NewServer` returning ECB rates vs HTTP 503 fallback).
  2. `TestConciergeAdvise_ValidationError` (missing `eventId` returns HTTP 400 with `code: "VALIDATION_ERROR"`).
  3. `TestConciergeAdvise_CompositionAndHeuristicEngine` (valid payload composes weather + currency + tier/add-on advice and returns HTTP 200 with `composedSources`, `recommendedTier`, `recommendedAddonId`, `insiderTips`).

- [x] **Step 2: Run `go test` in `backend/booking-service` to verify RED failure**
  Run: `cd backend/booking-service && go test -timeout 30s ./...`

- [x] **Step 3: Implement `currency.go` upgrade, `concierge.go`, route registration in `backend/booking-service/main.go`, and Gateway routes in `backend/api-gateway/nginx.conf`**
  - Register `/concierge/advise` and `/api/concierge/advise` in `backend/booking-service/main.go`.
  - Add `/api/openapi.json`, `/api/docs`, `/api/events/:id/mashup` (covered by `/api/events/`), and `/api/concierge/advise` in `backend/api-gateway/nginx.conf`.

- [x] **Step 4: Run `go test` and `gofmt` in `backend/booking-service` to verify GREEN**
  Run: `cd backend/booking-service && gofmt -s -w . && go test -timeout 30s ./...`

---

### Task 3: React Frontend Mash-Up Intelligence Bar & AI Event Concierge UI + Vitest Suite

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/services/api.ts`
- Modify: `frontend/src/components/VenueSeatMap.tsx`
- Create: `frontend/src/test/mashup_concierge.test.ts`

**Interfaces:**
- Produces in `frontend/src/types/index.ts`:
  - `WeatherForecast`, `MediaTrackPreview`, `EventMashupData`, `ConciergeAdviceRequest`, `ConciergeAdviceResponse`
- Produces in `frontend/src/services/api.ts`:
  - `fetchEventMashup(eventId: string): Promise<EventMashupData>`
  - `requestConciergeAdvice(req: ConciergeAdviceRequest): Promise<ConciergeAdviceResponse>`
  - `fetchOpenApiSpec(): Promise<Record<string, unknown>>`

- [x] **Step 1: Write failing Vitest unit tests in `frontend/src/test/mashup_concierge.test.ts`**
  Test `fetchEventMashup`, `requestConciergeAdvice`, `fetchOpenApiSpec`, and fallback resilience when API returns degraded/offline state.

- [x] **Step 2: Run Vitest to verify RED failure**
  Run: `cd frontend && npx --no-install vitest run src/test/mashup_concierge.test.ts`

- [x] **Step 3: Implement types, API client methods, and UI in `VenueSeatMap.tsx`**
  - Add the **Live Mash-Up Intelligence Bar** in `VenueSeatMap.tsx` displaying Open-Meteo venue weather (`temp`, `condition`, `precipitationProb`), iTunes 30-second audio track preview button/player, and a toggleable **AI Event Concierge Advisor** panel (`data-testid="ai-concierge-panel"`).

- [x] **Step 4: Run full Frontend test suite & TypeScript check to verify GREEN**
  Run: `cd frontend && npx --no-install -p typescript tsc --noEmit --pretty false && npx --no-install vitest run`

---

### Task 4: Prompting, Skills & Subagent Orchestration Diary (`docs/PROMPT_AND_ORCHESTRATION_DIARY.md`) & Architecture Update

**Files:**
- Create: `docs/PROMPT_AND_ORCHESTRATION_DIARY.md`
- Modify: `docs/ARCHITECTURE.md`
- Modify: `README.md`

- [x] **Step 1: Create `docs/PROMPT_AND_ORCHESTRATION_DIARY.md`**
  Include:
  1. Executive Summary of the EnIA "From Coding to Composing" paradigm.
  2. Evolution from Ad-Hoc Prompts to Domain & Workflow Skills (`.agent/skills/*` and global orchestration skills).
  3. Multi-Engine Lead & 4-Tier Worker Subagent Hierarchy (Argon/Opus Lead Brain vs. Flash/OpenCode/River/Copilot Workers) and the 4 Token Economy Laws (`docs/ARCHITECTURE.md` cache, Zero-Copy File Pointers, Silent-On-Success Test Wrappers, Signature-First Inspection).
  4. Chronological Prompting, Composition & Architectural Rulings Log (Milestones M1–M33).
  5. Runtime Mash-Up & Contract Validation Architecture Diagram (Mermaid).

- [x] **Step 2: Update `docs/ARCHITECTURE.md` and `README.md` with Milestone 33 (F33) and link to `docs/PROMPT_AND_ORCHESTRATION_DIARY.md` and `docs/openapi.yaml`**

- [x] **Step 3: Commit all changes cleanly on feature branch `feature/milestone-33-mashup-openapi-concierge`**
