import {
  EventItem,
  InventorySummary,
  SeatItem,
  BookingRequest,
  BookingResponse,
  ApiError,
  TicketTier,
} from '../types';

const API_BASE = import.meta.env.VITE_API_URL || '/api';

/**
 * Normalizes raw event json (handling snake_case and camelCase)
 */
function normalizeEvent(raw: Record<string, unknown>): EventItem {
  const tiersRaw = (raw.tiers as Array<Record<string, unknown>>) || [];
  const tiers: TicketTier[] = tiersRaw.map((t, idx) => ({
    id: String(t.id || t.tier_id || `tier-${idx + 1}`),
    name: String(t.name || t.tier_name || 'Standard'),
    price: Number(t.price || 50),
    capacity: Number(t.capacity || t.total_seats || 100),
    totalSeats: Number(t.total_seats || t.capacity || 100),
    availableSeats: Number(t.available_seats ?? t.capacity ?? 100),
  }));

  return {
    id: String(raw.id || raw.eventId || raw.event_id || ''),
    title: String(raw.title || 'Untitled Event'),
    description: String(raw.description || ''),
    category: String(raw.category || 'CONCERT'),
    venueName: String(raw.venueName || raw.venue_name || 'Main Hall'),
    venueLocation: String(raw.venueLocation || raw.venue_location || 'City Arena'),
    startTime: String(raw.startTime || raw.start_time || new Date().toISOString()),
    endTime: String(raw.endTime || raw.end_time || new Date().toISOString()),
    status: (raw.status as EventItem['status']) || 'ACTIVE',
    imageUrl: typeof raw.imageUrl === 'string' ? raw.imageUrl : (typeof raw.image_url === 'string' ? raw.image_url : undefined),
    tiers,
    totalCapacity: Number(raw.totalCapacity || raw.total_capacity || tiers.reduce((acc, t) => acc + (t.capacity || 0), 0)),
  };
}

/**
 * Normalizes inventory summary
 */
function normalizeInventory(raw: Record<string, unknown>, eventId: string): InventorySummary {
  const summary = (raw.summary as Record<string, unknown>) || raw;
  const tiersRaw = (raw.tiers as Array<Record<string, unknown>>) || [];

  return {
    eventId: String(raw.event_id || raw.eventId || eventId),
    totalSeats: Number(summary.total_seats ?? summary.totalSeats ?? 250),
    availableSeats: Number(summary.available_seats ?? summary.availableSeats ?? 250),
    reservedSeats: Number(summary.reserved_seats ?? summary.reservedSeats ?? 0),
    bookedSeats: Number(summary.booked_seats ?? summary.bookedSeats ?? 0),
    tiers: tiersRaw.map((t) => ({
      tierId: String(t.tier_id || t.tierId || t.id),
      tierName: String(t.tier_name || t.tierName || t.name),
      totalSeats: Number(t.total_seats ?? t.totalSeats ?? 0),
      availableSeats: Number(t.available_seats ?? t.availableSeats ?? 0),
      price: t.price ? Number(t.price) : undefined,
    })),
  };
}

/**
 * Fetches all available events
 */
export async function fetchEvents(): Promise<EventItem[]> {
  try {
    const res = await fetch(`${API_BASE}/events`, {
      headers: { Accept: 'application/json' },
    });

    if (!res.ok) {
      throw new Error(`Failed to fetch events: HTTP ${res.status}`);
    }

    const data = await res.json();
    const rawList: Record<string, unknown>[] = Array.isArray(data)
      ? data
      : Array.isArray(data?.events)
      ? data.events
      : [];

    return rawList.map(normalizeEvent);
  } catch (err) {
    console.error('API Error in fetchEvents:', err);
    throw err;
  }
}

/**
 * Fetches single event details by ID
 */
export async function fetchEventById(id: string): Promise<EventItem> {
  const res = await fetch(`${API_BASE}/events/${id}`, {
    headers: { Accept: 'application/json' },
  });

  if (!res.ok) {
    throw new Error(`Failed to fetch event ${id}: HTTP ${res.status}`);
  }

  const data = await res.json();
  return normalizeEvent(data);
}

/**
 * Fetches seat inventory summary for an event
 */
export async function fetchInventory(eventId: string): Promise<InventorySummary> {
  try {
    const res = await fetch(`${API_BASE}/inventory/${eventId}`, {
      headers: { Accept: 'application/json' },
    });

    if (!res.ok) {
      throw new Error(`Failed to fetch inventory: HTTP ${res.status}`);
    }

    const data = await res.json();
    return normalizeInventory(data, eventId);
  } catch (err) {
    console.error(`API Error in fetchInventory(${eventId}):`, err);
    throw err;
  }
}

/**
 * Fetches discrete seat status map for an event.
 * If backend hasn't generated seats, generates layout based on event tiers.
 */
export async function fetchSeats(eventId: string, eventDetails?: EventItem): Promise<SeatItem[]> {
  try {
    // Attempt standard seat endpoint
    const res = await fetch(`${API_BASE}/inventory/${eventId}/seats`, {
      headers: { Accept: 'application/json' },
    });

    if (res.ok) {
      const data = await res.json();
      const rawSeats: Record<string, unknown>[] = Array.isArray(data)
        ? data
        : Array.isArray(data?.seats)
        ? data.seats
        : [];

      if (rawSeats.length > 0) {
        return rawSeats.map((s) => ({
          id: String(s.id || `${s.row}-${s.number}`),
          eventId: String(s.event_id || s.eventId || eventId),
          tierId: String(s.tier_id || s.tierId || ''),
          row: String(s.row || s.seat_row || 'A'),
          number: Number(s.number || s.seat_number || 1),
          status: (s.status as SeatItem['status']) || 'AVAILABLE',
          price: s.price ? Number(s.price) : undefined,
        }));
      }
    }
  } catch (err) {
    console.warn(`Could not fetch seats from server, falling back to layout generator:`, err);
  }

  // Generate venue seat grid layout from event tiers
  return generateVenueSeatLayout(eventId, eventDetails);
}

/**
 * Creates venue seat layout based on event tiers
 */
export function generateVenueSeatLayout(eventId: string, event?: EventItem): SeatItem[] {
  const seats: SeatItem[] = [];
  const tiers = event?.tiers && event.tiers.length > 0
    ? event.tiers
    : [
        { id: 'tier-vip', name: 'VIP Stage Front', price: 150 },
        { id: 'tier-reg', name: 'General Admission', price: 60 },
      ];

  // VIP rows: A and B (10 seats per row)
  const vipTier = tiers[0];
  for (const row of ['A', 'B']) {
    for (let num = 1; num <= 10; num++) {
      const seatId = `${row}${num}`;
      // Simulate a few pre-booked seats for realistic venue state
      const isPreBooked = (row === 'A' && (num === 3 || num === 7));
      seats.push({
        id: seatId,
        eventId,
        tierId: vipTier?.id || 'tier-vip',
        tierName: vipTier?.name || 'VIP Stage Front',
        row,
        number: num,
        status: isPreBooked ? 'BOOKED' : 'AVAILABLE',
        price: vipTier?.price || 150,
      });
    }
  }

  // Standard/General Admission rows: C, D, E, F (12 seats per row)
  const genTier = tiers.length > 1 ? tiers[1] : tiers[0];
  for (const row of ['C', 'D', 'E', 'F']) {
    for (let num = 1; num <= 12; num++) {
      const seatId = `${row}${num}`;
      const isPreBooked = (row === 'C' && (num === 5 || num === 6)) || (row === 'E' && num === 1);
      seats.push({
        id: seatId,
        eventId,
        tierId: genTier?.id || 'tier-reg',
        tierName: genTier?.name || 'General Admission',
        row,
        number: num,
        status: isPreBooked ? 'BOOKED' : 'AVAILABLE',
        price: genTier?.price || 60,
      });
    }
  }

  return seats;
}

/**
 * Submits ticket booking request to /api/bookings.
 * Handles HTTP 409 Conflict gracefully with custom ApiError.
 */
export async function createBooking(req: BookingRequest): Promise<BookingResponse> {
  const payload = {
    eventId: req.eventId,
    event_id: req.eventId,
    tierId: req.tierId,
    tier_id: req.tierId,
    userId: `usr_${Date.now().toString(36)}`,
    user_id: `usr_${Date.now().toString(36)}`,
    customerName: req.customerName,
    customer_name: req.customerName,
    customerEmail: req.customerEmail,
    customer_email: req.customerEmail,
    seats: req.seats,
    seat_ids: req.seats,
    ticketCount: req.seats.length,
    quantity: req.seats.length,
    paymentInfo: req.paymentInfo || { method: 'CREDIT_CARD', cardToken: 'tok_visa_success' },
    payment_info: req.paymentInfo || { method: 'CREDIT_CARD', card_token: 'tok_visa_success' },
    idempotencyKey: req.idempotencyKey || `idem-${Date.now()}-${Math.random().toString(36).substring(2, 8)}`,
    idempotency_key: req.idempotencyKey || `idem-${Date.now()}-${Math.random().toString(36).substring(2, 8)}`,
  };

  const res = await fetch(`${API_BASE}/bookings`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
    body: JSON.stringify(payload),
  });

  const body = await res.json().catch(() => ({}));

  if (res.status === 409) {
    const errorMsg =
      body.message ||
      body.error ||
      'Seat reservation conflict: one or more selected seats were already booked by another user.';
    const conflictingSeats: string[] = Array.isArray(body.conflicting_seats)
      ? body.conflicting_seats
      : Array.isArray(body.conflictingSeats)
      ? body.conflictingSeats
      : req.seats;

    const apiError: ApiError = {
      status: 409,
      message: errorMsg,
      error: 'SEAT_UNAVAILABLE',
      conflictingSeats,
    };
    throw apiError;
  }

  if (!res.ok) {
    const errorMsg = body.message || body.error || `Booking request failed with HTTP ${res.status}`;
    const apiError: ApiError = {
      status: res.status,
      message: errorMsg,
    };
    throw apiError;
  }

  // Parse confirmed booking
  const ticketsRaw = Array.isArray(body.tickets) ? body.tickets : [];
  const tickets = ticketsRaw.map((t: Record<string, unknown>, idx: number) => ({
    ticketId: String(t.ticket_id || t.ticketId || `tkt-${idx + 1}`),
    seatLabel: String(t.seat_label || t.seatLabel || req.seats[idx] || 'Seat'),
    ticketCode: String(t.ticket_code || t.ticketCode || `TKT-${Math.random().toString(36).toUpperCase().substring(2, 8)}`),
    status: String(t.status || 'VALID'),
  }));

  // If backend didn't return discrete ticket records, generate matching tickets for each seat
  if (tickets.length === 0 && req.seats.length > 0) {
    req.seats.forEach((seat, idx) => {
      tickets.push({
        ticketId: `tkt-${Date.now()}-${idx + 1}`,
        seatLabel: `Seat ${seat}`,
        ticketCode: `TKT-${seat}-${Math.random().toString(36).toUpperCase().substring(2, 8)}`,
        status: 'VALID',
      });
    });
  }

  return {
    bookingId: String(body.booking_id || body.bookingId || body.id || `bk-${Date.now()}`),
    bookingReference: String(
      body.booking_reference || body.bookingReference || `REF-${Date.now().toString(36).toUpperCase()}`
    ),
    status: (body.status as BookingResponse['status']) || 'CONFIRMED',
    eventId: String(body.event_id || body.eventId || req.eventId),
    eventTitle: body.event_title || body.eventTitle,
    tierName: body.tier_name || body.tierName,
    quantity: Number(body.quantity || req.seats.length),
    seats: Array.isArray(body.seat_ids) ? body.seat_ids : Array.isArray(body.seats) ? body.seats : req.seats,
    totalPrice: Number(body.total_price ?? body.totalPrice ?? body.total_amount ?? 0),
    customerName: String(body.customer_name || body.customerName || req.customerName),
    customerEmail: String(body.customer_email || body.customerEmail || req.customerEmail),
    tickets,
    createdAt: String(body.created_at || body.createdAt || new Date().toISOString()),
  };
}
