package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type WeatherForecast struct {
	TemperatureC      float64 `json:"temperatureC"`
	TemperatureF      float64 `json:"temperatureF"`
	Condition         string  `json:"condition"`
	WeatherCode       int     `json:"weatherCode"`
	PrecipitationProb int     `json:"precipitationProb"`
	WindSpeedKmh      float64 `json:"windSpeedKmh"`
	IsOutdoorFriendly bool    `json:"isOutdoorFriendly"`
	Advisory          string  `json:"advisory"`
	Source            string  `json:"source"`
}

type MediaTrackPreview struct {
	TrackName   string `json:"trackName"`
	ArtistName  string `json:"artistName"`
	AlbumName   string `json:"albumName"`
	PreviewURL  string `json:"previewUrl"`
	ArtworkURL  string `json:"artworkUrl"`
	Genre       string `json:"genre"`
	ReleaseDate string `json:"releaseDate"`
	Source      string `json:"source"`
}

type EventMashupResponse struct {
	EventID      string            `json:"eventId"`
	EventTitle   string            `json:"eventTitle"`
	VenueName    string            `json:"venueName"`
	VenueCity    string            `json:"venueCity"`
	Latitude     float64           `json:"latitude"`
	Longitude    float64           `json:"longitude"`
	Weather      WeatherForecast   `json:"weather"`
	MediaPreview MediaTrackPreview `json:"mediaPreview"`
	FallbackUsed bool              `json:"fallbackUsed"`
	Cached       bool              `json:"cached"`
	FetchedAt    string            `json:"fetchedAt"`
}

// resolveCityCoords resolves known city coordinates or provides sensible defaults
func resolveCityCoords(location string) (city string, lat float64, lon float64) {
	locLower := strings.ToLower(strings.TrimSpace(location))
	rawCity := extractCity(location)
	if rawCity == "General" || rawCity == "" {
		rawCity = strings.TrimSpace(location)
	}

	switch {
	case strings.Contains(locLower, "san francisco"):
		return "San Francisco", 37.7749, -122.4194
	case strings.Contains(locLower, "new york"):
		return "New York", 40.7128, -74.0060
	case strings.Contains(locLower, "london"):
		return "London", 51.5074, -0.1278
	case strings.Contains(locLower, "tokyo"):
		return "Tokyo", 35.6762, 139.6503
	case strings.Contains(locLower, "berlin"):
		return "Berlin", 52.5200, 13.4050
	case strings.Contains(locLower, "paris"):
		return "Paris", 48.8566, 2.3522
	case strings.Contains(locLower, "austin"):
		return "Austin", 30.2672, -97.7431
	case strings.Contains(locLower, "los angeles"):
		return "Los Angeles", 34.0522, -118.2437
	case strings.Contains(locLower, "chicago"):
		return "Chicago", 41.8781, -87.6298
	case strings.Contains(locLower, "seattle"):
		return "Seattle", 47.6062, -122.3321
	case strings.Contains(locLower, "sydney"):
		return "Sydney", -33.8688, 151.2093
	default:
		c := rawCity
		if strings.TrimSpace(c) == "" {
			c = "New York"
		}
		return c, 40.7128, -74.0060
	}
}

// fetchOpenMeteoWeather queries Open-Meteo public forecast API with fallback
func fetchOpenMeteoWeather(ctx context.Context, client *http.Client, baseURL string, lat, lon float64) (WeatherForecast, bool) {
	if client == nil {
		client = &http.Client{Timeout: 2500 * time.Millisecond}
	}
	if baseURL == "" {
		baseURL = os.Getenv("OPEN_METEO_BASE_URL")
		if baseURL == "" {
			baseURL = "https://api.open-meteo.com/v1/forecast"
		}
	}

	separator := "?"
	if strings.Contains(baseURL, "?") {
		separator = "&"
	}
	reqURL := fmt.Sprintf("%s%slatitude=%.4f&longitude=%.4f&current=temperature_2m,weather_code,wind_speed_10m&hourly=precipitation_probability&timezone=auto",
		baseURL, separator, lat, lon)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fallbackWeather(), true
	}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return fallbackWeather(), true
	}
	defer resp.Body.Close()

	var payload struct {
		Current struct {
			Temperature2m float64 `json:"temperature_2m"`
			WeatherCode   int     `json:"weather_code"`
			WindSpeed10m  float64 `json:"wind_speed_10m"`
		} `json:"current"`
		CurrentWeather struct {
			Temperature float64 `json:"temperature"`
			WeatherCode int     `json:"weathercode"`
			WindSpeed   float64 `json:"windspeed"`
		} `json:"current_weather"`
		Hourly struct {
			PrecipitationProbability []int `json:"precipitation_probability"`
		} `json:"hourly"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return fallbackWeather(), true
	}

	tempC := payload.Current.Temperature2m
	code := payload.Current.WeatherCode
	windKmh := payload.Current.WindSpeed10m

	if tempC == 0 && payload.CurrentWeather.Temperature != 0 {
		tempC = payload.CurrentWeather.Temperature
		code = payload.CurrentWeather.WeatherCode
		windKmh = payload.CurrentWeather.WindSpeed
	}

	precipProb := 10
	if len(payload.Hourly.PrecipitationProbability) > 0 {
		precipProb = payload.Hourly.PrecipitationProbability[0]
	}

	condition, isOutdoorFriendly, advisory := decodeWeatherCode(code, precipProb)
	tempF := (tempC * 9.0 / 5.0) + 32.0

	return WeatherForecast{
		TemperatureC:      tempC,
		TemperatureF:      tempF,
		Condition:         condition,
		WeatherCode:       code,
		PrecipitationProb: precipProb,
		WindSpeedKmh:      windKmh,
		IsOutdoorFriendly: isOutdoorFriendly,
		Advisory:          advisory,
		Source:            "open-meteo",
	}, false
}

func decodeWeatherCode(code, precipProb int) (condition string, isOutdoorFriendly bool, advisory string) {
	switch {
	case code == 0:
		return "Clear Skies", true, "Ideal clear skies for live events and outdoor seating."
	case code >= 1 && code <= 3:
		return "Partly Cloudy", true, "Comfortable mild conditions. Great for indoor & outdoor activities."
	case code == 45 || code == 48:
		return "Foggy", true, "Cooler mist and fog. Allow extra travel time to the venue."
	case code >= 51 && code <= 55:
		return "Light Drizzle", false, "Light drizzle expected. Light rain jacket recommended."
	case code >= 61 && code <= 65:
		return "Rainy", false, "Rain in the area. Covered seating or an umbrella is advised."
	case code >= 71 && code <= 77:
		return "Snow / Flurries", false, "Chilly winter conditions. Dress warmly for entry lines."
	case code >= 80 && code <= 82:
		return "Rain Showers", false, "Scattered rain showers expected near showtime."
	case code >= 95:
		return "Thunderstorm Alert", false, "Severe weather alert: Thunderstorms forecasted. Plan arrival carefully."
	default:
		if precipProb > 50 {
			return "Overcast / Rain Threat", false, "High chance of precipitation around venue."
		}
		return "Fair / Mild", true, "Pleasant weather forecast for attending live events."
	}
}

func fallbackWeather() WeatherForecast {
	return WeatherForecast{
		TemperatureC:      21.0,
		TemperatureF:      69.8,
		Condition:         "Partly Cloudy",
		WeatherCode:       1,
		PrecipitationProb: 15,
		WindSpeedKmh:      12.0,
		IsOutdoorFriendly: true,
		Advisory:          "Mild weather forecast. Ideal for attending live events.",
		Source:            "fallback-static",
	}
}

// fetchITunesPreview queries Apple iTunes Search API for 30s track preview
func fetchITunesPreview(ctx context.Context, client *http.Client, baseURL string, query string) (MediaTrackPreview, bool) {
	if client == nil {
		client = &http.Client{Timeout: 2500 * time.Millisecond}
	}
	if baseURL == "" {
		baseURL = os.Getenv("ITUNES_BASE_URL")
		if baseURL == "" {
			baseURL = "https://itunes.apple.com/search"
		}
	}

	separator := "?"
	if strings.Contains(baseURL, "?") {
		separator = "&"
	}
	cleanQuery := strings.TrimSpace(query)
	cleanQuery = strings.ReplaceAll(cleanQuery, " 2026", "")
	cleanQuery = strings.ReplaceAll(cleanQuery, " 2025", "")

	reqURL := fmt.Sprintf("%s%sterm=%s&entity=song&limit=1", baseURL, separator, url.QueryEscape(cleanQuery))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fallbackMediaTrack(query), true
	}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
		return fallbackMediaTrack(query), true
	}
	defer resp.Body.Close()

	var payload struct {
		ResultCount int `json:"resultCount"`
		Results     []struct {
			TrackName        string `json:"trackName"`
			ArtistName       string `json:"artistName"`
			CollectionName   string `json:"collectionName"`
			PreviewURL       string `json:"previewUrl"`
			ArtworkURL100    string `json:"artworkUrl100"`
			PrimaryGenreName string `json:"primaryGenreName"`
			ReleaseDate      string `json:"releaseDate"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil || payload.ResultCount == 0 || len(payload.Results) == 0 {
		return fallbackMediaTrack(query), true
	}

	res := payload.Results[0]
	if res.TrackName == "" || res.PreviewURL == "" {
		return fallbackMediaTrack(query), true
	}

	return MediaTrackPreview{
		TrackName:   res.TrackName,
		ArtistName:  res.ArtistName,
		AlbumName:   res.CollectionName,
		PreviewURL:  res.PreviewURL,
		ArtworkURL:  res.ArtworkURL100,
		Genre:       res.PrimaryGenreName,
		ReleaseDate: res.ReleaseDate,
		Source:      "itunes",
	}, false
}

func fallbackMediaTrack(query string) MediaTrackPreview {
	artist := query
	if artist == "" {
		artist = "Live Experience Audio"
	}
	return MediaTrackPreview{
		TrackName:   "Official Event Theme & Soundstage",
		ArtistName:  artist,
		AlbumName:   "Live Tour Acoustic Experience",
		PreviewURL:  "https://www.soundhelix.com/examples/mp3/SoundHelix-Song-1.mp3",
		ArtworkURL:  "https://images.unsplash.com/photo-1514525253161-7a46d19cd819?w=300",
		Genre:       "Live Sound",
		ReleaseDate: "2026",
		Source:      "fallback-static",
	}
}

// fetchEventByID retrieves event from DB or mock data
func (s *Server) fetchEventByID(ctx context.Context, eventID string) (*Event, error) {
	if s.db != nil {
		var e Event
		var desc, img sql.NullString
		err := s.db.QueryRowContext(ctx,
			`SELECT id, title, description, category, venue_name, venue_location, start_time, end_time, status, image_url, created_at, updated_at FROM events WHERE id = $1`,
			eventID,
		).Scan(
			&e.ID, &e.Title, &desc, &e.Category,
			&e.VenueName, &e.VenueLocation,
			&e.StartTime, &e.EndTime,
			&e.Status, &img,
			&e.CreatedAt, &e.UpdatedAt,
		)
		if err == nil {
			if desc.Valid {
				e.Description = desc.String
			}
			if img.Valid {
				e.ImageURL = img.String
			}
			e.EventID = e.ID
			return &e, nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	// Fallback to mock events
	mockEvents := s.fetchAllEventsFromDBOrMock(ctx)
	for _, me := range mockEvents {
		if me.ID == eventID || me.EventID == eventID {
			return &me, nil
		}
	}

	if strings.HasPrefix(eventID, "evt-") {
		return &Event{
			ID:            eventID,
			EventID:       eventID,
			Title:         "Live Showcase " + eventID,
			Category:      "CONCERT",
			VenueName:     "Civic Center Arena",
			VenueLocation: "San Francisco, CA",
			StartTime:     time.Now().Add(48 * time.Hour),
			Status:        "ACTIVE",
		}, nil
	}

	return nil, fmt.Errorf("event not found: %s", eventID)
}

// handleEventMashup serves GET /events/:id/mashup
func (s *Server) handleEventMashup(w http.ResponseWriter, r *http.Request, eventID string) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if eventID == "" {
		http.Error(w, `{"error":"Event ID is required"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	cacheKey := fmt.Sprintf("mashup:event:%s", eventID)

	// Check Redis cache
	if s.rdb != nil {
		cachedData, err := s.rdb.Get(ctx, cacheKey).Result()
		if err == nil {
			var cachedResp EventMashupResponse
			if err := json.Unmarshal([]byte(cachedData), &cachedResp); err == nil {
				cachedResp.Cached = true
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				_ = json.NewEncoder(w).Encode(cachedResp)
				return
			}
		}
	}

	event, err := s.fetchEventByID(ctx, eventID)
	if err != nil || event == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Event not found", "eventId": eventID})
		return
	}

	city, lat, lon := resolveCityCoords(event.VenueLocation)

	mashupCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()

	httpClient := &http.Client{Timeout: 2500 * time.Millisecond}

	var wg sync.WaitGroup
	var weather WeatherForecast
	var wFallback bool
	var media MediaTrackPreview
	var mFallback bool

	wg.Add(2)
	go func() {
		defer wg.Done()
		weather, wFallback = fetchOpenMeteoWeather(mashupCtx, httpClient, "", lat, lon)
	}()

	go func() {
		defer wg.Done()
		media, mFallback = fetchITunesPreview(mashupCtx, httpClient, "", event.Title)
	}()

	wg.Wait()

	fallbackUsed := wFallback || mFallback
	if fallbackUsed {
		w.Header().Set("X-Mashup-Degraded", "true")
	}

	resp := EventMashupResponse{
		EventID:      event.ID,
		EventTitle:   event.Title,
		VenueName:    event.VenueName,
		VenueCity:    city,
		Latitude:     lat,
		Longitude:    lon,
		Weather:      weather,
		MediaPreview: media,
		FallbackUsed: fallbackUsed,
		Cached:       false,
		FetchedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	jsonData, err := json.Marshal(resp)
	if err == nil && s.rdb != nil && !fallbackUsed {
		_ = s.rdb.Set(ctx, cacheKey, jsonData, 30*time.Minute).Err()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonData)
}
