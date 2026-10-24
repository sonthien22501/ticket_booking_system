import {
  EventItem,
  InventorySummary,
  SeatItem,
  BookingRequest,
  BookingResponse,
  ApiError,
  TicketTier,
  VerifyTicketResult,
  AdminAnalyticsData,
  CreateEventPayload,
  VoucherItem,
  VoucherValidationResult,
  CreateVoucherPayload,
  InvoiceData,
  WaitlistEntry,
  JoinWaitlistPayload,
  TicketTransferPayload,
  TicketTransferResult,
  NotificationListResponse,
  EventAddon,
  EventReview,
  EventReviewsResponse,
  CreateReviewRequest,
  EventSearchFilters,
  SearchEventsResponse,
  VenueFloorplan,
} from '../types';
import { getStoredToken } from './auth';

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
  const token = typeof localStorage !== 'undefined' ? localStorage.getItem('ticketverse_auth_token') : null;
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const payload = {
    eventId: req.eventId,
    event_id: req.eventId,
    tierId: req.tierId,
    tier_id: req.tierId,
    userId: req.userId || `usr_${Date.now().toString(36)}`,
    user_id: req.userId || `usr_${Date.now().toString(36)}`,
    customerName: req.customerName,
    customer_name: req.customerName,
    customerEmail: req.customerEmail,
    customer_email: req.customerEmail,
    seats: req.seats,
    seat_ids: req.seats,
    ticketCount: req.seats.length,
    quantity: req.seats.length,
    voucherCode: req.voucherCode,
    voucher_code: req.voucherCode,
    currency: req.currency,
    companyName: req.companyName,
    company_name: req.companyName,
    taxId: req.taxId,
    tax_id: req.taxId,
    billingAddress: req.billingAddress,
    billing_address: req.billingAddress,
    addons: req.addons,
    paymentInfo: req.paymentInfo || { method: 'CREDIT_CARD', cardToken: 'tok_visa_success' },
    payment_info: req.paymentInfo || { method: 'CREDIT_CARD', card_token: 'tok_visa_success' },
    idempotencyKey: req.idempotencyKey || `idem-${Date.now()}-${Math.random().toString(36).substring(2, 8)}`,
    idempotency_key: req.idempotencyKey || `idem-${Date.now()}-${Math.random().toString(36).substring(2, 8)}`,
  };

  const res = await fetch(`${API_BASE}/bookings`, {
    method: 'POST',
    headers,
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
    voucherCode: body.voucher_code || body.voucherCode || req.voucherCode,
    discountAmount: Number(body.discount_amount ?? body.discountAmount ?? 0),
    currency: (body.currency as string) || req.currency,
    exchangeRate: Number(body.exchange_rate ?? body.exchangeRate ?? 1.0),
    taxRate: Number(body.tax_rate ?? body.taxRate ?? 0),
    taxAmount: Number(body.tax_amount ?? body.taxAmount ?? 0),
    companyName: (body.company_name || body.companyName || req.companyName) as string | undefined,
    taxId: (body.tax_id || body.taxId || req.taxId) as string | undefined,
    billingAddress: (body.billing_address || body.billingAddress || req.billingAddress) as string | undefined,
    customerName: String(body.customer_name || body.customerName || req.customerName),
    customerEmail: String(body.customer_email || body.customerEmail || req.customerEmail),
    tickets,
    addons: Array.isArray(body.addons)
      ? body.addons.map((a: Record<string, unknown>) => ({
          id: String(a.id || a.addon_id || a.addonId || ''),
          bookingId: String(a.booking_id || a.bookingId || ''),
          addonId: String(a.addon_id || a.addonId || ''),
          addonName: String(a.addon_name || a.addonName || ''),
          unitPrice: Number(a.unit_price ?? a.unitPrice ?? 0),
          quantity: Number(a.quantity ?? 0),
          totalPrice: Number(a.total_price ?? a.totalPrice ?? 0),
        }))
      : undefined,
    createdAt: String(body.created_at || body.createdAt || new Date().toISOString()),
  };
}

/**
 * Retrieves booking history for a specific user ID or customer email.
 */
export async function fetchUserBookings(params: { userId?: string; email?: string }): Promise<BookingResponse[]> {
  const token = getStoredToken();
  const query = new URLSearchParams();
  if (params.userId) query.set('userId', params.userId);
  if (params.email) query.set('email', params.email);

  const headers: Record<string, string> = {
    'Accept': 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/bookings?${query.toString()}`, {
    method: 'GET',
    headers,
  });

  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}));
    throw {
      status: res.status,
      message: errBody.message || errBody.error || `Failed to fetch bookings (${res.status})`,
    } as ApiError;
  }

  const data = await res.json();
  const rawList = Array.isArray(data) ? data : ((data && data.bookings) || []);
  return rawList.map((item: Record<string, unknown>) => {
    const ticketsRaw = (item.tickets as Array<Record<string, unknown>>) || [];
    const tickets = ticketsRaw.map((t) => ({
      ticketId: String(t.id || t.ticketId || t.ticket_id || ''),
      seatLabel: String(t.seatLabel || t.seat_label || ''),
      ticketCode: String(t.ticketCode || t.ticket_code || ''),
      status: String(t.status || 'VALID'),
    }));
    return {
      bookingId: String(item.id || item.bookingId || item.booking_id || ''),
      bookingReference: String(item.bookingReference || item.booking_reference || ''),
      status: (item.status as BookingResponse['status']) || 'CONFIRMED',
      eventId: String(item.eventId || item.event_id || ''),
      eventTitle: (item.eventTitle || item.event_title) as string | undefined,
      tierName: (item.tierName || item.tier_name) as string | undefined,
      quantity: Number(item.quantity || 1),
      seats: Array.isArray(item.seatIds) ? (item.seatIds as string[]) : Array.isArray(item.seats) ? (item.seats as string[]) : [],
      totalPrice: Number(item.totalAmount ?? item.total_amount ?? item.totalPrice ?? item.total_price ?? 0),
      voucherCode: (item.voucherCode || item.voucher_code) as string | undefined,
      discountAmount: Number(item.discountAmount ?? item.discount_amount ?? 0),
      currency: (item.currency as string) || undefined,
      exchangeRate: Number(item.exchangeRate ?? item.exchange_rate ?? 1.0),
      taxRate: Number(item.taxRate ?? item.tax_rate ?? 0),
      taxAmount: Number(item.taxAmount ?? item.tax_amount ?? 0),
      companyName: (item.companyName || item.company_name) as string | undefined,
      taxId: (item.taxId || item.tax_id) as string | undefined,
      billingAddress: (item.billingAddress || item.billing_address) as string | undefined,
      customerName: String(item.customerName || item.customer_name || ''),
      customerEmail: String(item.customerEmail || item.customer_email || ''),
      tickets,
      addons: Array.isArray(item.addons)
        ? (item.addons as Array<Record<string, unknown>>).map((a) => ({
            id: String(a.id || a.addon_id || a.addonId || ''),
            bookingId: String(a.booking_id || a.bookingId || ''),
            addonId: String(a.addon_id || a.addonId || ''),
            addonName: String(a.addon_name || a.addonName || ''),
            unitPrice: Number(a.unit_price ?? a.unitPrice ?? 0),
            quantity: Number(a.quantity ?? 0),
            totalPrice: Number(a.total_price ?? a.totalPrice ?? 0),
          }))
        : undefined,
      createdAt: String(item.createdAt || item.created_at || new Date().toISOString()),
    };
  });
}

/**
 * Cancels a booking via the backend Saga orchestrator.
 */
export async function cancelBooking(bookingId: string): Promise<{ success: boolean; message: string }> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    'Accept': 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/bookings/${bookingId}/cancel`, {
    method: 'POST',
    headers,
  });

  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}));
    throw {
      status: res.status,
      message: errBody.message || errBody.error || `Failed to cancel booking (${res.status})`,
    } as ApiError;
  }

  const body = await res.json().catch(() => ({}));
  return {
    success: true,
    message: body.message || 'Booking cancelled and seats released successfully',
  };
}

/**
 * Verifies or checks in a digital pass code.
 */
export async function verifyTicket(params: { ticketCode: string; eventId?: string; action?: 'VERIFY' | 'CHECK_IN' }): Promise<VerifyTicketResult> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'Accept': 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/bookings/verify-ticket`, {
    method: 'POST',
    headers,
    body: JSON.stringify({
      ticketCode: params.ticketCode,
      eventId: params.eventId,
      action: params.action || 'VERIFY',
    }),
  });

  const body = await res.json().catch(() => ({}));
  if (!res.ok && res.status !== 400 && res.status !== 404 && res.status !== 409) {
    throw {
      status: res.status,
      message: body.message || body.error || `Failed to verify ticket (${res.status})`,
    } as ApiError;
  }

  return {
    valid: Boolean(body.valid),
    status: body.status || 'INVALID',
    message: body.message || (body.valid ? 'Ticket is valid' : 'Ticket is invalid'),
    ticket: body.ticket ? {
      ticketId: String(body.ticket.id || body.ticket.ticketId || body.ticket.ticket_id || ''),
      seatLabel: String(body.ticket.seatLabel || body.ticket.seat_label || ''),
      ticketCode: String(body.ticket.ticketCode || body.ticket.ticket_code || ''),
      status: String(body.ticket.status || 'VALID'),
    } : undefined,
    booking: body.booking ? {
      bookingId: String(body.booking.id || body.booking.bookingId || ''),
      bookingReference: String(body.booking.bookingReference || body.booking.booking_reference || ''),
      eventTitle: body.booking.eventTitle || body.booking.event_title,
      customerName: body.booking.customerName || body.booking.customer_name,
    } : undefined,
  };
}

/**
 * Fetches admin & organizer live telemetry and analytics (Milestone 24)
 */
export async function fetchAdminAnalytics(): Promise<AdminAnalyticsData> {
  const token = getStoredToken();
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/admin/analytics`, { headers });
  if (!res.ok) {
    throw new Error(`Failed to fetch admin analytics: HTTP ${res.status}`);
  }
  return res.json();
}

/**
 * Creates a new event in the catalog with automatic tiers and seat provisioning (Milestone 24)
 */
export async function createAdminEvent(payload: CreateEventPayload): Promise<{ id: string; status: string }> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/events`, {
    method: 'POST',
    headers,
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    const errorBody = await res.json().catch(() => ({}));
    throw new Error(errorBody.error || `Failed to create event: HTTP ${res.status}`);
  }
  return res.json();
}

/**
 * Updates event status (ACTIVE, PAUSED, COMPLETED, CANCELLED) (Milestone 24)
 */
export async function updateEventStatus(eventId: string, status: string): Promise<void> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/events/${eventId}/status`, {
    method: 'PUT',
    headers,
    body: JSON.stringify({ status }),
  });
  if (!res.ok) {
    throw new Error(`Failed to update event status: HTTP ${res.status}`);
  }
}

/**
 * Local fallback dictionary for demo/offline voucher evaluations (Milestone 25)
 */
const FALLBACK_VOUCHERS: Record<string, { type: 'PERCENTAGE' | 'FIXED'; value: number; minSpend: number }> = {
  EARLYBIRD20: { type: 'PERCENTAGE', value: 20, minSpend: 50 },
  SAVE10: { type: 'FIXED', value: 10, minSpend: 30 },
  VIP50: { type: 'PERCENTAGE', value: 50, minSpend: 100 },
  WELCOME5: { type: 'FIXED', value: 5, minSpend: 0 },
};

/**
 * Validates a promotional voucher code against order subtotal (Milestone 25)
 */
export async function validateVoucher(
  code: string,
  subtotal: number,
  eventId?: string
): Promise<VoucherValidationResult> {
  const normCode = code.trim().toUpperCase();
  if (!normCode) {
    return {
      valid: false,
      code: '',
      message: 'Voucher code is required.',
    };
  }

  try {
    const res = await fetch(`${API_BASE}/vouchers/validate`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
      body: JSON.stringify({
        code: normCode,
        subtotal,
        eventId,
      }),
    });

    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
      return {
        valid: false,
        code: normCode,
        message: data.message || data.error || `Invalid voucher code (HTTP ${res.status})`,
      };
    }

    return {
      valid: Boolean(data.valid),
      code: String(data.code || normCode),
      discountType: data.discountType,
      discountValue: Number(data.discountValue ?? 0),
      discountAmount: Number(data.discountAmount ?? 0),
      finalTotal: Number(data.finalTotal ?? subtotal),
      minSpend: Number(data.minSpend ?? 0),
      message: data.message,
    };
  } catch (err) {
    // Network failure fallback for offline/demo resilience
    const fallback = FALLBACK_VOUCHERS[normCode];
    if (fallback) {
      if (fallback.minSpend > 0 && subtotal < fallback.minSpend) {
        return {
          valid: false,
          code: normCode,
          minSpend: fallback.minSpend,
          message: `minimum spend of $${fallback.minSpend.toFixed(2)} required for voucher '${normCode}'`,
        };
      }
      let discount = fallback.type === 'PERCENTAGE' ? (subtotal * fallback.value) / 100 : fallback.value;
      if (discount > subtotal) discount = subtotal;
      discount = Math.round(discount * 100) / 100;
      const finalTotal = Math.max(0, Math.round((subtotal - discount) * 100) / 100);

      return {
        valid: true,
        code: normCode,
        discountType: fallback.type,
        discountValue: fallback.value,
        discountAmount: discount,
        finalTotal,
        minSpend: fallback.minSpend,
        message: `Voucher '${normCode}' applied successfully! Saved $${discount.toFixed(2)}`,
      };
    }

    return {
      valid: false,
      code: normCode,
      message: `voucher code '${normCode}' not found`,
    };
  }
}

/**
 * Fetches all promotional vouchers for the Organizer Studio (Milestone 25)
 */
export async function fetchAdminVouchers(): Promise<VoucherItem[]> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  try {
    const res = await fetch(`${API_BASE}/admin/vouchers`, { headers });
    if (!res.ok) {
      throw new Error(`Failed to fetch admin vouchers: HTTP ${res.status}`);
    }
    const data = await res.json();
    return Array.isArray(data) ? data : [];
  } catch (err) {
    // Fallback vouchers for offline preview
    return [
      {
        id: 'vch-earlybird20',
        code: 'EARLYBIRD20',
        discountType: 'PERCENTAGE',
        discountValue: 20,
        minSpend: 50,
        maxUses: 200,
        currentUses: 14,
        active: true,
        createdAt: new Date().toISOString(),
      },
      {
        id: 'vch-save10',
        code: 'SAVE10',
        discountType: 'FIXED',
        discountValue: 10,
        minSpend: 30,
        maxUses: 500,
        currentUses: 48,
        active: true,
        createdAt: new Date().toISOString(),
      },
      {
        id: 'vch-vip50',
        code: 'VIP50',
        discountType: 'PERCENTAGE',
        discountValue: 50,
        minSpend: 100,
        maxUses: 50,
        currentUses: 8,
        active: true,
        createdAt: new Date().toISOString(),
      },
      {
        id: 'vch-welcome5',
        code: 'WELCOME5',
        discountType: 'FIXED',
        discountValue: 5,
        minSpend: 0,
        maxUses: 1000,
        currentUses: 120,
        active: true,
        createdAt: new Date().toISOString(),
      },
    ];
  }
}

/**
 * Creates a new campaign voucher code in Organizer Studio (Milestone 25)
 */
export async function createAdminVoucher(payload: CreateVoucherPayload): Promise<VoucherItem> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/admin/vouchers`, {
    method: 'POST',
    headers,
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}));
    throw new Error(errBody.error || `Failed to create voucher: HTTP ${res.status}`);
  }

  return res.json();
}

/**
 * Returns direct URL to download binary invoice PDF (Milestone 26)
 */
export function getInvoicePdfUrl(bookingId: string): string {
  return `${API_BASE}/bookings/${bookingId}/invoice/pdf`;
}

/**
 * Fetches structured invoice metadata for a booking (Milestone 26)
 */
export async function getInvoice(bookingId: string): Promise<InvoiceData> {
  const token = getStoredToken();
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/bookings/${bookingId}/invoice`, { headers });
  if (!res.ok) {
    throw new Error(`Failed to fetch invoice: HTTP ${res.status}`);
  }
  return res.json();
}

/**
 * Downloads the official PDF ticket receipt/invoice directly to the user's filesystem (Milestone 26)
 */
export async function downloadInvoicePdf(bookingId: string, reference?: string): Promise<void> {
  const token = getStoredToken();
  const headers: Record<string, string> = {};
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/bookings/${bookingId}/invoice/pdf`, { headers });
  if (!res.ok) {
    throw new Error(`Failed to download invoice PDF: HTTP ${res.status}`);
  }

  const blob = await res.blob();
  const url = window.URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `invoice-${reference || bookingId}.pdf`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  window.URL.revokeObjectURL(url);
}

/**
 * Join event waitlist queue (Milestone 27)
 */
export async function joinWaitlist(payload: JoinWaitlistPayload): Promise<WaitlistEntry> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/waitlist`, {
    method: 'POST',
    headers,
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Failed to join waitlist: HTTP ${res.status}`);
  }
  return res.json();
}

/**
 * Fetch waitlist entries for a user, email, or event (Milestone 27)
 */
export async function fetchUserWaitlists(params?: {
  userId?: string;
  email?: string;
  eventId?: string;
}): Promise<WaitlistEntry[]> {
  const token = getStoredToken();
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const queryParams = new URLSearchParams();
  if (params?.userId) queryParams.set('userId', params.userId);
  if (params?.email) queryParams.set('email', params.email);
  if (params?.eventId) queryParams.set('eventId', params.eventId);

  const url = `${API_BASE}/waitlist${queryParams.toString() ? `?${queryParams.toString()}` : ''}`;
  const res = await fetch(url, { headers });

  if (!res.ok) {
    if (res.status === 404) return [];
    throw new Error(`Failed to fetch waitlist: HTTP ${res.status}`);
  }

  const data = await res.json();
  if (!Array.isArray(data)) return [];

  return data.map((item: Record<string, unknown>) => ({
    id: String(item.id || ''),
    eventId: String(item.eventId || item.event_id || ''),
    eventTitle: String(item.eventTitle || item.event_title || ''),
    tierId: item.tierId || item.tier_id ? String(item.tierId || item.tier_id) : undefined,
    tierName: item.tierName || item.tier_name ? String(item.tierName || item.tier_name) : undefined,
    seatId: item.seatId || item.seat_id ? String(item.seatId || item.seat_id) : undefined,
    userId: String(item.userId || item.user_id || ''),
    customerName: String(item.customerName || item.customer_name || ''),
    customerEmail: String(item.customerEmail || item.customer_email || ''),
    quantity: Number(item.quantity || 1),
    status: (item.status || 'PENDING') as WaitlistEntry['status'],
    reservationId: item.reservationId || item.reservation_id ? String(item.reservationId || item.reservation_id) : undefined,
    promotedAt: item.promotedAt || item.promoted_at ? String(item.promotedAt || item.promoted_at) : undefined,
    expiresAt: item.expiresAt || item.expires_at ? String(item.expiresAt || item.expires_at) : undefined,
    position: item.position !== undefined ? Number(item.position) : undefined,
    createdAt: String(item.createdAt || item.created_at || new Date().toISOString()),
    updatedAt: String(item.updatedAt || item.updated_at || new Date().toISOString()),
  }));
}

/**
 * Cancel / leave waitlist entry (Milestone 27)
 */
export async function cancelWaitlistEntry(waitlistId: string, userId?: string): Promise<void> {
  const token = getStoredToken();
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const url = `${API_BASE}/waitlist/${waitlistId}${userId ? `?userId=${encodeURIComponent(userId)}` : ''}`;
  const res = await fetch(url, {
    method: 'DELETE',
    headers,
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Failed to cancel waitlist entry: HTTP ${res.status}`);
  }
}

/**
 * Claim a promoted waitlist reservation (Milestone 27)
 */
export async function claimWaitlistReservation(waitlistId: string): Promise<BookingResponse> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/waitlist/${waitlistId}/claim`, {
    method: 'POST',
    headers,
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Failed to claim waitlist hold: HTTP ${res.status}`);
  }
  return res.json();
}

/**
 * Initiate secure digital ticket transfer to another recipient (Milestone 27)
 */
export async function transferTicket(payload: TicketTransferPayload): Promise<TicketTransferResult> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/tickets/transfer`, {
    method: 'POST',
    headers,
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Failed to transfer ticket: HTTP ${res.status}`);
  }
  return res.json();
}

/**
 * Fetch in-app notifications for attendee / user (Milestone 28)
 */
export async function fetchNotifications(email?: string, userId?: string): Promise<NotificationListResponse> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const params = new URLSearchParams();
  if (email) params.append('email', email);
  if (userId) params.append('userId', userId);

  const res = await fetch(`${API_BASE}/notifications?${params.toString()}`, {
    method: 'GET',
    headers,
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Failed to fetch notifications: HTTP ${res.status}`);
  }
  return res.json();
}

/**
 * Mark a single notification as read (Milestone 28)
 */
export async function markNotificationRead(notificationId: string): Promise<void> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/notifications/${notificationId}/read`, {
    method: 'POST',
    headers,
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Failed to mark notification read: HTTP ${res.status}`);
  }
}

/**
 * Mark all notifications as read for current user/email (Milestone 28)
 */
export async function markAllNotificationsRead(email?: string, userId?: string): Promise<void> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/notifications/read-all`, {
    method: 'POST',
    headers,
    body: JSON.stringify({ email, userId }),
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `Failed to mark all notifications read: HTTP ${res.status}`);
  }
}

/**
 * Return direct download link for standard RFC 5545 iCalendar (.ics) (Milestone 28)
 */
export function getCalendarICSUrl(bookingId: string): string {
  return `${API_BASE}/bookings/${bookingId}/calendar.ics`;
}

/**
 * Trigger browser file download of calendar .ics pass (Milestone 28)
 */
export async function downloadBookingCalendarICS(bookingId: string, eventTitle?: string): Promise<void> {
  const url = getCalendarICSUrl(bookingId);
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`Failed to download calendar event: HTTP ${res.status}`);
  }
  const blob = await res.blob();
  if (typeof window !== 'undefined' && window.URL && typeof document !== 'undefined') {
    const blobUrl = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = blobUrl;
    const safeTitle = (eventTitle || 'event').toLowerCase().replace(/[^a-z0-9]/g, '-');
    a.download = `${safeTitle}-${bookingId}.ics`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(blobUrl);
  }
}

/**
 * Fetch available VIP Add-ons, Merch and Concessions for an event (Milestone 29)
 */
export async function fetchEventAddons(eventId: string): Promise<EventAddon[]> {
  try {
    const res = await fetch(`${API_BASE}/events/${eventId}/addons`);
    if (res.ok) {
      const data = await res.json();
      const list = Array.isArray(data) ? data : Array.isArray(data?.addons) ? data.addons : [];
      if (list.length > 0) {
        return list.map((a: Record<string, unknown>) => ({
          id: String(a.id),
          eventId: String(a.event_id || a.eventId || eventId),
          name: String(a.name),
          description: String(a.description || ''),
          price: Number(a.price || 0),
          category: (a.category as EventAddon['category']) || 'MERCHANDISE',
          availableInventory: Number(a.available_inventory ?? a.availableInventory ?? 0),
          maxQuantityPerBooking: Number(a.max_quantity_per_booking ?? a.maxQuantityPerBooking ?? 5),
        }));
      }
    }
  } catch (err) {
    console.warn(`Could not fetch addons for event ${eventId}:`, err);
  }

  // Graceful fallback for offline demo / decoupled mode
  return [
    {
      id: `addon-parking-${eventId}`,
      eventId,
      name: 'Priority Venue Parking',
      description: 'Dedicated reserved bay adjacent to VIP stadium entrance',
      price: 25.0,
      category: 'PARKING',
      availableInventory: 50,
      maxQuantityPerBooking: 2,
    },
    {
      id: `addon-tshirt-${eventId}`,
      eventId,
      name: 'Limited Edition Tour T-Shirt',
      description: 'Premium heavyweight cotton tour commemorative tee',
      price: 35.0,
      category: 'MERCHANDISE',
      availableInventory: 100,
      maxQuantityPerBooking: 4,
    },
    {
      id: `addon-lounge-${eventId}`,
      eventId,
      name: 'Executive Lounge & Open Bar Pass',
      description: 'Exclusive lounge access with complimentary craft beverages',
      price: 75.0,
      category: 'VIP_LOUNGE',
      availableInventory: 30,
      maxQuantityPerBooking: 4,
    },
  ];
}

/**
 * Fetch attendee reviews and summary statistics for an event (Milestone 30)
 */
export async function fetchEventReviews(eventId: string): Promise<EventReviewsResponse> {
  try {
    const res = await fetch(`${API_BASE}/events/${eventId}/reviews`);
    if (res.ok) {
      const data = await res.json();
      if (data && data.summary && Array.isArray(data.reviews)) {
        return data as EventReviewsResponse;
      }
    }
  } catch (err) {
    console.warn(`Could not fetch reviews for event ${eventId}:`, err);
  }

  // Graceful fallback for offline demo / decoupled mode
  return {
    eventId,
    summary: {
      averageRating: 4.8,
      totalReviews: 3,
      ratingDistribution: { 1: 0, 2: 0, 3: 0, 4: 1, 5: 2 },
    },
    reviews: [
      {
        id: `rev-${eventId}-1`,
        eventId,
        reviewerName: 'Alex Rivers',
        reviewerEmail: 'alex@example.com',
        rating: 5,
        title: 'Unbelievable audio fidelity and lighting!',
        comment: 'Row A stage-front was an unforgettable sensory experience. Laser show synchronized to the orchestral drops flawlessly.',
        verifiedBuyer: true,
        helpfulVotes: 24,
        createdAt: new Date(Date.now() - 86400000 * 2).toISOString(),
      },
      {
        id: `rev-${eventId}-2`,
        eventId,
        reviewerName: 'Maria Garcia',
        reviewerEmail: 'maria@example.com',
        rating: 5,
        title: 'Seamless digital ticket entry and pristine venue',
        comment: 'Fast check-in via Apple Wallet passbook pass. Sound engineering in the Grand Hall was second to none.',
        verifiedBuyer: true,
        helpfulVotes: 14,
        createdAt: new Date(Date.now() - 86400000).toISOString(),
      },
      {
        id: `rev-${eventId}-3`,
        eventId,
        reviewerName: 'Elena Rostova',
        reviewerEmail: 'elena@example.com',
        rating: 4,
        title: 'Masterclass musicianship, minor queue at concessions',
        comment: 'The performance blew the roof off. Highly recommend grabbing VIP express parking add-on to skip arena congestion.',
        verifiedBuyer: true,
        helpfulVotes: 8,
        createdAt: new Date(Date.now() - 43200000).toISOString(),
      },
    ],
  };
}

/**
 * Submit an attendee review for an event (Milestone 30)
 */
export async function submitEventReview(req: CreateReviewRequest): Promise<EventReview> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${API_BASE}/events/${req.eventId}/reviews`, {
    method: 'POST',
    headers,
    body: JSON.stringify(req),
  });

  if (!res.ok) {
    const errBody = await res.json().catch(() => ({}));
    throw new Error(errBody.error || errBody.message || `Failed to submit review (HTTP ${res.status})`);
  }

  return res.json();
}

/**
 * Vote helpful on a review (Milestone 30)
 */
export async function voteHelpfulReview(reviewId: string): Promise<boolean> {
  const res = await fetch(`${API_BASE}/reviews/${reviewId}/vote`, {
    method: 'POST',
    headers: { Accept: 'application/json' },
  });
  return res.ok;
}

/**
 * Advanced multi-facet event search & geolocation filtering (Milestone 31)
 */
export async function searchEvents(filters: EventSearchFilters): Promise<SearchEventsResponse> {
  const query = new URLSearchParams();
  if (filters.q) query.set('q', filters.q);
  if (filters.city && filters.city !== 'ALL') query.set('city', filters.city);
  if (filters.category && filters.category !== 'ALL') query.set('category', filters.category);
  if (filters.minPrice !== undefined) query.set('min_price', String(filters.minPrice));
  if (filters.maxPrice !== undefined) query.set('max_price', String(filters.maxPrice));
  if (filters.sortBy) query.set('sort_by', filters.sortBy);

  try {
    const res = await fetch(`${API_BASE}/events/search?${query.toString()}`);
    if (res.ok) {
      const data = await res.json();
      if (data && Array.isArray(data.events) && data.facets) {
        return {
          events: data.events.map(normalizeEvent),
          facets: data.facets,
          totalCount: data.totalCount || data.events.length,
        };
      }
    }
  } catch (err) {
    console.warn('Search events API error:', err);
  }

  // Local client fallback: fetch all events and filter locally
  const allEvents = await fetchEvents();
  const qLower = (filters.q || '').toLowerCase();
  const cityLower = (filters.city || '').toLowerCase();
  const catUpper = (filters.category || '').toUpperCase();

  const citiesMap: Record<string, number> = {};
  const categoriesMap: Record<string, number> = {};
  let minP = 999999;
  let maxP = 0;

  allEvents.forEach((e) => {
    const city = (e.venueLocation || '').split(',')[0].trim() || 'General';
    citiesMap[city] = (citiesMap[city] || 0) + 1;
    categoriesMap[e.category] = (categoriesMap[e.category] || 0) + 1;
    e.tiers.forEach((t) => {
      if (t.price < minP) minP = t.price;
      if (t.price > maxP) maxP = t.price;
    });
  });

  const filtered = allEvents.filter((e) => {
    if (qLower) {
      const combined = `${e.title} ${e.description} ${e.venueName} ${e.venueLocation}`.toLowerCase();
      if (!combined.includes(qLower)) return false;
    }
    if (cityLower && cityLower !== 'all') {
      const eCity = (e.venueLocation || '').toLowerCase();
      if (!eCity.includes(cityLower)) return false;
    }
    if (catUpper && catUpper !== 'ALL') {
      if (e.category.toUpperCase() !== catUpper) return false;
    }
    const lowestPrice = e.tiers.length > 0 ? Math.min(...e.tiers.map((t) => t.price)) : 0;
    if (filters.minPrice !== undefined && lowestPrice < filters.minPrice) return false;
    if (filters.maxPrice !== undefined && lowestPrice > filters.maxPrice) return false;
    return true;
  });

  return {
    events: filtered,
    facets: {
      cities: citiesMap,
      categories: categoriesMap,
      minPrice: minP === 999999 ? 0 : minP,
      maxPrice: maxP,
    },
    totalCount: filtered.length,
  };
}

/**
 * Fetch venue floorplan sections for an event (Milestone 31)
 */
export async function fetchVenueFloorplan(eventId: string): Promise<VenueFloorplan> {
  try {
    const res = await fetch(`${API_BASE}/events/${eventId}/floorplan`);
    if (res.ok) {
      const data = await res.json();
      if (data && Array.isArray(data.sections)) {
        return data as VenueFloorplan;
      }
    }
  } catch (err) {
    console.warn(`Could not load floorplan for event ${eventId}:`, err);
  }

  // Graceful fallback floorplan
  return {
    eventId,
    venueName: 'Grand Arena',
    venueLocation: 'San Francisco, CA',
    stageOrientation: 'NORTH',
    totalCapacity: 68,
    sections: [
      {
        id: 'sec-stage',
        name: 'Main Center Stage',
        category: 'STAGE',
        capacity: 0,
        basePrice: 0,
        rowRange: 'Center Stage',
        color: '#6366f1',
      },
      {
        id: 'sec-vip',
        name: 'VIP Front Stage Pit',
        category: 'VIP_PIT',
        capacity: 20,
        basePrice: 150.0,
        rowRange: 'Rows A - B',
        color: '#f59e0b',
      },
      {
        id: 'sec-orch',
        name: 'Orchestra Lower Bowl',
        category: 'ORCHESTRA',
        capacity: 48,
        basePrice: 60.0,
        rowRange: 'Rows C - F',
        color: '#3b82f6',
      },
    ],
  };
}

