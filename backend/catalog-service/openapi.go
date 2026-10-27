package main

import (
	"encoding/json"
	"net/http"
)

// openAPISpecData defines the authoritative OpenAPI 3.0.3 specification for the system
var openAPISpecData = map[string]interface{}{
	"openapi": "3.0.3",
	"info": map[string]interface{}{
		"title":       "Ticket Booking System API",
		"version":     "1.0.0",
		"description": "Unified OpenAPI 3.0 contract for Event Catalog, Venue Floorplan, Mash-Up Hub, Bookings, Currency Rates, and AI Concierge.",
		"contact": map[string]interface{}{
			"name":  "EnIA System Team",
			"email": "architecture@ticketbooking.local",
		},
	},
	"servers": []map[string]interface{}{
		{
			"url":         "/",
			"description": "Local API Gateway / Nginx Reverse Proxy",
		},
	},
	"paths": map[string]interface{}{
		"/api/events": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "List catalog events",
				"description": "Retrieve list of all active events with ticket tiers and venue info.",
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Successful retrieval of events",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"$ref": "#/components/schemas/Event",
									},
								},
							},
						},
					},
				},
			},
			"post": map[string]interface{}{
				"summary":     "Create new event",
				"description": "Admin endpoint to publish a new event.",
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"$ref": "#/components/schemas/CreateEventRequest",
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"201": map[string]interface{}{
						"description": "Event created successfully",
					},
				},
			},
		},
		"/api/events/{id}/mashup": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "Get event mashup intelligence",
				"description": "Orchestrates internal catalog data with live Open-Meteo weather forecast and Apple iTunes 30s track audio preview.",
				"parameters": []map[string]interface{}{
					{
						"name":        "id",
						"in":          "path",
						"required":    true,
						"description": "Event identifier",
						"schema": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Composed external mashup data",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/EventMashupResponse",
								},
							},
						},
					},
					"404": map[string]interface{}{
						"description": "Event not found",
					},
				},
			},
		},
		"/api/bookings": map[string]interface{}{
			"post": map[string]interface{}{
				"summary":     "Create ticket booking",
				"description": "Reserve tickets and initiate payment and ticket issuance.",
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"$ref": "#/components/schemas/BookingRequest",
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Booking confirmed",
					},
				},
			},
		},
		"/api/currencies/rates": map[string]interface{}{
			"get": map[string]interface{}{
				"summary":     "Get currency exchange rates & regional tax matrix",
				"description": "Composes live Frankfurter ECB foreign exchange rates with internal tax regulations and deterministic fallback.",
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Currency rates and tax rules",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/CurrencyRatesResponse",
								},
							},
						},
					},
				},
			},
		},
		"/api/concierge/advise": map[string]interface{}{
			"post": map[string]interface{}{
				"summary":     "Generate runtime AI event concierge advice",
				"description": "Synthesizes event telemetry, venue weather, live FX rates, and attendee preferences into actionable concierge recommendations.",
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"$ref": "#/components/schemas/ConciergeAdviceRequest",
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "AI concierge recommendation",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/ConciergeAdviceResponse",
								},
							},
						},
					},
					"400": map[string]interface{}{
						"description": "Validation error",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"$ref": "#/components/schemas/ErrorResponse",
								},
							},
						},
					},
				},
			},
		},
	},
	"components": map[string]interface{}{
		"schemas": map[string]interface{}{
			"Event": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"id":            map[string]interface{}{"type": "string"},
					"title":         map[string]interface{}{"type": "string"},
					"description":   map[string]interface{}{"type": "string"},
					"category":      map[string]interface{}{"type": "string"},
					"venueName":     map[string]interface{}{"type": "string"},
					"venueLocation": map[string]interface{}{"type": "string"},
					"startTime":     map[string]interface{}{"type": "string", "format": "date-time"},
					"endTime":       map[string]interface{}{"type": "string", "format": "date-time"},
					"status":        map[string]interface{}{"type": "string"},
					"imageUrl":      map[string]interface{}{"type": "string"},
				},
			},
			"CreateEventRequest": map[string]interface{}{
				"type":     "object",
				"required": []string{"title", "category", "venueName", "venueLocation", "startTime"},
				"properties": map[string]interface{}{
					"title":         map[string]interface{}{"type": "string"},
					"description":   map[string]interface{}{"type": "string"},
					"category":      map[string]interface{}{"type": "string"},
					"venueName":     map[string]interface{}{"type": "string"},
					"venueLocation": map[string]interface{}{"type": "string"},
					"startTime":     map[string]interface{}{"type": "string"},
					"endTime":       map[string]interface{}{"type": "string"},
				},
			},
			"WeatherForecast": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"temperatureC":      map[string]interface{}{"type": "number"},
					"temperatureF":      map[string]interface{}{"type": "number"},
					"condition":         map[string]interface{}{"type": "string"},
					"weatherCode":       map[string]interface{}{"type": "integer"},
					"precipitationProb": map[string]interface{}{"type": "integer"},
					"windSpeedKmh":      map[string]interface{}{"type": "number"},
					"isOutdoorFriendly": map[string]interface{}{"type": "boolean"},
					"advisory":          map[string]interface{}{"type": "string"},
					"source":            map[string]interface{}{"type": "string"},
				},
			},
			"MediaTrackPreview": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"trackName":   map[string]interface{}{"type": "string"},
					"artistName":  map[string]interface{}{"type": "string"},
					"albumName":   map[string]interface{}{"type": "string"},
					"previewUrl":  map[string]interface{}{"type": "string"},
					"artworkUrl":  map[string]interface{}{"type": "string"},
					"genre":       map[string]interface{}{"type": "string"},
					"releaseDate": map[string]interface{}{"type": "string"},
					"source":      map[string]interface{}{"type": "string"},
				},
			},
			"EventMashupResponse": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"eventId":      map[string]interface{}{"type": "string"},
					"eventTitle":   map[string]interface{}{"type": "string"},
					"venueName":    map[string]interface{}{"type": "string"},
					"venueCity":    map[string]interface{}{"type": "string"},
					"latitude":     map[string]interface{}{"type": "number"},
					"longitude":    map[string]interface{}{"type": "number"},
					"weather":      map[string]interface{}{"$ref": "#/components/schemas/WeatherForecast"},
					"mediaPreview": map[string]interface{}{"$ref": "#/components/schemas/MediaTrackPreview"},
					"fallbackUsed": map[string]interface{}{"type": "boolean"},
					"cached":       map[string]interface{}{"type": "boolean"},
					"fetchedAt":    map[string]interface{}{"type": "string"},
				},
			},
			"BookingRequest": map[string]interface{}{
				"type":     "object",
				"required": []string{"eventId", "tierId", "quantity", "customerEmail"},
				"properties": map[string]interface{}{
					"eventId":       map[string]interface{}{"type": "string"},
					"tierId":        map[string]interface{}{"type": "string"},
					"quantity":      map[string]interface{}{"type": "integer"},
					"customerEmail": map[string]interface{}{"type": "string"},
				},
			},
			"CurrencyRatesResponse": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"base":         map[string]interface{}{"type": "string"},
					"rates":        map[string]interface{}{"type": "object"},
					"symbols":      map[string]interface{}{"type": "object"},
					"taxRates":     map[string]interface{}{"type": "object"},
					"decimals":     map[string]interface{}{"type": "object"},
					"currencies":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"updatedAt":    map[string]interface{}{"type": "string"},
					"source":       map[string]interface{}{"type": "string"},
					"fallbackUsed": map[string]interface{}{"type": "boolean"},
				},
			},
			"ConciergeAdviceRequest": map[string]interface{}{
				"type":     "object",
				"required": []string{"eventId"},
				"properties": map[string]interface{}{
					"eventId":          map[string]interface{}{"type": "string"},
					"eventTitle":       map[string]interface{}{"type": "string"},
					"category":         map[string]interface{}{"type": "string"},
					"venueName":        map[string]interface{}{"type": "string"},
					"venueLocation":    map[string]interface{}{"type": "string"},
					"currency":         map[string]interface{}{"type": "string"},
					"budgetMax":        map[string]interface{}{"type": "number"},
					"weatherCondition": map[string]interface{}{"type": "string"},
					"rainProbability":  map[string]interface{}{"type": "integer"},
					"preferences":      map[string]interface{}{"type": "string"},
				},
			},
			"ConciergeAdviceResponse": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"eventId":              map[string]interface{}{"type": "string"},
					"headline":             map[string]interface{}{"type": "string"},
					"weatherAdvisory":      map[string]interface{}{"type": "string"},
					"recommendedTier":      map[string]interface{}{"type": "string"},
					"recommendedAddonName": map[string]interface{}{"type": "string"},
					"recommendedAddonId":   map[string]interface{}{"type": "string"},
					"localizedBudgetNote":  map[string]interface{}{"type": "string"},
					"insiderTips":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"composedSources":      map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"engine":               map[string]interface{}{"type": "string"},
					"fallbackUsed":         map[string]interface{}{"type": "boolean"},
					"generatedAt":          map[string]interface{}{"type": "string"},
				},
			},
			"ErrorResponse": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"code":         map[string]interface{}{"type": "string"},
					"error":        map[string]interface{}{"type": "string"},
					"fallbackUsed": map[string]interface{}{"type": "boolean"},
				},
			},
		},
	},
}

// handleOpenAPISpec serves GET /openapi.json and GET /api/openapi.json
func (s *Server) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(openAPISpecData)
}

// handleSwaggerDocs serves GET /docs and GET /api/docs
func (s *Server) handleSwaggerDocs(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Ticket Booking System - OpenAPI 3.0 Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; background-color: #0f172a; color: #f8fafc; font-family: sans-serif; }
    .topbar { display: none; }
    .swagger-ui { filter: invert(88%) hue-rotate(180deg); }
    .swagger-ui .wrapper { max-width: 1200px; padding: 20px; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function() {
      SwaggerUIBundle({
        url: "/api/openapi.json",
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>`
	w.Write([]byte(html))
}
