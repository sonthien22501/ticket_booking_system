#!/usr/bin/env bash
# ==============================================================================
# Ticket Booking System - Test Dataset Loader & Verification Script
# Seeds 100% of features into PostgreSQL & invalidates Redis caches.
# ==============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
SQL_FILE="${SCRIPT_DIR}/seed_test_dataset.sql"

echo "======================================================================"
echo "🚀 Seeding Master Test Dataset into Ticket Booking System"
echo "======================================================================"

if [ ! -f "${SQL_FILE}" ]; then
    echo "❌ Error: SQL seed file not found at ${SQL_FILE}"
    exit 1
fi

# 1. Execute SQL script via Docker or local psql
if docker ps --format '{{.Names}}' | grep -q "ticket_postgres"; then
    echo "📦 Detected running 'ticket_postgres' Docker container."
    echo "⏳ Applying ${SQL_FILE} to database 'ticket_db'..."
    docker exec -i ticket_postgres psql -U postgres -d ticket_db < "${SQL_FILE}" > /dev/null
    echo "✅ Database schema & seed records applied successfully."
else
    echo "⚠️ Docker container 'ticket_postgres' not detected. Attempting local psql on port 5433 / 5432..."
    PGPASSWORD=postgrespassword psql -h localhost -p 5433 -U postgres -d ticket_db -f "${SQL_FILE}" > /dev/null 2>&1 || \
    PGPASSWORD=postgrespassword psql -h localhost -p 5432 -U postgres -d ticket_db -f "${SQL_FILE}" > /dev/null
    echo "✅ Applied via local psql."
fi

# 2. Invalidate Redis Caches
if docker ps --format '{{.Names}}' | grep -q "ticket_redis"; then
    echo "🔄 Invalidating Redis catalog and rate-limit caches..."
    docker exec ticket_redis redis-cli FLUSHALL > /dev/null
    echo "✅ Redis cache cleared."
fi

# 3. Print Row Counts from Database
echo ""
echo "📊 Current Database Record Counts:"
echo "----------------------------------------------------------------------"
if docker ps --format '{{.Names}}' | grep -q "ticket_postgres"; then
    docker exec ticket_postgres psql -U postgres -d ticket_db -t -c "
        SELECT 'Users:           ' || count(*) FROM users
        UNION ALL SELECT 'Events:          ' || count(*) FROM events
        UNION ALL SELECT 'Ticket Tiers:    ' || count(*) FROM ticket_tiers
        UNION ALL SELECT 'Seat Items:      ' || count(*) FROM seat_items
        UNION ALL SELECT 'Reservations:    ' || count(*) FROM reservations
        UNION ALL SELECT 'Bookings:        ' || count(*) FROM bookings
        UNION ALL SELECT 'Digital Tickets: ' || count(*) FROM tickets
        UNION ALL SELECT 'Waitlist Queue:  ' || count(*) FROM waitlist
        UNION ALL SELECT 'Vouchers:        ' || count(*) FROM vouchers
        UNION ALL SELECT 'Notifications:   ' || count(*) FROM notifications;
    " | sed '/^$/d'
fi

# 4. Verify Gateway Endpoints
echo ""
echo "🔍 Verifying Core Gateway APIs (http://localhost:8080):"
echo "----------------------------------------------------------------------"
GATEWAY_URL="http://localhost:8080"

check_endpoint() {
    local name="$1"
    local url="$2"
    if curl -s -f -o /dev/null -w "%{http_code}" "${url}" | grep -qE "^(200|204)$"; then
        echo "  ✅ ${name}: OK (${url})"
    else
        echo "  ⚠️ ${name}: Non-200 response (${url})"
    fi
}

check_endpoint "Gateway Health" "${GATEWAY_URL}/health"
check_endpoint "Event Catalog" "${GATEWAY_URL}/api/events"
check_endpoint "Currencies & Tax Rates" "${GATEWAY_URL}/api/currencies/rates"
check_endpoint "Voucher Engine" "${GATEWAY_URL}/api/admin/vouchers"
check_endpoint "Admin Telemetry" "${GATEWAY_URL}/api/admin/analytics"

echo ""
echo "======================================================================"
echo "🎉 Master Test Dataset is fully loaded and ready!"
echo "📖 See docs/TEST_DATASET_GUIDE.md for detailed testing instructions."
echo "======================================================================"
