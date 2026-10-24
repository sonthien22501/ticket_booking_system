# Milestone 31: Advanced Geolocation Search, Venue Floorplans & Category Facets

## Overview
Implement advanced search and discovery capabilities across the event catalog:
1. Multi-facet search query engine (keyword, city/location, category, minimum/maximum price bounds, sorting by date/price/name).
2. Category and City facet counts calculation for rapid filtering.
3. Interactive venue floorplan metadata (spatial sections: `STAGE`, `FLOOR`, `LOWER_BOWL`, `MEZZANINE`, `BALCONY`) with section visualizer in the seat map.

---

## Architecture & Data Flow

```
[ Frontend: Search & Facets Bar + Venue Floorplan Navigator ]
                             │
                             ▼ :8080 (API Gateway)
              /api/events/search   /api/events/:id/floorplan
                             │
                             ▼ :8081 (Catalog Service)
        [ Search & Facet Engine + Venue Floorplan Builder ]
                             │
                             ▼ :5432 (Postgres / Redis)
            [ events / ticket_tiers / venues DB ]
```

---

## 1. Backend Changes (Catalog Service)
- New file: `backend/catalog-service/search.go`
  - `SearchEventsQuery`: `Q`, `City`, `Category`, `MinPrice`, `MaxPrice`, `SortBy`
  - `SearchFacets`: `Cities` (`map[string]int`), `Categories` (`map[string]int`), `MinPrice`, `MaxPrice`
  - `SearchResponse`: `Events`, `Facets`, `TotalCount`
  - `VenueFloorplan`: Sections (`ID`, `Name`, `TierID`, `Capacity`, `BasePrice`, `RowRange`, `Coordinates`)
  - Endpoints:
    - `GET /api/events/search` (and `/events/search`)
    - `GET /api/events/{id}/floorplan` (and `/events/{id}/floorplan`)
- Register in `backend/catalog-service/main.go` and `backend/api-gateway/nginx.conf`.

---

## 2. Frontend Changes
- Types in `frontend/src/types/index.ts`:
  - `EventSearchFilters`, `SearchFacets`, `VenueSection`, `VenueFloorplan`
- API in `frontend/src/services/api.ts`:
  - `searchEvents(filters: EventSearchFilters): Promise<{ events: EventItem[]; facets: SearchFacets }>`
  - `fetchVenueFloorplan(eventId: string): Promise<VenueFloorplan>`
- Components:
  - `EventCatalog.tsx`: city filter pills, price range presets, sort dropdown, active filter badges.
  - `VenueSeatMap.tsx`: Section Switcher / Floorplan mini-map showing stage orientation and section tier highlights.
- Tests:
  - `backend/catalog-service/search_test.go`
  - `frontend/src/test/faceted_search_floorplan.test.ts`
