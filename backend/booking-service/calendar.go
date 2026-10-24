package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleBookingCalendarICS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Expected path: /api/bookings/{id}/calendar.ics or /bookings/{id}/calendar.ics
	path := strings.TrimPrefix(r.URL.Path, "/api/bookings/")
	path = strings.TrimPrefix(path, "/bookings/")
	bookingID := strings.TrimSuffix(path, "/calendar.ics")
	bookingID = strings.TrimSuffix(bookingID, ".ics")
	bookingID = strings.TrimSpace(bookingID)

	if bookingID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "MISSING_BOOKING_ID",
			"message": "booking ID required",
		})
		return
	}

	var eventID, status, customerName, customerEmail, bookingRef string
	var totalAmount float64
	var createdAt time.Time

	if s.db == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "BOOKING_NOT_FOUND",
			"message": "Database unavailable",
		})
		return
	}

	err := s.db.QueryRowContext(r.Context(), `
		SELECT event_id, status, total_amount, created_at,
		       COALESCE(customer_name, ''), COALESCE(customer_email, ''), COALESCE(booking_reference, id)
		FROM bookings
		WHERE id = $1
	`, bookingID).Scan(&eventID, &status, &totalAmount, &createdAt, &customerName, &customerEmail, &bookingRef)

	if err != nil {
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "BOOKING_NOT_FOUND",
				"message": fmt.Sprintf("Booking %s not found", bookingID),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "DATABASE_ERROR",
			"message": err.Error(),
		})
		return
	}

	// Query event metadata
	var eventTitle, venue, city string
	var startTime, endTime time.Time

	err = s.db.QueryRowContext(r.Context(), `
		SELECT title, venue_name, venue_location, start_time, COALESCE(end_time, start_time + INTERVAL '3 hours')
		FROM events
		WHERE id = $1
	`, eventID).Scan(&eventTitle, &venue, &city, &startTime, &endTime)

	if err != nil {
		eventTitle = "Live Event"
		venue = "TicketVerse Main Stage"
		city = "Online / Global"
		startTime = time.Now().Add(24 * time.Hour)
		endTime = startTime.Add(3 * time.Hour)
	}

	// Query tickets
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT COALESCE(ticket_code, id), COALESCE(seat_label, seat_id), COALESCE(tier_name, 'Standard')
		FROM tickets
		WHERE booking_id = $1
	`, bookingID)

	var seatsSummary []string
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var code, seat, tier string
			if err := rows.Scan(&code, &seat, &tier); err == nil {
				seatsSummary = append(seatsSummary, fmt.Sprintf("%s (%s, %s)", seat, tier, code))
			}
		}
	}

	seatsStr := strings.Join(seatsSummary, ", ")
	if seatsStr == "" {
		seatsStr = "General Admission"
	}

	location := venue
	if city != "" {
		location = fmt.Sprintf("%s, %s", venue, city)
	}

	// Format RFC 5545 timestamp strings (UTC: YYYYMMDDTHHMMSSZ)
	formatUTC := func(t time.Time) string {
		return t.UTC().Format("20060102T150405Z")
	}

	nowStr := formatUTC(time.Now())
	startStr := formatUTC(startTime)
	endStr := formatUTC(endTime)
	uid := fmt.Sprintf("%s@ticketverse.com", bookingID)

	description := fmt.Sprintf("Admission: %s\\nBooking Reference: %s\\nAttendee: %s (%s)\\nVenue: %s\\nDigital Pass: http://localhost:3000\\nPowered by TicketVerse Microservices",
		seatsStr, bookingRef, customerName, customerEmail, location)

	// RFC 5545 strictly mandates CRLF (\r\n) line terminators
	crlf := "\r\n"
	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR" + crlf)
	sb.WriteString("VERSION:2.0" + crlf)
	sb.WriteString("PRODID:-//TicketVerse//Event Ticketing System//EN" + crlf)
	sb.WriteString("CALSCALE:GREGORIAN" + crlf)
	sb.WriteString("METHOD:PUBLISH" + crlf)
	sb.WriteString("BEGIN:VEVENT" + crlf)
	sb.WriteString(fmt.Sprintf("UID:%s%s", uid, crlf))
	sb.WriteString(fmt.Sprintf("DTSTAMP:%s%s", nowStr, crlf))
	sb.WriteString(fmt.Sprintf("DTSTART:%s%s", startStr, crlf))
	sb.WriteString(fmt.Sprintf("DTEND:%s%s", endStr, crlf))
	sb.WriteString(fmt.Sprintf("SUMMARY:%s%s", eventTitle, crlf))
	sb.WriteString(fmt.Sprintf("DESCRIPTION:%s%s", description, crlf))
	sb.WriteString(fmt.Sprintf("LOCATION:%s%s", location, crlf))
	sb.WriteString(fmt.Sprintf("STATUS:%s%s", strings.ToUpper(status), crlf))
	sb.WriteString("BEGIN:VALARM" + crlf)
	sb.WriteString("TRIGGER:-PT2H" + crlf)
	sb.WriteString("ACTION:DISPLAY" + crlf)
	sb.WriteString(fmt.Sprintf("DESCRIPTION:Reminder: %s starts in 2 hours%s", eventTitle, crlf))
	sb.WriteString("END:VALARM" + crlf)
	sb.WriteString("END:VEVENT" + crlf)
	sb.WriteString("END:VCALENDAR" + crlf)

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"ticketverse-booking-%s.ics\"", bookingID))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sb.String()))
}
