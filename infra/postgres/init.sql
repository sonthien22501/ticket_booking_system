-- Ticket Booking System PostgreSQL Initialization & Seed Script

-- 1. Schema Definition

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
    status VARCHAR(32) NOT NULL DEFAULT 'AVAILABLE', -- AVAILABLE, HOLD, BOOKED, BLOCKED
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
    status VARCHAR(32) NOT NULL, -- HOLD, COMMITTED, RELEASED, EXPIRED
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
    status VARCHAR(32) NOT NULL DEFAULT 'PENDING', -- PENDING, CONFIRMED, CANCELLED, EXPIRED
    reservation_id VARCHAR(64),
    payment_status VARCHAR(32) NOT NULL DEFAULT 'PENDING',
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

-- Index creation for high-performance lookup
CREATE INDEX IF NOT EXISTS idx_seat_items_event_status ON seat_items(event_id, status);
CREATE INDEX IF NOT EXISTS idx_seat_items_hold_expires ON seat_items(status, hold_expires_at) WHERE status = 'HOLD';
CREATE INDEX IF NOT EXISTS idx_bookings_user ON bookings(user_id);
CREATE INDEX IF NOT EXISTS idx_bookings_email ON bookings(customer_email);
CREATE INDEX IF NOT EXISTS idx_reservations_status_expires ON reservations(status, expires_at);

-- 2. Seed Data

-- Clear existing data if re-running
TRUNCATE TABLE tickets CASCADE;
TRUNCATE TABLE bookings CASCADE;
TRUNCATE TABLE reservations CASCADE;
TRUNCATE TABLE seat_items CASCADE;
TRUNCATE TABLE event_inventory CASCADE;
TRUNCATE TABLE ticket_tiers CASCADE;
TRUNCATE TABLE events CASCADE;

-- Insert Events
INSERT INTO events (id, title, description, category, venue_name, venue_location, start_time, end_time, status, image_url)
VALUES 
(
    'evt-101',
    'Neon Symphony 2026',
    'An immersive electronic and orchestral live performance featuring world-renowned composers, synthesizers, and light projections.',
    'Music & Concerts',
    'Grand Arena Berlin',
    'Berlin, Germany',
    '2026-11-15 20:00:00+00',
    '2026-11-15 23:00:00+00',
    'ACTIVE',
    'https://images.unsplash.com/photo-1470225620780-dba8ba36b745?auto=format&fit=crop&w=1200&q=80'
),
(
    'evt-102',
    'Global Tech Summit 2026',
    'The premier developer, AI, and cloud architecture summit bringing together tech leaders, keynote talks, and hands-on workshops.',
    'Technology & Conferences',
    'Metropolitan Convention Center',
    'San Francisco, USA',
    '2026-12-01 09:00:00+00',
    '2026-12-03 18:00:00+00',
    'ACTIVE',
    'https://images.unsplash.com/photo-1540575467063-178a50c2df87?auto=format&fit=crop&w=1200&q=80'
);

-- Insert Ticket Tiers
INSERT INTO ticket_tiers (id, event_id, name, price, capacity)
VALUES
('tier-vip', 'evt-101', 'VIP Premium', 150.00, 50),
('tier-gen', 'evt-101', 'General Admission', 60.00, 100),
('tier-keynote', 'evt-102', 'Keynote Pass', 299.00, 40),
('tier-standard', 'evt-102', 'Standard Pass', 99.00, 80);

-- Insert Event Inventory
INSERT INTO event_inventory (event_id, tier_id, total_seats, available_seats, reserved_seats, booked_seats, version)
VALUES
('evt-101', 'tier-vip', 50, 50, 0, 0, 0),
('evt-101', 'tier-gen', 100, 100, 0, 0, 0),
('evt-102', 'tier-keynote', 40, 40, 0, 0, 0),
('evt-102', 'tier-standard', 80, 80, 0, 0, 0);

-- Insert Seats for Event 1 (evt-101): VIP Seats A1 - A50
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 
    'A' || s,
    'evt-101',
    'tier-vip',
    'A',
    s,
    'AVAILABLE'
FROM generate_series(1, 50) AS s;

-- Insert Seats for Event 1 (evt-101): General Seats B1 - B100
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 
    'B' || s,
    'evt-101',
    'tier-gen',
    'B',
    s,
    'AVAILABLE'
FROM generate_series(1, 100) AS s;

-- Insert Seats for Event 2 (evt-102): Keynote Seats K1 - K40
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 
    'K' || s,
    'evt-102',
    'tier-keynote',
    'K',
    s,
    'AVAILABLE'
FROM generate_series(1, 40) AS s;

-- Insert Seats for Event 2 (evt-102): Standard Seats S1 - S80
INSERT INTO seat_items (id, event_id, tier_id, seat_row, seat_number, status)
SELECT 
    'S' || s,
    'evt-102',
    'tier-standard',
    'S',
    s,
    'AVAILABLE'
FROM generate_series(1, 80) AS s;
