-- ============================================================================
-- Ticket Booking System - Comprehensive Master Test Dataset
-- Covers 100% of Features:
--   1. Authentication & RBAC (Admin, Organizer, Customer, Corporate B2B)
--   2. Event Catalog (6 Categories, 4 Lifecycle Statuses: ACTIVE, PAUSED, COMPLETED, CANCELLED)
--   3. Multi-Tier Ticket Pricing & Inventory Management
--   4. Interactive Seating Layouts (AVAILABLE, HOLD, BOOKED, BLOCKED)
--   5. Promotional Voucher & Coupon Engine (Percentage, Fixed, Quotas, Expired, Inactive, Min-Spend)
--   6. Multi-Currency Checkout & Regional VAT/Sales Tax (USD, EUR, GBP, JPY, CAD, AUD)
--   7. Corporate Expense Invoicing & Digital Tax Invoices (PDF 1.4 / B2B)
--   8. Digital Passbook & QR Ticket Verification (VALID, USED, CANCELLED)
--   9. Waitlist Queue & Promotion Holds (Sold-out event test case)
--  10. Admin & Organizer Studio Telemetry (Gross revenue, occupancy %, recent order feed)
-- ============================================================================

-- Ensure all extensions and tables exist
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(64) PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'customer',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS events (
    id VARCHAR(64) PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(64) NOT NULL,
    venue_name VARCHAR(255) NOT NULL,
    venue_location VARCHAR(255) NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'ACTIVE',
    image_url VARCHAR(512),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ticket_tiers (
    id VARCHAR(64) PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    price NUMERIC(10, 2) NOT NULL,
    capacity INT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS event_inventory (
    event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    tier_id VARCHAR(64) NOT NULL,
    total_seats INT NOT NULL,
    available_seats INT NOT NULL,
    reserved_seats INT NOT NULL DEFAULT 0,
    booked_seats INT NOT NULL DEFAULT 0,
    version INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (event_id, tier_id)
);

CREATE TABLE IF NOT EXISTS seat_items (
    id VARCHAR(64) PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    tier_id VARCHAR(64) NOT NULL,
    seat_row VARCHAR(16) NOT NULL,
    seat_number INT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'AVAILABLE',
    reservation_id VARCHAR(64),
    hold_expires_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    CONSTRAINT uk_event_seat UNIQUE (event_id, seat_row, seat_number)
);

CREATE TABLE IF NOT EXISTS reservations (
    reservation_id VARCHAR(64) PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    tier_id VARCHAR(64),
    seat_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    quantity INT NOT NULL,
    status VARCHAR(32) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bookings (
    id VARCHAR(64) PRIMARY KEY,
    booking_reference VARCHAR(64) NOT NULL UNIQUE,
    user_id VARCHAR(64),
    customer_name VARCHAR(255) NOT NULL,
    customer_email VARCHAR(255) NOT NULL,
    event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    event_title VARCHAR(255) NOT NULL,
    tier_id VARCHAR(64),
    quantity INT NOT NULL,
    seat_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    total_amount NUMERIC(10, 2) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    reservation_id VARCHAR(64),
    payment_status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    idempotency_key VARCHAR(128),
    voucher_code VARCHAR(64),
    discount_amount NUMERIC(10, 2) DEFAULT 0.0,
    currency VARCHAR(10) DEFAULT 'USD',
    exchange_rate NUMERIC(10, 4) DEFAULT 1.0,
    tax_rate NUMERIC(5, 4) DEFAULT 0.085,
    tax_amount NUMERIC(10, 2) DEFAULT 0.0,
    company_name VARCHAR(255),
    tax_id VARCHAR(100),
    billing_address TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tickets (
    id VARCHAR(64) PRIMARY KEY,
    booking_id VARCHAR(64) NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    seat_id VARCHAR(64),
    seat_label VARCHAR(64) NOT NULL,
    ticket_code VARCHAR(128) NOT NULL UNIQUE,
    status VARCHAR(32) NOT NULL DEFAULT 'VALID',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS waitlist (
    id VARCHAR(64) PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    tier_id VARCHAR(64),
    seat_id VARCHAR(64),
    user_id VARCHAR(64) NOT NULL,
    customer_name VARCHAR(255) NOT NULL,
    customer_email VARCHAR(255) NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
    reservation_id VARCHAR(64),
    promoted_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS vouchers (
    id VARCHAR(64) PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    discount_type VARCHAR(32) NOT NULL,
    discount_value NUMERIC(10, 2) NOT NULL,
    min_spend NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
    max_uses INT NOT NULL DEFAULT 0,
    current_uses INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMP WITH TIME ZONE,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ticket_transfers (
    id VARCHAR(64) PRIMARY KEY,
    original_ticket_id VARCHAR(64) NOT NULL,
    original_ticket_code VARCHAR(128) NOT NULL,
    new_ticket_id VARCHAR(64) NOT NULL,
    new_ticket_code VARCHAR(128) NOT NULL,
    booking_id VARCHAR(64) NOT NULL,
    new_booking_id VARCHAR(64) NOT NULL,
    from_user_id VARCHAR(64),
    from_customer_email VARCHAR(255),
    recipient_email VARCHAR(255) NOT NULL,
    recipient_name VARCHAR(255) NOT NULL,
    note TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'COMPLETED',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notifications (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64),
    recipient_email VARCHAR(255) NOT NULL,
    type VARCHAR(64) NOT NULL,
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    data JSONB DEFAULT '{}'::jsonb,
    is_read BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS event_addons (
    id VARCHAR(64) PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price NUMERIC(10,2) NOT NULL,
    category VARCHAR(64) NOT NULL,
    total_inventory INT NOT NULL DEFAULT 100,
    available_inventory INT NOT NULL DEFAULT 100,
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS booking_addons (
    id VARCHAR(64) PRIMARY KEY,
    booking_id VARCHAR(64) NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    addon_id VARCHAR(64) NOT NULL REFERENCES event_addons(id),
    name VARCHAR(255) NOT NULL,
    unit_price NUMERIC(10,2) NOT NULL,
    quantity INT NOT NULL DEFAULT 1,
    total_price NUMERIC(10,2) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reviews (
    id VARCHAR(64) PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id VARCHAR(64) REFERENCES users(id) ON DELETE SET NULL,
    booking_id VARCHAR(64) REFERENCES bookings(id) ON DELETE SET NULL,
    reviewer_name VARCHAR(128) NOT NULL,
    reviewer_email VARCHAR(128) NOT NULL,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    title VARCHAR(255) NOT NULL,
    comment TEXT NOT NULL,
    verified_buyer BOOLEAN DEFAULT FALSE,
    helpful_votes INT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notification_deliveries (
    id VARCHAR(64) PRIMARY KEY,
    channel VARCHAR(32) NOT NULL,
    recipient VARCHAR(255) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    subject VARCHAR(255),
    body TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'DELIVERED',
    attempts INT DEFAULT 1,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index creation for high-performance lookup
CREATE INDEX IF NOT EXISTS idx_seat_items_event_status ON seat_items(event_id, status);
CREATE INDEX IF NOT EXISTS idx_seat_items_hold_expires ON seat_items(status, hold_expires_at) WHERE status = 'HOLD';
CREATE INDEX IF NOT EXISTS idx_bookings_user ON bookings(user_id);
CREATE INDEX IF NOT EXISTS idx_bookings_email ON bookings(customer_email);
CREATE INDEX IF NOT EXISTS idx_reservations_status_expires ON reservations(status, expires_at);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_bookings_idempotency ON bookings(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_waitlist_event_status_created ON waitlist(event_id, status, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_waitlist_user ON waitlist(user_id);
CREATE INDEX IF NOT EXISTS idx_waitlist_reservation ON waitlist(reservation_id);
CREATE INDEX IF NOT EXISTS idx_vouchers_code ON vouchers(code);
CREATE INDEX IF NOT EXISTS idx_vouchers_active ON vouchers(active);
CREATE INDEX IF NOT EXISTS idx_ticket_transfers_orig ON ticket_transfers(original_ticket_id);
CREATE INDEX IF NOT EXISTS idx_ticket_transfers_recipient ON ticket_transfers(recipient_email);
CREATE INDEX IF NOT EXISTS idx_notifications_recipient ON notifications(recipient_email);
CREATE INDEX IF NOT EXISTS idx_notifications_user ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_read ON notifications(is_read);
CREATE INDEX IF NOT EXISTS idx_event_addons_event ON event_addons(event_id, active);
CREATE INDEX IF NOT EXISTS idx_booking_addons_bkg ON booking_addons(booking_id);
CREATE INDEX IF NOT EXISTS idx_reviews_event ON reviews(event_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_reviews_user ON reviews(user_id);
CREATE INDEX IF NOT EXISTS idx_notif_deliveries_recipient ON notification_deliveries(recipient, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notif_deliveries_status ON notification_deliveries(status);

-- Truncate all tables for clean idempotent re-seeding
TRUNCATE TABLE notification_deliveries CASCADE;
TRUNCATE TABLE reviews CASCADE;
TRUNCATE TABLE booking_addons CASCADE;
TRUNCATE TABLE event_addons CASCADE;
TRUNCATE TABLE notifications CASCADE;
TRUNCATE TABLE waitlist CASCADE;
TRUNCATE TABLE tickets CASCADE;
TRUNCATE TABLE bookings CASCADE;
TRUNCATE TABLE reservations CASCADE;
TRUNCATE TABLE seat_items CASCADE;
TRUNCATE TABLE event_inventory CASCADE;
TRUNCATE TABLE ticket_tiers CASCADE;
TRUNCATE TABLE events CASCADE;
TRUNCATE TABLE vouchers CASCADE;
TRUNCATE TABLE users CASCADE;

-- ============================================================================
-- 1. USERS & PERSONAS (Bcrypt hashed passwords)
-- ============================================================================
INSERT INTO users (id, email, password_hash, name, role, created_at, updated_at)
VALUES
('usr-admin', 'admin@tickets.com', '$2a$10$gDiegAvqBokYXvEraZmgt.jVGGDzpp7r8ueBuJyMx31Zcd19ZE9XO', 'Elena Rostova (System Admin)', 'admin', NOW() - interval '30 days', NOW()),
('usr-organizer', 'organizer@tickets.com', '$2a$10$0DDOkMOAftkhFlaeFrCeqOWbqDj2rS2h4whMqY/bdeltB9Dgm8HZ2', 'Marcus Vance (Event Producer)', 'admin', NOW() - interval '25 days', NOW()),
('usr-demo-customer', 'user@tickets.com', '$2a$10$//ATXk37kSUwy9zsMs73uuA.3ODn5N/TnIjlc4wIom4FPzVMSh3j2', 'Alex Morgan (Demo Customer)', 'customer', NOW() - interval '20 days', NOW()),
('usr-sarah-connor', 'sarah.connor@cyberdyne.io', '$2a$10$8M8RVXwMSaEH/Ujy6dD1oOWI1odzsLOWVYrKNbOQb5STdpSCQn06a', 'Sarah Connor', 'customer', NOW() - interval '18 days', NOW()),
('usr-alex-chen', 'alex.chen@technova.com', '$2a$10$8M8RVXwMSaEH/Ujy6dD1oOWI1odzsLOWVYrKNbOQb5STdpSCQn06a', 'Alex Chen', 'customer', NOW() - interval '15 days', NOW()),
('usr-maria-garcia', 'maria.garcia@madrid.es', '$2a$10$8M8RVXwMSaEH/Ujy6dD1oOWI1odzsLOWVYrKNbOQb5STdpSCQn06a', 'Maria Garcia', 'customer', NOW() - interval '12 days', NOW()),
('usr-kenji-sato', 'kenji.sato@tokyo-synth.jp', '$2a$10$8M8RVXwMSaEH/Ujy6dD1oOWI1odzsLOWVYrKNbOQb5STdpSCQn06a', 'Kenji Sato', 'customer', NOW() - interval '10 days', NOW()),
('usr-procurement-acme', 'procurement@acme-corp.com', '$2a$10$8M8RVXwMSaEH/Ujy6dD1oOWI1odzsLOWVYrKNbOQb5STdpSCQn06a', 'Acme Global Procurement', 'customer', NOW() - interval '8 days', NOW());

-- ============================================================================
-- 2. EVENTS (6 Categories, 4 Lifecycle Statuses)
-- Ordered by start_time ASC so evt-101 is the flagship leading event
-- ============================================================================
INSERT INTO events (id, title, description, category, venue_name, venue_location, start_time, end_time, status, image_url, created_at, updated_at)
VALUES
(
    'evt-101',
    'Neon Symphony 2026',
    'An immersive electronic and orchestral live performance featuring world-renowned composers, modular synthesizers, and synchronized 3D laser projections.',
    'Music & Concerts',
    'Grand Arena Berlin',
    'Berlin, Germany',
    '2026-11-01 20:00:00+00',
    '2026-11-01 23:30:00+00',
    'ACTIVE',
    'https://images.unsplash.com/photo-1470225620780-dba8ba36b745?auto=format&fit=crop&w=1200&q=80',
    NOW() - interval '14 days',
    NOW()
),
(
    'evt-102',
    'Global Tech Summit 2026',
    'The premier developer, generative AI, and cloud architecture summit bringing together world-class software architects, keynote talks, and hands-on labs.',
    'Technology & Conferences',
    'Metropolitan Convention Center',
    'San Francisco, USA',
    '2026-11-05 09:00:00+00',
    '2026-11-07 18:00:00+00',
    'ACTIVE',
    'https://images.unsplash.com/photo-1540575467063-178a50c2df87?auto=format&fit=crop&w=1200&q=80',
    NOW() - interval '12 days',
    NOW()
),
(
    'evt-103',
    'UEFA Champions Gala Match: Berlin vs Madrid',
    'Elite European football clash under the stadium floodlights. Witness championship tactics, star forwards, and 75,000 roaring fans in an electric derby.',
    'Sports & Fitness',
    'Olympiastadion Berlin',
    'Berlin, Germany',
    '2026-11-10 18:30:00+00',
    '2026-11-10 21:00:00+00',
    'ACTIVE',
    'https://images.unsplash.com/photo-1508098682722-e99c43a406b2?auto=format&fit=crop&w=1200&q=80',
    NOW() - interval '10 days',
    NOW()
),
(
    'evt-104',
    'Hamilton: The Musical Spectacular',
    'Lin-Manuel Miranda''s Pulitzer-winning hip-hop and Broadway masterpiece chronicling Alexander Hamilton''s rise and revolutionary impact.',
    'Theater & Arts',
    'Victoria Palace Theatre',
    'London, United Kingdom',
    '2026-11-15 19:30:00+00',
    '2026-11-15 22:15:00+00',
    'ACTIVE',
    'https://images.unsplash.com/photo-1507676184212-d03ab07a01bf?auto=format&fit=crop&w=1200&q=80',
    NOW() - interval '9 days',
    NOW()
),
(
    'evt-105',
    'Tokyo Cyberpunk Synth & Anime Live',
    'Future funk, anime soundtracks, and retro 80s synth wave featuring live Japanese holographic vocaloids and international guest electronic artists.',
    'Music & Concerts',
    'Nippon Budokan',
    'Tokyo, Japan',
    '2026-11-20 18:00:00+00',
    '2026-11-20 21:30:00+00',
    'ACTIVE',
    'https://images.unsplash.com/photo-1503899036084-c55cdd92da26?auto=format&fit=crop&w=1200&q=80',
    NOW() - interval '8 days',
    NOW()
),
(
    'evt-106',
    'Stand-Up Comedy All-Stars Showcase',
    'Intimate comedy club night with surprise Netflix and HBO headliners delivering unfiltered sets in the historic Greenwich Village basement.',
    'Comedy & Entertainment',
    'The Comedy Cellar',
    'New York, USA',
    '2026-11-25 21:00:00+00',
    '2026-11-25 23:00:00+00',
    'ACTIVE',
    'https://images.unsplash.com/photo-1585699324551-f6c309eedeca?auto=format&fit=crop&w=1200&q=80',
    NOW() - interval '7 days',
    NOW()
),
(
    'evt-107',
    'Microservices & Distributed Systems Masterclass',
    'Deep dive into Go, Docker, Saga orchestration, and Redis distributed locks for senior software architects and lead engineers.',
    'Workshops & Education',
    'Tech Hub Innovation Lab',
    'Toronto, Canada',
    '2026-12-01 10:00:00+00',
    '2026-12-01 17:00:00+00',
    'PAUSED',
    'https://images.unsplash.com/photo-1531482615713-2afd69097998?auto=format&fit=crop&w=1200&q=80',
    NOW() - interval '6 days',
    NOW()
),
(
    'evt-108',
    'Rock Fest Summer Retrospective',
    'Annual 3-day outdoor summer festival with alternative rock, indie bands, seaside sunset stages, and gourmet food trucks.',
    'Music & Concerts',
    'Parc del Fòrum',
    'Barcelona, Spain',
    '2026-12-10 16:00:00+00',
    '2026-12-12 23:59:00+00',
    'COMPLETED',
    'https://images.unsplash.com/photo-1459749411175-04bf5292ceea?auto=format&fit=crop&w=1200&q=80',
    NOW() - interval '60 days',
    NOW()
),
(
    'evt-109',
    'Midnight Electronic Carnival (Rainout)',
    'Cancelled due to severe tropical gale warning and localized flooding. All orders refunded and inventory returned to organizer vault.',
    'Music & Concerts',
    'Harbour Amphitheater',
    'Sydney, Australia',
    '2026-12-15 20:00:00+00',
    '2026-12-16 02:00:00+00',
    'CANCELLED',
    'https://images.unsplash.com/photo-1514525253161-7a46d19cd819?auto=format&fit=crop&w=1200&q=80',
    NOW() - interval '20 days',
    NOW()
);

-- ============================================================================
-- 3. TICKET TIERS
-- ============================================================================
INSERT INTO ticket_tiers (id, event_id, name, price, capacity, created_at)
VALUES
('tier-vip-101', 'evt-101', 'VIP Front Row', 150.00, 20, NOW() - interval '14 days'),
('tier-gen-101', 'evt-101', 'General Floor', 60.00, 30, NOW() - interval '14 days'),
('tier-balcony-101', 'evt-101', 'Balcony Suite', 45.00, 20, NOW() - interval '14 days'),
('tier-keynote-102', 'evt-102', 'Executive Keynote Pass', 299.00, 20, NOW() - interval '12 days'),
('tier-standard-102', 'evt-102', 'Full Conference Pass', 99.00, 30, NOW() - interval '12 days'),
('tier-box-103', 'evt-103', 'Presidential Skybox', 350.00, 10, NOW() - interval '10 days'),
('tier-stands-103', 'evt-103', 'Club Grandstand', 95.00, 30, NOW() - interval '10 days'),
('tier-orch-104', 'evt-104', 'Royal Stalls (Orchestra)', 180.00, 20, NOW() - interval '9 days'),
('tier-circle-104', 'evt-104', 'Grand Dress Circle', 85.00, 30, NOW() - interval '9 days'),
('tier-synth-vip-105', 'evt-105', 'Cyber VIP Arena Pass', 120.00, 20, NOW() - interval '8 days'),
('tier-synth-gen-105', 'evt-105', 'Mezzanine Pass', 50.00, 30, NOW() - interval '8 days'),
('tier-comedy-106', 'evt-106', 'Front Row Danger Zone', 75.00, 12, NOW() - interval '7 days'),
('tier-workshop-107', 'evt-107', 'Full Day Lab Pass', 240.00, 20, NOW() - interval '6 days'),
('tier-fest-108', 'evt-108', '3-Day Festival Pass', 195.00, 30, NOW() - interval '60 days'),
('tier-carnival-109', 'evt-109', 'General Admission', 80.00, 20, NOW() - interval '20 days');

-- ============================================================================
-- 4. SEATS (seat_items) - Detailed Seating Grid Layouts
-- Note: A1, A2, A3 on evt-101 are AVAILABLE to support automated E2E test suites
-- ============================================================================

-- Evt 101: Neon Symphony (A1-A4 available, A5 on hold, A10 blocked, B1-B3 booked)
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status, reservation_id, hold_expires_at)
SELECT 'A' || s, 'evt-101', 'tier-vip-101', 'A', s,
    CASE 
        WHEN s = 5 THEN 'HOLD'
        WHEN s = 10 THEN 'BLOCKED'
        ELSE 'AVAILABLE'
    END,
    CASE WHEN s = 5 THEN 'res-active-hold-01' ELSE NULL END,
    CASE WHEN s = 5 THEN NOW() + interval '10 minutes' ELSE NULL END
FROM generate_series(1, 10) AS s;

INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'B' || s, 'evt-101', 'tier-vip-101', 'B', s,
    CASE 
        WHEN s IN (1, 2, 3) THEN 'BOOKED'
        ELSE 'AVAILABLE'
    END
FROM generate_series(1, 10) AS s;

-- Row C & D (General 1-15 each = 30 seats)
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'C' || s, 'evt-101', 'tier-gen-101', 'C', s,
    CASE WHEN s IN (1, 2, 4) THEN 'BOOKED' ELSE 'AVAILABLE' END
FROM generate_series(1, 15) AS s;

INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'D' || s, 'evt-101', 'tier-gen-101', 'D', s, 'AVAILABLE'
FROM generate_series(1, 15) AS s;

-- Row E & F (Balcony 1-10 each = 20 seats)
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'E' || s, 'evt-101', 'tier-balcony-101', 'E', s,
    CASE WHEN s = 1 THEN 'BOOKED' ELSE 'AVAILABLE' END
FROM generate_series(1, 10) AS s;

INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'F' || s, 'evt-101', 'tier-balcony-101', 'F', s, 'AVAILABLE'
FROM generate_series(1, 10) AS s;

-- Evt 102: Global Tech Summit (Row K 1-20, Rows S & T 1-15)
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'K' || s, 'evt-102', 'tier-keynote-102', 'K', s,
    CASE WHEN s IN (1, 2, 3, 4, 5) THEN 'BOOKED' ELSE 'AVAILABLE' END
FROM generate_series(1, 20) AS s;

INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'S' || s, 'evt-102', 'tier-standard-102', 'S', s,
    CASE WHEN s IN (1, 2, 3) THEN 'BOOKED' ELSE 'AVAILABLE' END
FROM generate_series(1, 15) AS s;

INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'T' || s, 'evt-102', 'tier-standard-102', 'T', s, 'AVAILABLE'
FROM generate_series(1, 15) AS s;

-- Evt 103: UEFA Champions Gala
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'P' || s, 'evt-103', 'tier-box-103', 'P', s,
    CASE WHEN s IN (1, 2) THEN 'BOOKED' ELSE 'AVAILABLE' END
FROM generate_series(1, 10) AS s;

INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'G' || s, 'evt-103', 'tier-stands-103', 'G', s,
    CASE WHEN s IN (1, 2, 3, 4) THEN 'BOOKED' ELSE 'AVAILABLE' END
FROM generate_series(1, 15) AS s;

INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'H' || s, 'evt-103', 'tier-stands-103', 'H', s, 'AVAILABLE'
FROM generate_series(1, 15) AS s;

-- Evt 104: Hamilton
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'O' || s, 'evt-104', 'tier-orch-104', 'O', s,
    CASE WHEN s IN (1, 2, 3, 4) THEN 'BOOKED' ELSE 'AVAILABLE' END
FROM generate_series(1, 20) AS s;

INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'LC' || s, 'evt-104', 'tier-circle-104', 'C', s,
    CASE WHEN s IN (1, 2) THEN 'BOOKED' ELSE 'AVAILABLE' END
FROM generate_series(1, 15) AS s;

INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'LD' || s, 'evt-104', 'tier-circle-104', 'D', s, 'AVAILABLE'
FROM generate_series(1, 15) AS s;

-- Evt 105: Tokyo Cyberpunk Synth
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'V' || s, 'evt-105', 'tier-synth-vip-105', 'V', s,
    CASE WHEN s IN (1, 2, 3) THEN 'BOOKED' ELSE 'AVAILABLE' END
FROM generate_series(1, 20) AS s;

INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'M' || s, 'evt-105', 'tier-synth-gen-105', 'M', s, 'AVAILABLE'
FROM generate_series(1, 15) AS s;

INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'N' || s, 'evt-105', 'tier-synth-gen-105', 'N', s, 'AVAILABLE'
FROM generate_series(1, 15) AS s;

-- Evt 106: Comedy Cellar -> 100% SOLD OUT (12/12 BOOKED for WAITLIST testing)
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'CA' || s, 'evt-106', 'tier-comedy-106', 'A', s, 'BOOKED'
FROM generate_series(1, 12) AS s;

-- Evt 107: Masterclass (Toronto - PAUSED)
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'W' || s, 'evt-107', 'tier-workshop-107', 'W', s, 'AVAILABLE'
FROM generate_series(1, 20) AS s;

-- Evt 108: Rock Fest (Barcelona - COMPLETED)
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'R' || s, 'evt-108', 'tier-fest-108', 'R', s, 'BOOKED'
FROM generate_series(1, 30) AS s;

-- Evt 109: Carnival Rainout (Sydney - CANCELLED)
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 'SC' || s, 'evt-109', 'tier-carnival-109', 'S', s, 'AVAILABLE'
FROM generate_series(1, 20) AS s;

-- ============================================================================
-- 5. EVENT INVENTORY (Accurately computed from seat_items)
-- ============================================================================
INSERT INTO event_inventory (event_id, tier_id, total_seats, available_seats, reserved_seats, booked_seats, version)
SELECT 
    event_id,
    tier_id,
    COUNT(*) AS total_seats,
    COUNT(*) FILTER (WHERE status = 'AVAILABLE') AS available_seats,
    COUNT(*) FILTER (WHERE status = 'HOLD') AS reserved_seats,
    COUNT(*) FILTER (WHERE status = 'BOOKED') AS booked_seats,
    0
FROM seat_items
GROUP BY event_id, tier_id;

-- ============================================================================
-- 6. RESERVATIONS (Active Hold Demonstration)
-- ============================================================================
INSERT INTO reservations (reservation_id, event_id, tier_id, seat_ids, quantity, status, expires_at, created_at)
VALUES
('res-active-hold-01', 'evt-101', 'tier-vip-101', '["A5"]'::jsonb, 1, 'HOLD', NOW() + interval '10 minutes', NOW()),
('res-wl-promoted-003', 'evt-106', 'tier-comedy-106', '["CA1"]'::jsonb, 1, 'HOLD', NOW() + interval '15 minutes', NOW());

-- ============================================================================
-- 7. PROMOTIONAL VOUCHERS (10 Comprehensive Scenarios)
-- ============================================================================
INSERT INTO vouchers (id, code, discount_type, discount_value, min_spend, max_uses, current_uses, expires_at, active, created_at, updated_at)
VALUES
('vch-earlybird20', 'EARLYBIRD20', 'PERCENTAGE', 20.00, 50.00, 200, 15, NOW() + interval '90 days', true, NOW() - interval '14 days', NOW()),
('vch-vip50', 'VIP50', 'PERCENTAGE', 50.00, 100.00, 50, 10, NOW() + interval '60 days', true, NOW() - interval '10 days', NOW()),
('vch-summer15', 'SUMMER15', 'PERCENTAGE', 15.00, 30.00, 300, 8, NOW() + interval '45 days', true, NOW() - interval '5 days', NOW()),
('vch-save10', 'SAVE10', 'FIXED', 10.00, 30.00, 500, 45, NOW() + interval '120 days', true, NOW() - interval '20 days', NOW()),
('vch-welcome5', 'WELCOME5', 'FIXED', 5.00, 0.00, 1000, 120, NOW() + interval '365 days', true, NOW() - interval '30 days', NOW()),
('vch-festival25', 'FESTIVAL25', 'FIXED', 25.00, 75.00, 100, 22, NOW() + interval '60 days', true, NOW() - interval '12 days', NOW()),
('vch-corp100', 'CORP100', 'FIXED', 100.00, 300.00, 50, 5, NOW() + interval '90 days', true, NOW() - interval '7 days', NOW()),
('vch-exhausted', 'EXHAUSTED', 'PERCENTAGE', 30.00, 0.00, 10, 10, NOW() + interval '30 days', true, NOW() - interval '15 days', NOW()),
('vch-expired2025', 'EXPIRED2025', 'PERCENTAGE', 40.00, 20.00, 100, 12, '2025-01-01 00:00:00+00', true, '2024-11-01 00:00:00+00', NOW()),
('vch-inactive50', 'INACTIVE50', 'PERCENTAGE', 50.00, 0.00, 100, 0, NOW() + interval '30 days', false, NOW() - interval '10 days', NOW()),
('vch-highmin500', 'HIGHMIN500', 'FIXED', 150.00, 500.00, 20, 2, NOW() + interval '60 days', true, NOW() - interval '5 days', NOW());

-- ============================================================================
-- 8. BOOKINGS (Multi-Currency, Corporate Invoicing & Lifecycle States)
-- ============================================================================
INSERT INTO bookings (
    id, booking_reference, user_id, customer_name, customer_email, 
    event_id, event_title, tier_id, quantity, seat_ids, 
    total_amount, status, payment_status, idempotency_key, 
    voucher_code, discount_amount, currency, exchange_rate, 
    tax_rate, tax_amount, company_name, tax_id, billing_address, 
    created_at, updated_at
)
VALUES
('bkg-demo-001', 'REF-NEON-2026-001', 'usr-demo-customer', 'Alex Morgan (Demo Customer)', 'user@tickets.com', 'evt-101', 'Neon Symphony 2026', 'tier-vip-101', 1, '["B1"]'::jsonb, 162.75, 'CONFIRMED', 'PAID', 'idemp-demo-001', NULL, 0.00, 'USD', 1.0000, 0.0850, 12.75, NULL, NULL, NULL, NOW() - interval '5 days', NOW() - interval '5 days'),
('bkg-demo-002', 'REF-HAM-2026-002', 'usr-demo-customer', 'Alex Morgan (Demo Customer)', 'user@tickets.com', 'evt-104', 'Hamilton: The Musical Spectacular', 'tier-circle-104', 1, '["LC1"]'::jsonb, 102.00, 'CONFIRMED', 'PAID', 'idemp-demo-002', NULL, 0.00, 'GBP', 0.7900, 0.2000, 17.00, NULL, NULL, NULL, NOW() - interval '3 days', NOW() - interval '3 days'),
('bkg-demo-003', 'REF-CANC-2026-003', 'usr-demo-customer', 'Alex Morgan (Demo Customer)', 'user@tickets.com', 'evt-101', 'Neon Symphony 2026', 'tier-vip-101', 1, '["B2"]'::jsonb, 162.75, 'CANCELLED', 'REFUNDED', 'idemp-demo-003', NULL, 0.00, 'USD', 1.0000, 0.0850, 12.75, NULL, NULL, NULL, NOW() - interval '6 days', NOW() - interval '5 days'),
('bkg-sarah-004', 'REF-TECH-2026-004', 'usr-sarah-connor', 'Sarah Connor', 'sarah.connor@cyberdyne.io', 'evt-102', 'Global Tech Summit 2026', 'tier-keynote-102', 2, '["K1", "K2"]'::jsonb, 648.83, 'CONFIRMED', 'PAID', 'idemp-sarah-004', NULL, 0.00, 'USD', 1.0000, 0.0850, 50.83, 'Cyberdyne Systems LLC', 'US-EIN-94-3829102', '18144 El Camino Real, Suite 300, Sunnyvale, CA 94086, USA', NOW() - interval '7 days', NOW() - interval '7 days'),
('bkg-maria-005', 'REF-UEFA-2026-005', 'usr-maria-garcia', 'Maria Garcia', 'maria.garcia@madrid.es', 'evt-103', 'UEFA Champions Gala Match: Berlin vs Madrid', 'tier-box-103', 1, '["P1"]'::jsonb, 420.00, 'CONFIRMED', 'PAID', 'idemp-maria-005', NULL, 0.00, 'EUR', 0.9200, 0.2000, 70.00, 'Iberia Sports Club', 'ES-B84920182', 'Paseo de la Castellana 42, 28046 Madrid, Spain', NOW() - interval '4 days', NOW() - interval '4 days'),
('bkg-kenji-006', 'REF-TYO-2026-006', 'usr-kenji-sato', 'Kenji Sato', 'kenji.sato@tokyo-synth.jp', 'evt-105', 'Tokyo Cyberpunk Synth & Anime Live', 'tier-synth-vip-105', 2, '["V1", "V2"]'::jsonb, 40920.00, 'CONFIRMED', 'PAID', 'idemp-kenji-006', NULL, 0.00, 'JPY', 155.0000, 0.1000, 3720.00, NULL, NULL, NULL, NOW() - interval '2 days', NOW() - interval '2 days'),
('bkg-chen-007', 'REF-NEON-2026-007', 'usr-alex-chen', 'Alex Chen', 'alex.chen@technova.com', 'evt-101', 'Neon Symphony 2026', 'tier-gen-101', 2, '["C1", "C2"]'::jsonb, 104.16, 'CONFIRMED', 'PAID', 'idemp-chen-007', 'EARLYBIRD20', 24.00, 'USD', 1.0000, 0.0850, 8.16, NULL, NULL, NULL, NOW() - interval '2 days', NOW() - interval '2 days'),
('bkg-acme-008', 'REF-CORP-2026-008', 'usr-procurement-acme', 'Acme Global Procurement', 'procurement@acme-corp.com', 'evt-102', 'Global Tech Summit 2026', 'tier-keynote-102', 3, '["K3", "K4", "K5"]'::jsonb, 865.75, 'CONFIRMED', 'PAID', 'idemp-acme-008', 'CORP100', 100.00, 'USD', 1.0000, 0.0850, 68.75, 'Acme Corporation Worldwide Inc', 'US-EIN-13-5928100', '100 Innovation Way, Suite 400, New York, NY 10001, USA', NOW() - interval '1 day', NOW() - interval '1 day'),
('bkg-pend-009', 'REF-PEND-2026-009', 'usr-demo-customer', 'Alex Morgan (Demo Customer)', 'user@tickets.com', 'evt-101', 'Neon Symphony 2026', 'tier-gen-101', 1, '["C4"]'::jsonb, 65.10, 'PENDING', 'PENDING', 'idemp-pend-009', NULL, 0.00, 'USD', 1.0000, 0.0850, 5.10, NULL, NULL, NULL, NOW() - interval '15 minutes', NOW() - interval '15 minutes'),
('bkg-exp-010', 'REF-EXP-2026-010', 'usr-alex-chen', 'Alex Chen', 'alex.chen@technova.com', 'evt-103', 'UEFA Champions Gala Match: Berlin vs Madrid', 'tier-stands-103', 1, '["G1"]'::jsonb, 114.00, 'EXPIRED', 'FAILED', 'idemp-exp-010', NULL, 0.00, 'EUR', 0.9200, 0.2000, 19.00, NULL, NULL, NULL, NOW() - interval '1 day', NOW() - interval '1 day');

-- ============================================================================
-- 9. TICKETS (Digital QR Passbook & Gate Scanner Test Records)
-- ============================================================================
INSERT INTO tickets (id, booking_id, seat_id, seat_label, ticket_code, status, created_at)
VALUES
('tkt-demo-001', 'bkg-demo-001', 'B1', 'Row B, Seat 1 (VIP Front Row)', 'TKT-VALID-VIP01', 'VALID', NOW() - interval '5 days'),
('tkt-demo-002', 'bkg-demo-002', 'LC1', 'Row C, Seat 1 (Dress Circle)', 'TKT-VALID-HAM02', 'VALID', NOW() - interval '3 days'),
('tkt-demo-003', 'bkg-demo-003', 'B2', 'Row B, Seat 2 (VIP Front Row)', 'TKT-CANC-NEON03', 'CANCELLED', NOW() - interval '6 days'),
('tkt-sarah-004a', 'bkg-sarah-004', 'K1', 'Row K, Seat 1 (Keynote VIP)', 'TKT-USED-GATE99', 'USED', NOW() - interval '7 days'),
('tkt-sarah-004b', 'bkg-sarah-004', 'K2', 'Row K, Seat 2 (Keynote VIP)', 'TKT-VALID-TECH04', 'VALID', NOW() - interval '7 days'),
('tkt-maria-005', 'bkg-maria-005', 'P1', 'Row P, Seat 1 (Skybox)', 'TKT-VALID-UEFA05', 'VALID', NOW() - interval '4 days'),
('tkt-kenji-006a', 'bkg-kenji-006', 'V1', 'Row V, Seat 1 (Cyber VIP)', 'TKT-VALID-TYO06A', 'VALID', NOW() - interval '2 days'),
('tkt-kenji-006b', 'bkg-kenji-006', 'V2', 'Row V, Seat 2 (Cyber VIP)', 'TKT-VALID-TYO06B', 'VALID', NOW() - interval '2 days'),
('tkt-chen-007a', 'bkg-chen-007', 'C1', 'Row C, Seat 1 (General Floor)', 'TKT-VALID-CHEN07A', 'VALID', NOW() - interval '2 days'),
('tkt-chen-007b', 'bkg-chen-007', 'C2', 'Row C, Seat 2 (General Floor)', 'TKT-VALID-CHEN07B', 'VALID', NOW() - interval '2 days'),
('tkt-acme-008a', 'bkg-acme-008', 'K3', 'Row K, Seat 3 (Keynote VIP)', 'TKT-VALID-ACME08A', 'VALID', NOW() - interval '1 day'),
('tkt-acme-008b', 'bkg-acme-008', 'K4', 'Row K, Seat 4 (Keynote VIP)', 'TKT-VALID-ACME08B', 'VALID', NOW() - interval '1 day'),
('tkt-acme-008c', 'bkg-acme-008', 'K5', 'Row K, Seat 5 (Keynote VIP)', 'TKT-VALID-ACME08C', 'VALID', NOW() - interval '1 day');

-- ============================================================================
-- 10. WAITLIST (Sold-Out Event Waitlist Demonstrations for evt-106)
-- ============================================================================
INSERT INTO waitlist (
    id, event_id, tier_id, seat_id, user_id, 
    customer_name, customer_email, quantity, status, 
    reservation_id, promoted_at, expires_at, created_at, updated_at
)
VALUES
('wl-001', 'evt-106', 'tier-comedy-106', NULL, 'usr-sarah-connor', 'Sarah Connor', 'sarah.connor@cyberdyne.io', 2, 'PENDING', NULL, NULL, NULL, NOW() - interval '3 hours', NOW() - interval '3 hours'),
('wl-002', 'evt-106', 'tier-comedy-106', NULL, 'usr-alex-chen', 'Alex Chen', 'alex.chen@technova.com', 1, 'PENDING', NULL, NULL, NULL, NOW() - interval '2 hours', NOW() - interval '2 hours'),
('wl-003', 'evt-106', 'tier-comedy-106', 'CA1', 'usr-maria-garcia', 'Maria Garcia', 'maria.garcia@madrid.es', 1, 'PROMOTED', 'res-wl-promoted-003', NOW() - interval '5 minutes', NOW() + interval '15 minutes', NOW() - interval '4 hours', NOW() - interval '5 minutes'),
('wl-004', 'evt-106', 'tier-comedy-106', NULL, 'usr-demo-customer', 'Alex Morgan (Demo Customer)', 'user@tickets.com', 1, 'EXPIRED', NULL, NOW() - interval '24 hours', NOW() - interval '23 hours', NOW() - interval '2 days', NOW() - interval '23 hours');

-- ============================================================================
-- 11. NOTIFICATIONS (In-App Notification Demonstrations for Milestone 28)
-- ============================================================================
INSERT INTO notifications (id, user_id, recipient_email, type, title, message, data, is_read, created_at)
VALUES
('notif-demo-001', 'usr-demo-customer', 'user@tickets.com', 'BOOKING_CONFIRMED', 'Booking Confirmed!', 'Your order #REF-NEON-2026-001 for Neon Symphony 2026 is confirmed.', '{"bookingId":"bkg-demo-001","eventId":"evt-101"}'::jsonb, false, NOW() - interval '2 hours'),
('notif-demo-002', 'usr-demo-customer', 'user@tickets.com', 'SYSTEM_ALERT', 'Venue Gates Open at 6:00 PM', 'Grand Arena gates open early tonight for Neon Symphony 2026. Please have your QR ticket pass ready.', '{"eventId":"evt-101"}'::jsonb, false, NOW() - interval '1 hour'),
('notif-maria-003', 'usr-maria-garcia', 'maria.garcia@madrid.es', 'WAITLIST_PROMOTED', 'You Are Off The Waitlist!', 'A front-row seat opened up for Stand-Up Comedy All-Stars. You have 15 minutes to claim your hold.', '{"eventId":"evt-106","reservationId":"res-wl-promoted-003"}'::jsonb, false, NOW() - interval '5 minutes'),
('notif-sarah-004', 'usr-sarah-connor', 'sarah.connor@cyberdyne.io', 'BOOKING_CONFIRMED', 'Corporate Invoice Ready', 'Your corporate B2B tax invoice for Global Tech Summit 2026 is ready for download.', '{"bookingId":"bkg-sarah-004","eventId":"evt-102"}'::jsonb, true, NOW() - interval '1 day');

-- ============================================================================
-- 12. EVENT ADD-ONS (VIP Add-ons, Merchandise & Concessions for Milestone 29)
-- ============================================================================
INSERT INTO event_addons (id, event_id, name, description, price, category, total_inventory, available_inventory, active)
VALUES
('addon-parking-101', 'evt-101', 'VIP Reserved Parking Pass', 'Guaranteed spot in the covered underground arena deck with express exit lane.', 25.00, 'PARKING', 100, 95, true),
('addon-tshirt-101', 'evt-101', 'Neon Symphony Tour T-Shirt (L)', 'Limited edition heavyweight cotton tour graphic tee with glow-in-the-dark print.', 35.00, 'MERCHANDISE', 200, 185, true),
('addon-lounge-101', 'evt-101', 'Backstage VIP Lounge & Open Bar', 'Pre-show access to private lounge, curated hors doeuvres, and craft cocktails.', 55.00, 'HOSPITALITY', 50, 42, true),
('addon-keynote-102', 'evt-102', 'Executive Networking Luncheon', 'Private seated luncheon with keynote tech speakers and venture partners.', 75.00, 'HOSPITALITY', 60, 50, true),
('addon-merch-102', 'evt-102', 'Conference Swag Pack & Hoodie', 'Premium technical fleece hoodie, RFID cardholder, and conference notebook.', 40.00, 'MERCHANDISE', 150, 130, true),
('addon-buffet-103', 'evt-103', 'Champions Matchday Buffet', 'Gourmet matchday dining buffet in the Olympiastadion stadium sky lounge.', 65.00, 'CONCESSIONS', 80, 70, true),
('addon-champagne-104', 'evt-104', 'Intermission Champagne Service', 'Chilled Moët & Chandon champagne delivered to your seat during intermission.', 30.00, 'CONCESSIONS', 100, 88, true),
('addon-poster-105', 'evt-105', 'Holographic Anime Foil Poster', 'Numbered holographic commemorative art print signed by the synth producers.', 28.00, 'MERCHANDISE', 120, 110, true);

-- ============================================================================
-- 13. ATTENDEE REVIEWS & RATINGS (Social Proof Matrix for Milestone 30)
-- ============================================================================
INSERT INTO reviews (id, event_id, user_id, reviewer_name, reviewer_email, rating, title, comment, verified_buyer, helpful_votes, created_at)
VALUES
('rev-101-001', 'evt-101', 'usr-demo-customer', 'Alex Morgan', 'user@tickets.com', 5, 'Unbelievable audio fidelity and lighting!', 'Row A stage-front was an unforgettable sensory experience. Laser show synchronized to the orchestral drops flawlessly.', true, 24, NOW() - interval '2 days'),
('rev-101-002', 'evt-101', 'usr-maria-garcia', 'Maria Garcia', 'maria.garcia@madrid.es', 5, 'Seamless digital ticket entry and pristine venue', 'Fast check-in via Apple Wallet passbook pass. Sound engineering in the Grand Hall was second to none.', true, 14, NOW() - interval '1 day'),
('rev-101-003', 'evt-101', 'usr-alex-chen', 'Alex Chen', 'alex.chen@technova.com', 4, 'Masterclass musicianship, minor queue at concessions', 'The performance blew the roof off. Highly recommend grabbing VIP express parking add-on to skip arena congestion.', true, 8, NOW() - interval '12 hours'),
('rev-102-001', 'evt-102', 'usr-sarah-connor', 'Sarah Connor', 'sarah.connor@cyberdyne.io', 5, 'Best enterprise AI keynote in years', 'Fascinating keynote panels on agentic workflows and real-time distributed systems. Great networking reception.', true, 31, NOW() - interval '3 days'),
('rev-103-001', 'evt-103', 'usr-kenji-sato', 'Kenji Sato', 'kenji.sato@tokyo-synth.jp', 5, 'Atmosphere was electric!', 'Unmatched matchday atmosphere. The VIP skybox hospitality and catering exceeded all expectations.', true, 19, NOW() - interval '4 days'),
('rev-104-001', 'evt-104', 'usr-demo-customer', 'Alex Morgan', 'user@tickets.com', 5, 'Breathtaking choreography and orchestra', 'Hamilton was pure poetry in motion. The live philharmonic was mesmerizing.', true, 16, NOW() - interval '2 days');

-- ============================================================================
-- 14. NOTIFICATION DELIVERIES (Multi-Channel Delivery Audit Logs for Milestone 32)
-- ============================================================================
INSERT INTO notification_deliveries (id, channel, recipient, event_type, subject, body, status, attempts, created_at)
VALUES
('del-seed-001', 'EMAIL', 'user@tickets.com', 'booking.confirmed', 'Booking Confirmed: Neon Symphony 2026 (Ref: REF-NEON-2026-001)', '<h2>Order Confirmed!</h2><p>Dear Alex Morgan, your reservation is confirmed.</p>', 'DELIVERED', 1, NOW() - interval '2 hours'),
('del-seed-002', 'SMS', '+15551234567', 'waitlist.promoted', 'Transactional SMS', 'TicketVerse: Waitlist hold available for Cyberpunk Symphony! Claim reservation within 15 mins: ref WL-101-PROMOTE', 'DELIVERED', 1, NOW() - interval '30 minutes'),
('del-seed-003', 'EMAIL', 'sarah.connor@cyberdyne.io', 'booking.confirmed', 'Booking Confirmed: Global Tech Summit 2026 (Ref: REF-TECH-2026-004)', '<h2>Order Confirmed!</h2><p>Dear Sarah Connor, your corporate seats are confirmed.</p>', 'DELIVERED', 1, NOW() - interval '1 day'),
('del-seed-004', 'EMAIL', 'friend@technova.com', 'ticket.transferred', 'Ticket Transfer Confirmation: Neon Symphony 2026', '<h2>Ticket Transferred</h2><p>Ticket for Neon Symphony 2026 has been transferred successfully.</p>', 'DELIVERED', 1, NOW() - interval '4 hours');
