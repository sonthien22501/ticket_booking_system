export type SeatStatus = 'AVAILABLE' | 'HOLD' | 'BOOKED' | 'SELECTED' | 'BLOCKED' | 'WAITLIST';

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
  userId?: string;
  voucherCode?: string;
  currency?: string;
  companyName?: string;
  taxId?: string;
  billingAddress?: string;
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
  price?: number;
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
  voucherCode?: string;
  discountAmount?: number;
  currency?: string;
  exchangeRate?: number;
  taxRate?: number;
  taxAmount?: number;
  companyName?: string;
  taxId?: string;
  billingAddress?: string;
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

export interface User {
  id: string;
  email: string;
  name: string;
  role: 'customer' | 'admin' | string;
  createdAt?: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export interface LoginCredentials {
  email: string;
  password: string;
}

export interface RegisterCredentials {
  name: string;
  email: string;
  password: string;
  role?: string;
}

export interface VerifyTicketResult {
  valid: boolean;
  status: 'VALID' | 'USED' | 'CANCELLED' | 'TRANSFERRED' | 'INVALID';
  message: string;
  ticket?: TicketItem;
  booking?: {
    bookingId: string;
    bookingReference: string;
    eventTitle?: string;
    customerName?: string;
  };
}

export interface AdminAnalyticsBooking {
  id: string;
  bookingReference: string;
  customerName: string;
  customerEmail: string;
  eventTitle: string;
  totalAmount: number;
  status: string;
  quantity: number;
  createdAt: string;
}

export interface AdminEventSummary {
  eventId: string;
  title: string;
  category: string;
  venueName: string;
  status: string;
  totalSeats: number;
  bookedSeats: number;
  occupancyPercent: number;
  grossRevenue: number;
}

export interface AdminAnalyticsData {
  totalRevenue: number;
  totalBookings: number;
  confirmedBookings: number;
  cancelledBookings: number;
  totalTicketsSold: number;
  activeEventsCount: number;
  averageOccupancy: number;
  recentBookings: AdminAnalyticsBooking[];
  eventsSummary: AdminEventSummary[];
}

export interface CreateEventPayload {
  title: string;
  description: string;
  category: string;
  venueName: string;
  venueLocation: string;
  startTime: string;
  endTime: string;
  imageUrl?: string;
  tiers: {
    name: string;
    price: number;
    capacity: number;
  }[];
}

export interface VoucherItem {
  id: string;
  code: string;
  discountType: 'PERCENTAGE' | 'FIXED';
  discountValue: number;
  minSpend: number;
  maxUses: number;
  currentUses: number;
  expiresAt?: string;
  active: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface VoucherValidationResult {
  valid: boolean;
  code: string;
  discountType?: 'PERCENTAGE' | 'FIXED';
  discountValue?: number;
  discountAmount?: number;
  finalTotal?: number;
  message?: string;
  minSpend?: number;
}

export interface CreateVoucherPayload {
  code: string;
  discountType: 'PERCENTAGE' | 'FIXED';
  discountValue: number;
  minSpend?: number;
  maxUses?: number;
  expiresInDays?: number;
}

export type CurrencyCode = 'USD' | 'EUR' | 'GBP' | 'JPY' | 'CAD' | 'AUD';

export interface CurrencyInfo {
  code: CurrencyCode | string;
  name: string;
  symbol: string;
  rate: number;
  taxRate: number;
  decimals: number;
}

export interface CorporateBillingDetails {
  companyName: string;
  taxId: string;
  billingAddress: string;
}

export interface InvoiceItemData {
  description: string;
  seatId: string;
  quantity: number;
  unitPrice: number;
  totalPrice: number;
}

export interface InvoiceData {
  invoiceNumber: string;
  bookingId: string;
  bookingReference: string;
  issueDate: string;
  dueDate: string;
  status: string;
  paymentStatus: string;
  customerName: string;
  customerEmail: string;
  companyName?: string;
  taxId?: string;
  billingAddress?: string;
  eventId: string;
  eventTitle: string;
  currency: string;
  currencySymbol: string;
  exchangeRate: number;
  subtotal: number;
  discount: number;
  taxableAmount: number;
  taxRate: number;
  taxAmount: number;
  totalAmount: number;
  issuerName: string;
  issuerTaxId: string;
  issuerAddress: string;
  items: InvoiceItemData[];
}

export type WaitlistStatus = 'PENDING' | 'PROMOTED' | 'CLAIMED' | 'EXPIRED' | 'CANCELLED';

export interface WaitlistEntry {
  id: string;
  eventId: string;
  eventTitle?: string;
  tierId?: string;
  tierName?: string;
  seatId?: string;
  userId: string;
  customerName: string;
  customerEmail: string;
  quantity: number;
  status: WaitlistStatus;
  reservationId?: string;
  promotedAt?: string;
  expiresAt?: string;
  position?: number;
  createdAt: string;
  updatedAt: string;
}

export interface JoinWaitlistPayload {
  eventId: string;
  tierId?: string;
  userId?: string;
  customerName: string;
  customerEmail: string;
  quantity: number;
}

export interface TicketTransferPayload {
  ticketId?: string;
  ticketCode?: string;
  recipientEmail: string;
  recipientName: string;
  fromUserId?: string;
  note?: string;
}

export interface TicketTransferResult {
  transferId: string;
  bookingId: string;
  newBookingId: string;
  oldTicketId: string;
  newTicketId: string;
  oldTicketCode: string;
  newTicketCode: string;
  recipientEmail: string;
  recipientName: string;
  seatLabel: string;
  status: string;
  transferredAt: string;
  message: string;
}




