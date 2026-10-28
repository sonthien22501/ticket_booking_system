# Milestone 34: Cinema Premiere & Holographic Glow Motion System

**Goal**: Upgrade the TicketVerse dashboard, event cards, interactive seat arena, telemetry studio, and boarding passes with a zero-dependency **Cinema Premiere & Holographic Glow** motion engine (3D perspective tilt, dynamic cursor spotlights, auto-advancing Premiere Hero Showcase, staggered wave seat entrances, live audio equalizer bars, animated KPI count-ups/gauges, and holographic foil boarding passes) while preserving `prefers-reduced-motion` accessibility and 100% existing test compatibility.

**Base SHA**: `33b94286085080cc935db5786db2842567751528`

---

## Architecture & File Map

1. **`frontend/src/utils/motion.ts` (New)**:
   - `calculate3DTilt(rect: DOMRect, clientX: number, clientY: number, maxDeg?: number)`: Pure deterministic math helper returning `{ rotateX, rotateY, spotlightX, spotlightY, glareAngle }` for 3D perspective card & ticket tilt and radial cursor spotlight tracking.
   - `interpolateCountUp(start: number, end: number, progress: number)`: Cubic ease-out (`1 - Math.pow(1 - t, 3)`) numeric interpolation for smooth KPI count-up animations.
   - `calculateStaggerDelay(index: number, baseMs?: number, maxMs?: number)`: Computes deterministic staggered animation delay (`--stagger-delay`) for event cards and seat matrix wave entrances.
   - `calculateRingDashOffset(percent: number, radius: number)`: Computes SVG `strokeDasharray` and `strokeDashoffset` for animated circular occupancy gauges in Organizer Studio.

2. **`frontend/src/components/EventCard.tsx` (Modify)**:
   - Add `index?: number` prop for staggered grid entrance (`--stagger-delay`).
   - Add interactive 3D tilt & radial spotlight cursor tracking via `onMouseMove` and `onMouseLeave` updating CSS custom properties (`--rx`, `--ry`, `--spot-x`, `--spot-y`) with `data-testid="event-card-3d"`.
   - Add animated seat scarcity progress bar (`event-scarcity-bar`) showing real-time remaining seat ratio with shimmer pulse.

3. **`frontend/src/components/EventCatalog.tsx` (Modify)**:
   - Add **Cinema Premiere Hero Showcase** (`premiere-showcase-banner`, `data-testid="premiere-hero-showcase"`) above the search bar featuring:
     - Ambient animated aurora & Dual IMAX projector light beams.
     - Auto-advancing featured event spotlight with interactive indicator pills (`data-testid="premiere-slide-indicator-0"`, etc.), live seat availability pulse, and 1-click "Book Premiere Seats" CTA.
     - Live stats ticker ribbon (`Live Events`, `Atomic Hold Engine <15ms`, `Verified Instant QR`).
   - Pass `index={idx}` to each `<EventCard />` in `.events-grid` for staggered cascade entrance when filtering/sorting.

4. **`frontend/src/components/SeatNode.tsx` & `frontend/src/components/SeatGrid.tsx` (Modify)**:
   - Pass `rowIndex` and `colIndex` to `SeatNode` to set `--seat-stagger` (`calculateStaggerDelay(rowIndex * 4 + colIndex, 15, 450)`) so the theater matrix cascades in a smooth front-to-back wave from the curved screen.
   - Add elastic spring pop & expanding luminous shockwave ring (`seat-ripple-ring`) when a seat is selected.

5. **`frontend/src/components/VenueSeatMap.tsx` (Modify)**:
   - Add animated 4-bar CSS equalizer (`audio-equalizer-bars`, `data-testid="audio-equalizer-bars"`) inside the iTunes preview pill that dances rhythmically when `isPlayingPreview` is active.
   - Upgrade `.selection-summary-footer` with spring-physics badge pulse when seat count changes and animated subtotal transition.

6. **`frontend/src/components/AdminDashboardModal.tsx` (Modify)**:
   - Add smooth `requestAnimationFrame` count-up hook using `interpolateCountUp` for `Total Revenue`, `Tickets Issued`, and `Avg Occupancy`.
   - Add an animated SVG circular ring gauge (`occupancy-ring-gauge`, `data-testid="occupancy-ring-gauge"`) using `calculateRingDashOffset` alongside the occupancy progress bar.

7. **`frontend/src/components/TicketPassModal.tsx` (Modify)**:
   - Add interactive 3D tilt + holographic foil shimmer overlay (`holographic-pass-overlay`) and animated laser scanner line (`qr-scanner-laser`, `data-testid="qr-scanner-laser"`) across the QR code area.

8. **`frontend/src/index.css` (Modify)**:
   - Define Cinema Premiere & Holographic Glow CSS variables (`--ease-spring`, `--ease-out-expo`), `@keyframes` (`cardCascadeIn`, `auroraDrift`, `projectorSweep`, `seatWaveIn`, `seatSelectPop`, `equalizerBounce`, `holographicFoil`, `laserScanSweep`, `shimmerSweep`), 3D perspective card classes, and strict `@media (prefers-reduced-motion: reduce)` overrides.

9. **`frontend/src/test/cinema_premiere_animations.test.ts` (New)**:
   - Comprehensive unit & component test suite verifying 3D tilt math, count-up easing interpolation, SVG gauge offset calculation, stagger delay bounds, component props/rendering across `EventCard`, `EventCatalog`, `SeatNode`, `SeatGrid`, `AdminDashboardModal`, and `TicketPassModal`, and reduced-motion safety.

---

## Review Focus (Top 5 Edge Cases & Failure Modes)
1. **Zero Regression on Existing 16 Test Suites**: All existing `data-testid` attributes, props, and DOM structures in `EventCatalog`, `EventCard`, `SeatNode`, `SeatGrid`, `VenueSeatMap`, `AdminDashboardModal`, and `TicketPassModal` must remain 100% backward-compatible.
2. **Division-by-Zero & Bounds Safety in 3D Tilt**: `calculate3DTilt` must safely handle zero-width/zero-height `DOMRect` (e.g. in headless JSDOM/Vitest environments) without returning `NaN` or `Infinity`.
3. **Stagger Delay Cap**: `calculateStaggerDelay` must clamp maximum delay (e.g. `<= 600ms`) so large catalogs or 100-seat grids never feel sluggish or block user interaction.
4. **Reduced Motion Accessibility (`prefers-reduced-motion: reduce`)**: CSS animations and transitions must cleanly collapse to instant/no-op when the user has reduced motion enabled.
5. **Audio Equalizer & Empty Catalog Edge Cases**: Premiere Hero Showcase in `EventCatalog` must gracefully handle `events.length === 0` without crashing on undefined index `0`.
