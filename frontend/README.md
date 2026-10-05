# Ticket Booking System — React + TypeScript Frontend

A modern, high-performance web frontend for the Event Ticket Booking System built with **React 18**, **TypeScript** (Strict Mode), and **Vite**.

---

## 1. Features & Architecture

### Event Catalog (`EventCatalog`, `EventCard`)
- Fetches real-time event listings from the API Gateway (`/api/events`).
- Search by artist, title, venue, or description.
- Category pills (All, Concerts, Festivals, Theater, Sports).
- Displays ticket tiers, minimum pricing, and live availability badge.

### Interactive Venue Seat Map (`VenueSeatMap`, `SeatGrid`, `SeatNode`)
- Visual stage header (**"STAGE / PERFORMANCE AREA"**) with glowing curve.
- Grid seating with row identifiers (e.g. Row A & B for VIP, Rows C–F for General Admission).
- Real-time status indicators:
  - **Available**: Emerald green.
  - **Selected**: Glowing amber/gold.
  - **Booked / Reserved**: Muted slate gray (disabled).
  - **VIP Tier**: Highlighted purple border.
- Sticky live selection summary bar displaying seat count, selected chips, subtotal calculation, and checkout trigger.

### Booking Flow & 409 Conflict Handling (`BookingModal`, `CheckoutForm`, `BookingConfirmation`)
- Attendee name and email validation.
- Submits atomic booking payload to `POST /api/bookings`.
- **409 Conflict Handling**: When seats are concurrently booked by another user, the system detects HTTP 409 (`SEAT_UNAVAILABLE`), displays a conflict alert, refreshes the seat map to update booked states, and removes the conflicting seats from the user's cart without discarding attendee input.
- Digital ticket issuance with unique ticket codes, barcode mockup, and print/download capability.

---

## 2. Directory Structure

```
frontend/
├── package.json          # Dependencies (React 18, Vite, TypeScript)
├── tsconfig.json         # Strict TypeScript compiler options
├── vite.config.ts        # Vite build & API proxy configuration
├── nginx.conf            # Nginx SPA fallback & /health endpoint
├── index.html            # HTML5 entrypoint
├── README.md             # Project documentation
└── src/
    ├── main.tsx          # React DOM entrypoint
    ├── App.tsx           # State orchestrator & conflict handler
    ├── index.css         # Modern dark glassmorphism stylesheet
    ├── vite-env.d.ts     # Vite environment types
    ├── types/
    │   └── index.ts      # Data contracts (Event, Seat, Booking, Error)
    ├── services/
    │   └── api.ts        # API client & seat layout generator
    ├── components/
    │   ├── Icons.tsx               # Standalone SVG icons
    │   ├── Header.tsx              # Navigation & gateway status
    │   ├── EventCatalog.tsx        # Event browsing & search
    │   ├── EventCard.tsx           # Event summary card
    │   ├── VenueSeatMap.tsx        # Stage, legend, & selection bar
    │   ├── SeatGrid.tsx            # Row layout container
    │   ├── SeatNode.tsx            # Individual interactive seat
    │   ├── BookingModal.tsx        # Checkout & confirmation modal
    │   ├── CheckoutForm.tsx        # Attendee inputs & 409 alert
    │   ├── BookingConfirmation.tsx # Confirmed tickets & reference
    │   ├── AlertNotification.tsx   # Toast/alert banners
    │   └── LoadingSpinner.tsx      # Async loading indicator
    └── test/
        └── unit.test.ts  # Layout & 409 conflict test suite
```

---

## 3. Local Development

```bash
# Install dependencies
npm install

# Start Vite dev server on port 3000
npm run dev

# Compile TypeScript and build production bundle
npm run build

# Run unit tests
npm test
```

---

## 4. Production Containerization (Docker + Nginx)

### Multi-Stage Build:
- **Builder Stage**: `node:20-alpine` runs `npm install` and `npm run build` (`tsc && vite build`).
- **Runtime Stage**: `nginx:1.25-alpine` serves `/app/dist` on port 80 (mapped to port 3000 in `docker-compose.yml`).
- **SPA Routing**: `try_files $uri $uri/ /index.html;` ensures client-side routing works smoothly.
- **Health Check**: `GET /health` returns `{ "status": "UP", "service": "frontend" }`.
