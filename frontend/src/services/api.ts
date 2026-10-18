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


