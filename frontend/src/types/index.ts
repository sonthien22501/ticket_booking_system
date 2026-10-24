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
  addons?: SelectedAddonItem[];
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
  addons?: BookingAddon[];
  createdAt: string;
}

export interface SelectedAddonItem {
  addonId: string;
  quantity: number;
  name?: string;
  price?: number;
}

export interface EventAddon {
  id: string;
  eventId: string;
  name: string;
  description?: string;
  price: number;
  category: 'PARKING' | 'MERCHANDISE' | 'HOSPITALITY' | 'CONCESSIONS' | string;
  totalInventory?: number;
  availableInventory: number;
  maxQuantityPerBooking?: number;
  active?: boolean;
  createdAt?: string;
}

export interface BookingAddon {
  id: string;
  bookingId: string;
  addonId: string;
  name?: string;
  addonName?: string;
  unitPrice: number;
  quantity: number;
  totalPrice: number;
  createdAt?: string;
}

export interface EventReview {
  id: string;
  eventId: string;
  userId?: string;
  bookingId?: string;
  reviewerName: string;
  reviewerEmail: string;
  rating: number;
  title: string;
  comment: string;
  verifiedBuyer: boolean;
  helpfulVotes: number;
  createdAt: string;
}

export interface ReviewSummary {
  averageRating: number;
  totalReviews: number;
  ratingDistribution: Record<number, number>;
}

export interface EventReviewsResponse {
  eventId: string;
  summary: ReviewSummary;
  reviews: EventReview[];
}

export interface CreateReviewRequest {
  eventId: string;
  userId?: string;
  bookingId?: string;
  reviewerName: string;
  reviewerEmail: string;
  rating: number;
  title?: string;
  comment: string;
}

export interface SearchFacets {
  cities: Record<string, number>;
  categories: Record<string, number>;
  minPrice: number;
  maxPrice: number;
}

export interface EventSearchFilters {
  q?: string;
  city?: string;
  category?: string;
  minPrice?: number;
  maxPrice?: number;
  sortBy?: 'date_asc' | 'date_desc' | 'price_asc' | 'price_desc' | 'title_asc' | string;
}

export interface SearchEventsResponse {
  events: EventItem[];
  facets: SearchFacets;
  totalCount: number;
}

export interface VenueFloorplanSection {
  id: string;
  name: string;
  category: 'STAGE' | 'VIP_PIT' | 'ORCHESTRA' | 'MEZZANINE' | 'BALCONY' | string;
  tierId?: string;
  tierName?: string;
  capacity: number;
  basePrice: number;
  rowRange: string;
  color: string;
}

export interface VenueFloorplan {
  eventId: string;
  venueName: string;
  venueLocation: string;
  stageOrientation: string;
  totalCapacity: number;
  sections: VenueFloorplanSection[];
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

export interface AppNotification {
  id: string;
  userId?: string;
  recipientEmail: string;
  type: 'WAITLIST_PROMOTED' | 'TICKET_TRANSFERRED' | 'BOOKING_CONFIRMED' | 'SYSTEM_ALERT';
  title: string;
  message: string;
  data?: {
    eventId?: string;
    eventTitle?: string;
    bookingId?: string;
    ticketCode?: string;
    seatLabel?: string;
    reservationId?: string;
    [key: string]: any;
  };
  isRead: boolean;
  createdAt: string;
}

export interface NotificationListResponse {
  notifications: AppNotification[];
  total: number;
  unreadCount: number;
}

export interface NotificationDeliveryLog {
  id: string;
  channel: 'EMAIL' | 'SMS' | 'PUSH' | string;
  recipient: string;
  eventType: string;
  subject: string;
  body: string;
  status: 'DELIVERED' | 'FAILED' | 'QUEUED' | 'RETRIED' | string;
  attempts: number;
  errorMessage?: string;
  createdAt: string;
}

export interface DispatchNotificationRequest {
  eventType: string;
  bookingRef?: string;
  eventTitle?: string;
  recipient: string;
  customerName?: string;
  seats?: string[];
  totalPrice?: number;
  currency?: string;
  ticketPassUrl?: string;
  channel?: 'EMAIL' | 'SMS' | string;
}






