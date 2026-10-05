#!/usr/bin/env python3
"""
Adversarial Concurrency & Anti-Oversell Stress Test Suite
Challenger 1 — Empirical Verification of Seat Inventory Reservation Mechanism

This suite empirically stress-tests:
  1. Massive Single-Seat Contention (50 concurrent threads via Barrier)
  2. Multi-Seat Overlapping Contention & Deadlock Resistance (30 concurrent threads)
  3. Complete Capacity Exhaustion Race (80 concurrent threads for 40-seat tier)
  4. Post-Sellout Rejection & Negative Inventory Invariant
  5. Short-TTL Lease Expiration Janitor & Re-sale Race
  6. Payment Failure Saga Compensation & Seat Recycling Race
  7. Partial Seat Unavailability Atomic Rollback (All-or-Nothing Invariant)
  8. Cross-Tier Concurrency Independence
  9. High-Frequency Rapid Booking Burst (40 sequential bookings)
 10. Direct PostgreSQL Database Oracle Invariant Verification
"""

import argparse
import json
import os
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, List, Optional, Tuple


USE_COLOR = sys.stdout.isatty() and os.environ.get("NO_COLOR") is None
GREEN = "\033[92m" if USE_COLOR else ""
RED = "\033[91m" if USE_COLOR else ""
YELLOW = "\033[93m" if USE_COLOR else ""
BLUE = "\033[94m" if USE_COLOR else ""
CYAN = "\033[96m" if USE_COLOR else ""
BOLD = "\033[1m" if USE_COLOR else ""
RESET = "\033[0m" if USE_COLOR else ""


class TestContext:
    def __init__(self, gateway_url: str):
        self.gateway_url = gateway_url.rstrip("/")
        self.passed = 0
        self.failed = 0
        self.errors: List[str] = []

    def record_pass(self, name: str, detail: str = ""):
        self.passed += 1
        print(f" {GREEN}✓ PASS{RESET} {BOLD}{name}{RESET}" + (f" — {detail}" if detail else ""))

    def record_fail(self, name: str, error: str):
        self.failed += 1
        self.errors.append(f"{name}: {error}")
        print(f" {RED}✗ FAIL{RESET} {BOLD}{name}{RESET} — {RED}{error}{RESET}")

    def log_info(self, msg: str):
        print(f" {BLUE}ℹ INFO{RESET} {msg}")

    def log_section(self, title: str):
        print(f"\n{CYAN}{'=' * 70}{RESET}")
        print(f"{CYAN}{BOLD} {title}{RESET}")
        print(f"{CYAN}{'=' * 70}{RESET}")


def http_request(
    url: str,
    method: str = "GET",
    payload: Optional[Dict[str, Any]] = None,
    headers: Optional[Dict[str, str]] = None,
    timeout: float = 10.0,
) -> Tuple[int, Any]:
    req_headers = {"Accept": "application/json"}
    if headers:
        req_headers.update(headers)

    data_bytes = None
    if payload is not None:
        data_bytes = json.dumps(payload).encode("utf-8")
        req_headers["Content-Type"] = "application/json"

    req = urllib.request.Request(url, data=data_bytes, headers=req_headers, method=method)

    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            status = resp.status
            body_bytes = resp.read()
            body_str = body_bytes.decode("utf-8", errors="replace")
            try:
                res_data = json.loads(body_str) if body_str else {}
            except Exception:
                res_data = body_str
            return status, res_data
    except urllib.error.HTTPError as e:
        body_bytes = e.read()
        body_str = body_bytes.decode("utf-8", errors="replace")
        try:
            res_data = json.loads(body_str) if body_str else {}
        except Exception:
            res_data = body_str
        return e.code, res_data
    except Exception as e:
        return 599, {"error": str(e)}


def query_db_oracle() -> List[Dict[str, Any]]:
    cmd = [
        "docker", "exec", "ticket_postgres", "psql", "-U", "postgres", "-d", "ticket_db", "-t", "-A", "-F", "|",
        "-c",
        """
        SELECT 
            ei.event_id, 
            ei.tier_id, 
            ei.total_seats, 
            ei.available_seats, 
            ei.reserved_seats, 
            ei.booked_seats, 
            (ei.available_seats + ei.reserved_seats + ei.booked_seats = ei.total_seats) AS sum_ok, 
            (ei.available_seats >= 0) AS non_neg, 
            COUNT(CASE WHEN s.status = 'BOOKED' THEN 1 END) AS act_booked, 
            COUNT(CASE WHEN s.status = 'HOLD' THEN 1 END) AS act_hold, 
            COUNT(CASE WHEN s.status = 'AVAILABLE' THEN 1 END) AS act_avail 
        FROM event_inventory ei 
        LEFT JOIN seat_items s ON ei.event_id = s.event_id AND ei.tier_id = s.tier_id 
        GROUP BY ei.event_id, ei.tier_id, ei.total_seats, ei.available_seats, ei.reserved_seats, ei.booked_seats
        ORDER BY ei.event_id, ei.tier_id;
        """
    ]
    res = subprocess.run(cmd, capture_output=True, text=True, check=True)
    rows = []
    for line in res.stdout.strip().splitlines():
        if not line or "|" not in line:
            continue
        parts = line.split("|")
        rows.append({
            "event_id": parts[0],
            "tier_id": parts[1],
            "total_seats": int(parts[2]),
            "available_seats": int(parts[3]),
            "reserved_seats": int(parts[4]),
            "booked_seats": int(parts[5]),
            "sum_ok": parts[6] == "t",
            "non_neg": parts[7] == "t",
            "act_booked": int(parts[8]),
            "act_hold": int(parts[9]),
            "act_avail": int(parts[10]),
        })
    return rows


# ==============================================================================
# Adversarial Challenge Tests
# ==============================================================================

def test_1_massive_single_seat_contention(ctx: TestContext):
    """
    Stress 50 simultaneous threads contending for exact same seat A10.
    Barrier synchronization guarantees zero stagger.
    """
    ctx.log_section("CHALLENGE 1: Massive Single-Seat Contention (50 Concurrent Threads)")
    event_id = "evt-101"
    seat = "A10"
    num_threads = 50

    ctx.log_info(f"Firing {num_threads} simultaneous threads for seat '{seat}' on '{event_id}' via Barrier...")
    barrier = threading.Barrier(num_threads)
    results: List[Tuple[int, Any]] = [None] * num_threads

    def worker(idx: int):
        payload = {
            "eventId": event_id,
            "seats": [seat],
            "ticketCount": 1,
            "customerName": f"Massive Racer {idx}",
            "customerEmail": f"massive_racer_{idx}@example.com",
            "userId": f"usr_massive_{idx}"
        }
        barrier.wait()
        status, data = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=payload)
        results[idx] = (status, data)

    threads = [threading.Thread(target=worker, args=(i,)) for i in range(num_threads)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    wins = [r for r in results if r[0] == 201]
    conflicts = [r for r in results if r[0] == 409]
    others = [r for r in results if r[0] not in (201, 409)]

    ctx.log_info(f"Outcomes: 201 Created = {len(wins)}, 409 Conflict = {len(conflicts)}, Other = {len(others)}")

    if len(wins) == 1 and len(conflicts) == num_threads - 1 and len(others) == 0:
        booking = wins[0][1]
        ctx.record_pass(
            "1.1 Single Winner Assertion",
            f"Exactly 1 winner (bookingId: {booking.get('id')}), {len(conflicts)} clean 409s, 0 errors."
        )
    else:
        ctx.record_fail(
            "1.1 Single Winner Assertion",
            f"Expected exactly 1 win & {num_threads-1} conflicts, got {len(wins)} wins, {len(conflicts)} conflicts, {len(others)} others: {others}"
        )


def test_2_overlapping_seat_contention(ctx: TestContext):
    """
    Stress 28 threads contending for overlapping seat pairs and reversed order.
    Checks all-or-nothing atomicity and deadlock prevention.
    """
    ctx.log_section("CHALLENGE 2: Overlapping Multi-Seat Contention & Deadlock Resistance")
    event_id = "evt-101"

    seat_requests = [
        ["A15", "A16"],
        ["A16", "A17"],
        ["A17", "A18"],
        ["A18", "A19"],
        ["A19", "A20"],
        ["A20", "A15"],
        ["A16", "A15"],
        ["A17", "A16"],
        ["A18", "A17"],
        ["A19", "A18"],
        ["A20", "A19"],
        ["A15", "A20"],
        ["A15", "A16", "A17"],
        ["A17", "A18", "A19"],
        ["A18", "A19", "A20"],
        ["A16", "A18", "A20"],
        ["A15"], ["A16"], ["A17"], ["A18"], ["A19"], ["A20"],
        ["A15", "A16"],
        ["A17", "A18"],
        ["A19", "A20"],
        ["A16", "A17"],
        ["A18", "A19"],
        ["A15", "A17"],
    ]
    num_threads = len(seat_requests)
    ctx.log_info(f"Firing {num_threads} simultaneous threads requesting overlapping seats in range A15..A20...")

    barrier = threading.Barrier(num_threads)
    results: List[Tuple[int, Any, List[str]]] = [None] * num_threads

    def worker(idx: int):
        seats = seat_requests[idx]
        payload = {
            "eventId": event_id,
            "seats": seats,
            "ticketCount": len(seats),
            "customerName": f"Overlap Racer {idx}",
            "customerEmail": f"overlap_{idx}@example.com",
            "userId": f"usr_overlap_{idx}"
        }
        barrier.wait()
        status, data = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=payload)
        results[idx] = (status, data, seats)

    threads = [threading.Thread(target=worker, args=(i,)) for i in range(num_threads)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    wins = [r for r in results if r[0] == 201]
    conflicts = [r for r in results if r[0] == 409]
    others = [r for r in results if r[0] not in (201, 409)]

    ctx.log_info(f"Overlap outcomes: 201 Created = {len(wins)}, 409 Conflict = {len(conflicts)}, Other = {len(others)}")

    if len(others) == 0:
        ctx.record_pass("2.1 Deadlock Resistance", "Zero HTTP 500s or lock deadlock failures detected.")
    else:
        ctx.record_fail("2.1 Deadlock Resistance", f"Encountered non-409/non-201 failures: {others}")

    claimed_seats = set()
    collision_detected = False
    for status, booking, req_seats in wins:
        confirmed_seats = booking.get("seatIds") or booking.get("seats") or req_seats
        for s in confirmed_seats:
            if s in claimed_seats:
                collision_detected = True
                ctx.record_fail("2.2 Mutual Exclusion Invariant", f"Seat '{s}' was double-booked across winners!")
            claimed_seats.add(s)

    if not collision_detected:
        ctx.record_pass(
            "2.2 Mutual Exclusion Invariant",
            f"All {len(claimed_seats)} booked seats are strictly mutually disjoint across {len(wins)} winning bookings."
        )


def test_3_complete_capacity_exhaustion_race(ctx: TestContext):
    """
    Stress 80 concurrent threads auto-allocating 1 seat each on a 40-seat tier (tier-keynote on evt-102).
    Asserts zero overselling, exactly 40 winners, exactly 40 rejections, remaining = 0.
    """
    ctx.log_section("CHALLENGE 3: Complete Capacity Exhaustion Race (80 Threads for 40 Seats)")
    event_id = "evt-102"
    tier_id = "tier-keynote"
    tier_capacity = 40
    num_threads = 80

    ctx.log_info(f"Firing {num_threads} simultaneous threads for {tier_capacity} seats on tier '{tier_id}'...")
    barrier = threading.Barrier(num_threads)
    results: List[Tuple[int, Any]] = [None] * num_threads

    def worker(idx: int):
        payload = {
            "eventId": event_id,
            "tierId": tier_id,
            "ticketCount": 1,
            "customerName": f"Stampede User {idx}",
            "customerEmail": f"stampede_{idx}@example.com",
            "userId": f"usr_stampede_{idx}"
        }
        barrier.wait()
        status, data = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=payload)
        results[idx] = (status, data)

    threads = [threading.Thread(target=worker, args=(i,)) for i in range(num_threads)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    wins = [r for r in results if r[0] == 201]
    conflicts = [r for r in results if r[0] == 409]
    others = [r for r in results if r[0] not in (201, 409)]

    ctx.log_info(f"Exhaustion race outcomes: 201 Created = {len(wins)}, 409 Conflict = {len(conflicts)}, Other = {len(others)}")

    if len(wins) == tier_capacity:
        ctx.record_pass(
            "3.1 Zero-Oversell Bound",
            f"Exactly {tier_capacity} bookings confirmed, zero overselling beyond tier capacity."
        )
    else:
        ctx.record_fail(
            "3.1 Zero-Oversell Bound",
            f"Expected exactly {tier_capacity} wins, got {len(wins)}! Overselling or premature rejection."
        )

    if len(conflicts) == num_threads - tier_capacity:
        ctx.record_pass(
            "3.2 Rejection Count Assertion",
            f"All {len(conflicts)} excess requests received HTTP 409 Conflict."
        )
    else:
        ctx.record_fail(
            "3.2 Rejection Count Assertion",
            f"Expected {num_threads - tier_capacity} conflicts, got {len(conflicts)} (others: {len(others)})"
        )


def test_4_sold_out_rejection_and_non_negative_check(ctx: TestContext):
    """
    Verify immediate subsequent booking attempts against sold out tier are blocked
    and available_seats remains strictly non-negative.
    """
    ctx.log_section("CHALLENGE 4: Sold-Out Rejection & Negative Inventory Prevention")
    event_id = "evt-102"
    tier_id = "tier-keynote"

    status, inv = http_request(f"{ctx.gateway_url}/api/inventory/{event_id}")
    avail = None
    for t in inv.get("tiers", []):
        if t.get("tier_id") == tier_id or t.get("tierId") == tier_id:
            avail = t.get("available_seats") if "available_seats" in t else t.get("availableSeats")

    ctx.log_info(f"Current available seats for {tier_id}: {avail}")

    if avail != 0:
        ctx.record_fail("4.1 Tier Sold Out Status", f"Expected available seats to be 0, got {avail}")
        return

    ctx.record_pass("4.1 Tier Sold Out Status", "Tier confirmed completely sold out (0 available seats).")

    ctx.log_info("Firing 10 concurrent requests to probe sold out tier...")
    num_probes = 10
    barrier = threading.Barrier(num_probes)
    probe_results = [None] * num_probes

    def worker(idx: int):
        payload = {
            "eventId": event_id,
            "tierId": tier_id,
            "ticketCount": 1,
            "customerName": f"Probe User {idx}",
            "customerEmail": f"probe_{idx}@example.com",
        }
        barrier.wait()
        st, d = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=payload)
        probe_results[idx] = (st, d)

    threads = [threading.Thread(target=worker, args=(i,)) for i in range(num_probes)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    all_409 = all(r[0] == 409 for r in probe_results)
    if all_409:
        ctx.record_pass("4.2 Sold-Out 409 Rejection", f"All {num_probes}/{num_probes} requests rejected with HTTP 409 Conflict.")
    else:
        ctx.record_fail("4.2 Sold-Out 409 Rejection", f"Some requests did not return 409: {probe_results}")

    _, inv_after = http_request(f"{ctx.gateway_url}/api/inventory/{event_id}")
    avail_after = None
    for t in inv_after.get("tiers", []):
        if t.get("tier_id") == tier_id or t.get("tierId") == tier_id:
            avail_after = t.get("available_seats") if "available_seats" in t else t.get("availableSeats")

    if avail_after == 0:
        ctx.record_pass("4.3 Strict Non-Negative Inventory", "Available inventory remained strictly 0 (never negative).")
    else:
        ctx.record_fail("4.3 Strict Non-Negative Inventory", f"Available inventory corrupted: {avail_after}")


def test_5_ttl_expiration_janitor_race(ctx: TestContext):
    """
    Reserve seat with 1s TTL.
    Wait for background Janitor to sweep expired lease.
    Assert expired commit is rejected and reclaimed seat is re-sold cleanly.
    """
    ctx.log_section("CHALLENGE 5: Short-TTL Janitor Sweep & Re-Sale Race")
    event_id = "evt-101"
    seat = "B50"

    reserve_payload = {
        "eventId": event_id,
        "seats": [seat],
        "holdSeconds": 1
    }
    status, res_data = http_request(f"{ctx.gateway_url}/api/inventory/reserve", method="POST", payload=reserve_payload)
    if status != 200:
        ctx.record_fail("5.1 Short TTL Hold", f"Failed to acquire initial hold: status {status}, data: {res_data}")
        return

    res_id = res_data.get("reservationId") or res_data.get("reservation_id")
    ctx.log_info(f"Seat '{seat}' held with 1s lease (reservationId: {res_id}). Waiting for Janitor sweep...")

    # Wait for Janitor to sweep expired reservation
    start_wait = time.time()
    swept = False
    while time.time() - start_wait < 6.0:
        cmd = ["docker", "exec", "ticket_postgres", "psql", "-U", "postgres", "-d", "ticket_db", "-t", "-A",
               "-c", f"SELECT status FROM reservations WHERE reservation_id = '{res_id}';"]
        res = subprocess.run(cmd, capture_output=True, text=True)
        if res.stdout.strip() == "EXPIRED":
            swept = True
            break
        time.sleep(0.3)

    if swept:
        ctx.record_pass("5.1 Janitor Sweep Verification", f"Janitor swept expired reservation {res_id} in {time.time()-start_wait:.2f}s.")
    else:
        ctx.record_fail("5.1 Janitor Sweep Verification", f"Timed out waiting for Janitor to sweep {res_id}")

    # Concurrently attempt to commit the expired reservation AND book the seat via a new booking
    barrier = threading.Barrier(4)
    commit_res = [None]
    booking_res = [None] * 3

    def commit_worker():
        barrier.wait()
        st, d = http_request(f"{ctx.gateway_url}/api/inventory/commit", method="POST", payload={"reservationId": res_id})
        commit_res[0] = (st, d)

    def book_worker(idx: int):
        barrier.wait()
        payload = {
            "eventId": event_id,
            "seats": [seat],
            "ticketCount": 1,
            "customerName": f"Reclaim Racer {idx}",
            "customerEmail": f"reclaim_{idx}@example.com"
        }
        st, d = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=payload)
        booking_res[idx] = (st, d)

    t_commit = threading.Thread(target=commit_worker)
    t_books = [threading.Thread(target=book_worker, args=(i,)) for i in range(3)]

    t_commit.start()
    for t in t_books:
        t.start()

    t_commit.join()
    for t in t_books:
        t.join()

    commit_status = commit_res[0][0]
    if commit_status in (404, 409):
        ctx.record_pass("5.2 Expired Commit Rejection", f"Expired reservation commit correctly blocked with HTTP {commit_status}.")
    else:
        ctx.record_fail("5.2 Expired Commit Rejection", f"Expired commit was not blocked! Status {commit_status}, data: {commit_res[0][1]}")

    book_wins = [b for b in booking_res if b[0] == 201]
    book_conflicts = [b for b in booking_res if b[0] == 409]

    if len(book_wins) == 1 and len(book_conflicts) == 2:
        ctx.record_pass("5.3 Reclaimed Seat Re-booking", f"Seat was successfully re-sold to exactly 1 new winner after expiration.")
    else:
        ctx.record_fail("5.3 Reclaimed Seat Re-booking", f"Expected 1 win & 2 conflicts, got wins: {len(book_wins)}, conflicts: {len(book_conflicts)}")


def test_6_payment_failure_saga_compensation_race(ctx: TestContext):
    """
    Test Saga rollback when payment is declined:
    Releases hold immediately, allowing subsequent booking of the seat.
    """
    ctx.log_section("CHALLENGE 6: Payment Failure Saga Compensation & Seat Recycling")
    event_id = "evt-101"
    seat = "B60"

    # Step 1: Attempt booking with failing card token
    fail_payload = {
        "eventId": event_id,
        "seats": [seat],
        "ticketCount": 1,
        "customerName": "Failing Card Buyer",
        "customerEmail": "fail_card@example.com",
        "paymentInfo": {"cardToken": "tok_fail"}
    }
    st_fail, d_fail = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=fail_payload)
    if st_fail == 402:
        ctx.record_pass("6.1 Payment Decline Simulation", "HTTP 402 Payment Required properly returned for declined card.")
    else:
        ctx.record_fail("6.1 Payment Decline Simulation", f"Expected HTTP 402, got {st_fail}: {d_fail}")

    # Step 2: Confirm seat is NOT held and can be booked by a valid payer
    success_payload = {
        "eventId": event_id,
        "seats": [seat],
        "ticketCount": 1,
        "customerName": "Success Card Buyer",
        "customerEmail": "success_card@example.com",
        "paymentInfo": {"cardToken": "tok_valid"}
    }
    st_succ, d_succ = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=success_payload)
    if st_succ == 201 and d_succ.get("status") == "CONFIRMED":
        ctx.record_pass("6.2 Saga Hold Release & Re-booking", "Seat B60 was successfully booked after failed payment released hold.")
    else:
        ctx.record_fail("6.2 Saga Hold Release & Re-booking", f"Expected HTTP 201 CONFIRMED, got {st_succ}: {d_succ}")


def test_7_partial_unavailability_all_or_nothing(ctx: TestContext):
    """
    Request 3 seats: ['B70', 'B71', 'B72'].
    Pre-book B71 so it is unavailable.
    Assert: The multi-seat request is rejected (409), and NEITHER B70 NOR B72 are held/booked.
    """
    ctx.log_section("CHALLENGE 7: Partial Seat Unavailability Atomic All-or-Nothing Rollback")
    event_id = "evt-101"

    # Pre-book B71
    p_b71 = {
        "eventId": event_id,
        "seats": ["B71"],
        "ticketCount": 1,
        "customerName": "Pre-booker B71",
        "customerEmail": "b71@example.com"
    }
    st, d = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=p_b71)
    if st != 201:
        ctx.record_fail("7.1 Pre-booking Prerequisite", f"Failed to pre-book B71: {st} {d}")
        return

    # Attempt to book ['B70', 'B71', 'B72']
    p_multi = {
        "eventId": event_id,
        "seats": ["B70", "B71", "B72"],
        "ticketCount": 3,
        "customerName": "Multi Booker",
        "customerEmail": "multi@example.com"
    }
    st_multi, d_multi = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=p_multi)
    if st_multi == 409:
        ctx.record_pass("7.2 Conflict Rejection", "Multi-seat request with 1 taken seat cleanly returned HTTP 409 Conflict.")
    else:
        ctx.record_fail("7.2 Conflict Rejection", f"Expected HTTP 409, got {st_multi}: {d_multi}")

    # Verify B70 and B72 remain AVAILABLE
    cmd = ["docker", "exec", "ticket_postgres", "psql", "-U", "postgres", "-d", "ticket_db", "-t", "-A",
           "-c", "SELECT id, status FROM seat_items WHERE event_id = 'evt-101' AND id IN ('B70', 'B72');"]
    res = subprocess.run(cmd, capture_output=True, text=True)
    lines = res.stdout.strip().splitlines()
    b70_avail = any("B70|AVAILABLE" in l for l in lines)
    b72_avail = any("B72|AVAILABLE" in l for l in lines)

    if b70_avail and b72_avail:
        ctx.record_pass("7.3 All-or-Nothing Atomicity", "Uncontested seats B70 & B72 remained AVAILABLE (no leaked holds).")
    else:
        ctx.record_fail("7.3 All-or-Nothing Atomicity", f"Seats leaked into HOLD or BOOKED: {lines}")


def test_8_cross_tier_concurrency_isolation(ctx: TestContext):
    """
    Simultaneously book on VIP and General tiers of evt-101.
    Assert zero crosstalk or interference between tiers.
    """
    ctx.log_section("CHALLENGE 8: Cross-Tier Concurrency Isolation")
    event_id = "evt-101"

    p_vip = {
        "eventId": event_id,
        "seats": ["A40"],
        "ticketCount": 1,
        "customerName": "VIP Customer",
        "customerEmail": "vip@example.com"
    }
    p_gen = {
        "eventId": event_id,
        "seats": ["B40"],
        "ticketCount": 1,
        "customerName": "Gen Customer",
        "customerEmail": "gen@example.com"
    }

    barrier = threading.Barrier(2)
    res = [None, None]

    def w_vip():
        barrier.wait()
        res[0] = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=p_vip)

    def w_gen():
        barrier.wait()
        res[1] = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=p_gen)

    t1 = threading.Thread(target=w_vip)
    t2 = threading.Thread(target=w_gen)
    t1.start()
    t2.start()
    t1.join()
    t2.join()

    if res[0][0] == 201 and res[1][0] == 201:
        ctx.record_pass("8.1 Independent Tier Booking", "Both VIP and General tier bookings confirmed simultaneously (HTTP 201).")
    else:
        ctx.record_fail("8.1 Independent Tier Booking", f"Failed concurrent cross-tier bookings: VIP={res[0][0]}, Gen={res[1][0]}")


def test_9_rapid_sequential_booking_burst(ctx: TestContext):
    """
    Execute 20 rapid sequential bookings on evt-102 standard tier (S1..S20).
    Verifies transaction isolation and connection pool stability under continuous write load.
    """
    ctx.log_section("CHALLENGE 9: Rapid Sequential Booking Burst (20 Bookings)")
    event_id = "evt-102"

    num_bookings = 20
    success_count = 0
    for i in range(1, num_bookings + 1):
        seat = f"S{i}"
        payload = {
            "eventId": event_id,
            "seats": [seat],
            "ticketCount": 1,
            "customerName": f"Burst Buyer {i}",
            "customerEmail": f"burst_{i}@example.com"
        }
        st, d = http_request(f"{ctx.gateway_url}/api/bookings", method="POST", payload=payload)
        if st == 201 and d.get("status") == "CONFIRMED":
            success_count += 1

    if success_count == num_bookings:
        ctx.record_pass("9.1 Sequential Burst Reliability", f"All {num_bookings}/{num_bookings} rapid bookings succeeded with HTTP 201.")
    else:
        ctx.record_fail("9.1 Sequential Burst Reliability", f"Only {success_count}/{num_bookings} rapid bookings succeeded.")


def test_10_database_oracle_invariants(ctx: TestContext):
    """
    Direct SQL Oracle verification across all events and tiers:
      - available + reserved + booked == total for every tier
      - available >= 0 for every tier
      - seat_items table counts match event_inventory counters exactly
      - tickets table has zero duplicate seat tickets
    """
    ctx.log_section("CHALLENGE 10: Direct Database Oracle Integrity & Consistency Check")

    # Brief settle time to ensure all background tasks are quiescent
    time.sleep(1.0)

    try:
        rows = query_db_oracle()
    except Exception as e:
        ctx.record_fail("10.1 Database Oracle Query", f"Failed to execute oracle query: {e}")
        return

    all_sums_ok = True
    all_non_neg = True
    all_counters_match = True

    for r in rows:
        ctx.log_info(
            f"Event {r['event_id']} / {r['tier_id']}: Total={r['total_seats']}, Avail={r['available_seats']}, "
            f"Reserved={r['reserved_seats']}, Booked={r['booked_seats']} | "
            f"ActualSeats(Avail={r['act_avail']}, Hold={r['act_hold']}, Booked={r['act_booked']})"
        )

        if not r["sum_ok"]:
            all_sums_ok = False
            ctx.record_fail(
                f"10.2 Sum Invariant ({r['event_id']}/{r['tier_id']})",
                f"Sum mismatch: {r['available_seats']} + {r['reserved_seats']} + {r['booked_seats']} != {r['total_seats']}"
            )

        if not r["non_neg"]:
            all_non_neg = False
            ctx.record_fail(
                f"10.3 Non-Negative Invariant ({r['event_id']}/{r['tier_id']})",
                f"Negative inventory detected: available={r['available_seats']}"
            )

        if r["act_booked"] != r["booked_seats"] or r["act_hold"] != r["reserved_seats"] or r["act_avail"] != r["available_seats"]:
            all_counters_match = False
            ctx.record_fail(
                f"10.4 Row Count Invariant ({r['event_id']}/{r['tier_id']})",
                f"Mismatch between event_inventory counters and seat_items rows! "
                f"Booked: {r['booked_seats']} vs {r['act_booked']}, "
                f"Hold: {r['reserved_seats']} vs {r['act_hold']}, "
                f"Avail: {r['available_seats']} vs {r['act_avail']}"
            )

    if all_sums_ok:
        ctx.record_pass("10.2 Total Seats Invariant", "available + reserved + booked == total across ALL events and tiers.")

    if all_non_neg:
        ctx.record_pass("10.3 Non-Negative Inventory Invariant", "available >= 0 verified across ALL events and tiers.")

    if all_counters_match:
        ctx.record_pass("10.4 Database Row-to-Counter Sync", "Exact 1:1 match between seat_items row states and event_inventory counts.")

    # Check ticket uniqueness
    cmd_tkt = [
        "docker", "exec", "ticket_postgres", "psql", "-U", "postgres", "-d", "ticket_db", "-t", "-A",
        "-c",
        """
        SELECT b.event_id, t.seat_label, COUNT(*) 
        FROM tickets t 
        JOIN bookings b ON t.booking_id = b.id 
        GROUP BY b.event_id, t.seat_label 
        HAVING COUNT(*) > 1;
        """
    ]
    res_tkt = subprocess.run(cmd_tkt, capture_output=True, text=True, check=True)
    dup_tickets = res_tkt.stdout.strip()
    if not dup_tickets:
        ctx.record_pass("10.5 Zero Duplicate Tickets Invariant", "No duplicate seat tickets emitted across any bookings.")
    else:
        ctx.record_fail("10.5 Zero Duplicate Tickets Invariant", f"Duplicate tickets found in database: {dup_tickets}")


# ==============================================================================
# Main Runner
# ==============================================================================

def main():
    parser = argparse.ArgumentParser(description="Adversarial Concurrency Stress Harness")
    parser.add_argument("--gateway", default="http://localhost:8080", help="API Gateway URL")
    args = parser.parse_args()

    ctx = TestContext(gateway_url=args.gateway)

    print(f"{BOLD}======================================================================{RESET}")
    print(f"{BOLD} ADVERSARIAL CONCURRENCY CHALLENGE SUITE — CHALLENGER 1{RESET}")
    print(f"{BOLD} Target Gateway : {args.gateway}{RESET}")
    print(f"{BOLD} Mode           : EMPIRICAL STRESS TESTING{RESET}")
    print(f"{BOLD}======================================================================{RESET}")

    start_time = time.time()

    test_1_massive_single_seat_contention(ctx)
    test_2_overlapping_seat_contention(ctx)
    test_3_complete_capacity_exhaustion_race(ctx)
    test_4_sold_out_rejection_and_non_negative_check(ctx)
    test_5_ttl_expiration_janitor_race(ctx)
    test_6_payment_failure_saga_compensation_race(ctx)
    test_7_partial_unavailability_all_or_nothing(ctx)
    test_8_cross_tier_concurrency_isolation(ctx)
    test_9_rapid_sequential_booking_burst(ctx)
    test_10_database_oracle_invariants(ctx)

    elapsed = time.time() - start_time

    total = ctx.passed + ctx.failed
    print(f"\n{BOLD}======================================================================{RESET}")
    print(f"{BOLD} CHALLENGE SUITE SUMMARY{RESET}")
    print(f"{BOLD}======================================================================{RESET}")
    print(f" Total Assertions : {total}")
    print(f" Passed           : {GREEN}{ctx.passed}{RESET}")
    print(f" Failed           : {RED if ctx.failed > 0 else GREEN}{ctx.failed}{RESET}")
    print(f" Duration         : {elapsed:.2f}s")
    print(f"{BOLD}======================================================================{RESET}")

    if ctx.failed > 0:
        print(f"\n{RED}{BOLD}[✗] CHALLENGE FAILED — {ctx.failed} assertion failure(s) detected!{RESET}\n")
        for err in ctx.errors:
            print(f"  - {RED}{err}{RESET}")
        sys.exit(1)
    else:
        print(f"\n{GREEN}{BOLD}[✓] CHALLENGE APPROVED — All adversarial stress assertions passed!{RESET}\n")
        sys.exit(0)


if __name__ == "__main__":
    main()
