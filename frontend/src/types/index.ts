export type SeatStatus = 'AVAILABLE' | 'HOLD' | 'BOOKED' | 'SELECTED' | 'BLOCKED';

export interface TicketTier {
  id: string;
  name: string;
  price: number;
  capacity?: number;
  totalSeats?: number;
  availableSeats?: number;
}

export interface EventItem {
  id: string;
  title: string;
  description: string;
  category: string;
  venueName: string;
  venueLocation: string;
  startTime: string;
  endTime: string;
  status: 'ACTIVE' | 'UPCOMING' | 'SOLD_OUT' | 'CANCELLED';
  imageUrl?: string;
  tiers: TicketTier[];
  totalCapacity?: number;
}

export interface SeatItem {
  id: string;
  eventId?: string;
  tierId?: string;
  row: string;
  number: number;
  status: SeatStatus;
  price?: number;
  tierName?: string;
}

export interface InventorySummary {
  eventId: string;
  totalSeats: number;
  availableSeats: number;
  reservedSeats: number;
  bookedSeats: number;
  tiers?: {
    tierId: string;
    tierName: string;
    totalSeats: number;
    availableSeats: number;
    price?: number;
  }[];
}

export interface BookingRequest {
  eventId: string;
  tierId?: string;
  seats: string[];
  customerName: string;
  customerEmail: string;
  paymentInfo?: {
    method: string;
    cardToken?: string;
  };
  idempotencyKey?: string;
}

export interface TicketItem {
  ticketId: string;
  seatLabel: string;
  ticketCode: string;
  status: string;
}

export interface BookingResponse {
  bookingId: string;
  bookingReference: string;
  status: 'CONFIRMED' | 'PENDING' | 'CANCELLED' | 'FAILED';
  eventId: string;
  eventTitle?: string;
  tierName?: string;
  quantity: number;
  seats: string[];
  totalPrice: number;
  customerName: string;
  customerEmail: string;
  tickets: TicketItem[];
  createdAt: string;
}

export interface ApiError {
  status: number;
  message: string;
  error?: string;
  conflictingSeats?: string[];
}
