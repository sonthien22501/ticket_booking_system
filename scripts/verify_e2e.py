#!/usr/bin/env python3
"""
Automated E2E Verification Test Harness for Event Ticket Booking System.
Tests purely against public entrypoints:
  - API Gateway (default: http://localhost:8080)
  - React Frontend (default: http://localhost:3000)

Zero external dependencies: uses Python 3 standard library exclusively
(urllib.request, json, time, sys, argparse, concurrent.futures).

Test Hierarchy:
  - Tier 1: Smoke & Liveness (Gateway /health, Frontend HTTP 200 + HTML DOM)
  - Tier 2: Acceptance Criteria (Catalog fetch, initial inventory S_0,
            booking creation 201 CONFIRMED, inventory decrement assertion S_0 - Q)
  - Tier 3: Boundary & Corner Cases (Duplicate seat conflict 409,
            overbooking rejection 400/409, nonexistent event 404, malformed request 400)
  - Tier 4: Concurrency Contention (Simultaneous race condition for single seat,
            asserting strictly 1 winner and N-1 conflicts, zero overselling)
"""

import argparse
import concurrent.futures
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from typing import Any, Dict, List, Optional, Tuple


# Terminal ANSI Colors
USE_COLOR = sys.stdout.isatty() and os.environ.get("NO_COLOR") is None
GREEN = "\033[92m" if USE_COLOR else ""
RED = "\033[91m" if USE_COLOR else ""
YELLOW = "\033[93m" if USE_COLOR else ""
BLUE = "\033[94m" if USE_COLOR else ""
CYAN = "\033[96m" if USE_COLOR else ""
BOLD = "\033[1m" if USE_COLOR else ""
RESET = "\033[0m" if USE_COLOR else ""


class TestFailure(Exception):
    """Raised when an explicit test assertion fails."""
    pass


class TestContext:
    def __init__(self, gateway_url: str, frontend_url: str, timeout: float = 10.0):
        self.gateway_url = gateway_url.rstrip("/")
        self.frontend_url = frontend_url.rstrip("/")
        self.timeout = timeout
        self.passed_tests = 0
        self.failed_tests = 0
        self.test_details: List[Tuple[str, str, Optional[str]]] = []

    def record_pass(self, name: str, detail: str = ""):
        self.passed_tests += 1
        self.test_details.append(("PASS", name, detail))
        print(f" {GREEN}✓ PASS{RESET} {BOLD}{name}{RESET}" + (f" — {detail}" if detail else ""))

    def record_fail(self, name: str, error: str):
        self.failed_tests += 1
        self.test_details.append(("FAIL", name, error))
        print(f" {RED}✗ FAIL{RESET} {BOLD}{name}{RESET} — {RED}{error}{RESET}")

    def log_info(self, msg: str):
        print(f" {BLUE}ℹ INFO{RESET} {msg}")

    def log_section(self, title: str):
        print(f"\n{CYAN}{'=' * 65}{RESET}")
        print(f"{CYAN}{BOLD} {title}{RESET}")
        print(f"{CYAN}{'=' * 65}{RESET}")


def http_request(
    url: str,
    method: str = "GET",
    payload: Optional[Dict[str, Any]] = None,
    headers: Optional[Dict[str, str]] = None,
    timeout: float = 10.0,
    expected_status: Optional[int] = None,
) -> Tuple[int, Any, Dict[str, str]]:
    """
    Executes an HTTP request using urllib.request.
    Returns (status_code, parsed_json_or_text, response_headers).
    """
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
            resp_headers = dict(resp.headers)
            body_str = body_bytes.decode("utf-8", errors="replace")
            try:
                res_data = json.loads(body_str) if body_str else {}
            except Exception:
                res_data = body_str

            if expected_status is not None and status != expected_status:
                raise TestFailure(
                    f"Expected HTTP {expected_status}, but received {status} for {method} {url}. Response: {body_str[:300]}"
                )
            return status, res_data, resp_headers

    except urllib.error.HTTPError as e:
        status = e.code
        body_bytes = e.read()
        resp_headers = dict(e.headers)
        body_str = body_bytes.decode("utf-8", errors="replace")
        try:
            res_data = json.loads(body_str) if body_str else {}
        except Exception:
            res_data = body_str

        if expected_status is not None and status == expected_status:
            return status, res_data, resp_headers

        if expected_status is not None:
            raise TestFailure(
                f"Expected HTTP {expected_status}, but got {status} ({e.reason}) for {method} {url}. Body: {body_str[:300]}"
            )
        return status, res_data, resp_headers

    except urllib.error.URLError as e:
        raise TestFailure(f"Connection failure for {method} {url}: {e.reason}")
    except Exception as e:
        raise TestFailure(f"Unexpected error requesting {method} {url}: {str(e)}")


def extract_available_seats(inv_data: Any) -> Optional[int]:
    """Extracts available seat count supporting multiple common schema keys."""
    if isinstance(inv_data, dict):
        for key in ["availableSeats", "available_seats", "available", "remaining_seats", "remainingSeats"]:
            if key in inv_data and isinstance(inv_data[key], (int, float)):
                return int(inv_data[key])
        if "summary" in inv_data and isinstance(inv_data["summary"], dict):
            for key in ["availableSeats", "available_seats", "available"]:
                if key in inv_data["summary"] and isinstance(inv_data["summary"][key], (int, float)):
                    return int(inv_data["summary"][key])
    return None


def extract_event_id(event_obj: Any) -> Optional[str]:
    """Extracts event ID supporting id, eventId, event_id."""
    if isinstance(event_obj, dict):
        for key in ["id", "eventId", "event_id"]:
            if key in event_obj and event_obj[key]:
                return str(event_obj[key])
    return None


def extract_booking_id(booking_obj: Any) -> Optional[str]:
    """Extracts booking identifier supporting id, bookingId, booking_id, bookingReference."""
    if isinstance(booking_obj, dict):
        for key in ["id", "bookingId", "booking_id", "bookingReference", "booking_reference"]:
            if key in booking_obj and booking_obj[key]:
                return str(booking_obj[key])
    return None


def extract_booking_status(booking_obj: Any) -> Optional[str]:
    """Extracts booking status supporting status, bookingStatus."""
    if isinstance(booking_obj, dict):
        for key in ["status", "bookingStatus", "state"]:
            if key in booking_obj and booking_obj[key]:
                return str(booking_obj[key]).upper()
    return None


# ==============================================================================
# Tier 1: Smoke & Liveness Tests
# ==============================================================================

def run_tier_1_smoke(ctx: TestContext):
    ctx.log_section("TIER 1: Smoke & Liveness Verification")

    # 1.1 Gateway /health
    test_name = "1.1 API Gateway /health Liveness"
    try:
        status, data, _ = http_request(f"{ctx.gateway_url}/health", method="GET", timeout=ctx.timeout)
        if status != 200:
            raise TestFailure(f"Gateway /health returned HTTP {status}, expected 200")
        ctx.record_pass(test_name, f"HTTP {status} OK, Gateway responsive")
    except Exception as e:
        ctx.record_fail(test_name, str(e))

    # 1.2 Frontend Web App (port 3000)
    test_name = "1.2 React Frontend Web Liveness (port 3000)"
    try:
        status, data, _ = http_request(
            f"{ctx.frontend_url}/",
            method="GET",
            headers={"Accept": "text/html,*/*"},
            timeout=ctx.timeout
        )
        if status != 200:
            raise TestFailure(f"Frontend returned HTTP {status}, expected 200")
        
        data_str = str(data)
        has_html = "<!doctype html" in data_str.lower() or "<html" in data_str.lower() or "root" in data_str
        if not has_html:
            raise TestFailure("Frontend response is not an HTML document")
        ctx.record_pass(test_name, f"HTTP 200 OK, valid HTML single-page app served")
    except Exception as e:
        ctx.record_fail(test_name, str(e))


# ==============================================================================
# Tier 2: Acceptance Criteria Verification
# ==============================================================================

def run_tier_2_acceptance(ctx: TestContext) -> Tuple[Optional[str], Optional[int]]:
    ctx.log_section("TIER 2: Acceptance Criteria (End-to-End Booking & Inventory Decrement)")

    target_event_id: Optional[str] = None
    initial_inventory: Optional[int] = None

    # 2.1 Fetch Event Catalog via Gateway
    test_name = "2.1 Fetch Event Catalog (/api/events)"
    events_list = []
    try:
        status, data, _ = http_request(f"{ctx.gateway_url}/api/events", method="GET", timeout=ctx.timeout, expected_status=200)
        if isinstance(data, list):
            events_list = data
        elif isinstance(data, dict) and "events" in data and isinstance(data["events"], list):
            events_list = data["events"]
        elif isinstance(data, dict) and "data" in data and isinstance(data["data"], list):
            events_list = data["data"]
        else:
            raise TestFailure(f"Invalid catalog structure received: {type(data)} -> {data}")

        if len(events_list) == 0:
            raise TestFailure("Event catalog is empty. Expected at least 1 seeded event.")

        target_event = events_list[0]
        target_event_id = extract_event_id(target_event)
        if not target_event_id:
            raise TestFailure(f"Could not extract event ID from event: {target_event}")

        title = target_event.get("title") or target_event.get("name") or "Unnamed Event"
        ctx.record_pass(test_name, f"Retrieved {len(events_list)} events. Target Event: '{target_event_id}' ({title})")
    except Exception as e:
        ctx.record_fail(test_name, str(e))
        return None, None

    # 2.2 Query Initial Inventory S_0
    test_name = f"2.2 Check Initial Inventory S_0 for Event '{target_event_id}'"
    try:
        status, data, _ = http_request(
            f"{ctx.gateway_url}/api/inventory/{target_event_id}",
            method="GET",
            timeout=ctx.timeout,
            expected_status=200
        )
        initial_inventory = extract_available_seats(data)
        if initial_inventory is None:
            raise TestFailure(f"Could not parse available seats from inventory response: {data}")
        if initial_inventory <= 0:
            raise TestFailure(f"Initial available seats must be > 0, got {initial_inventory}")

        ctx.record_pass(test_name, f"Initial available seats S_0 = {initial_inventory}")
    except Exception as e:
        ctx.record_fail(test_name, str(e))
        return target_event_id, None

    # 2.3 Create Booking via Gateway POST /api/bookings
    test_name = "2.3 Create Booking POST /api/bookings (assert 201 CONFIRMED)"
    seats_to_book = ["A1", "A2"]
    quantity = len(seats_to_book)
    booking_id = None
    try:
        payload = {
            "eventId": target_event_id,
            "event_id": target_event_id,
            "seats": seats_to_book,
            "seat_ids": seats_to_book,
            "ticketCount": quantity,
            "quantity": quantity,
            "customerName": "E2E Acceptance Tester",
            "customer_name": "E2E Acceptance Tester",
            "customerEmail": "e2e_acceptance@example.com",
            "customer_email": "e2e_acceptance@example.com",
            "userId": "usr_e2e_acceptance",
            "user_id": "usr_e2e_acceptance",
        }
        status, data, _ = http_request(
            f"{ctx.gateway_url}/api/bookings",
            method="POST",
            payload=payload,
            timeout=ctx.timeout,
            expected_status=201
        )
        booking_id = extract_booking_id(data)
        b_status = extract_booking_status(data)

        if not booking_id:
            raise TestFailure(f"Response missing booking ID: {data}")
        if b_status != "CONFIRMED":
            raise TestFailure(f"Expected booking status CONFIRMED, got '{b_status}' in {data}")

        ctx.record_pass(test_name, f"Booking confirmed (ID: {booking_id}, Status: {b_status}, Seats: {seats_to_book})")
    except Exception as e:
        ctx.record_fail(test_name, str(e))

    # 2.4 Confirm Inventory Decrement S_0 - Q
    test_name = f"2.4 Assert Inventory Decreased Accurately (S_0 - {quantity})"
    try:
        status, data, _ = http_request(
            f"{ctx.gateway_url}/api/inventory/{target_event_id}",
            method="GET",
            timeout=ctx.timeout,
            expected_status=200
        )
        current_inventory = extract_available_seats(data)
        if current_inventory is None:
            raise TestFailure(f"Could not parse available seats after booking: {data}")

        expected_seats = initial_inventory - quantity
        if current_inventory != expected_seats:
            raise TestFailure(
                f"Seat decrement mismatch! Expected {expected_seats} (initial {initial_inventory} - {quantity}), "
                f"but found {current_inventory}"
            )

        ctx.record_pass(
            test_name,
            f"Inventory accurately decremented from {initial_inventory} to {current_inventory} (-{quantity})"
        )
    except Exception as e:
        ctx.record_fail(test_name, str(e))

    return target_event_id, initial_inventory


# ==============================================================================
# Tier 3: Boundary & Corner Cases
# ==============================================================================

def run_tier_3_boundaries(ctx: TestContext, event_id: Optional[str]):
    ctx.log_section("TIER 3: Boundary & Corner Cases")

    if not event_id:
        ctx.record_fail("Tier 3 Prerequisite", "No valid eventId available from Tier 2")
        return

    # 3.1 Duplicate Seat Booking Conflict (409)
    test_name = "3.1 Duplicate Seat Booking Conflict (Assert HTTP 409)"
    try:
        # Re-attempt booking the same seats ["A1", "A2"] that were confirmed in Tier 2
        payload = {
            "eventId": event_id,
            "event_id": event_id,
            "seats": ["A1", "A2"],
            "seat_ids": ["A1", "A2"],
            "ticketCount": 2,
            "quantity": 2,
            "customerName": "Second Contender",
            "customer_name": "Second Contender",
            "customerEmail": "second_user@example.com",
            "customer_email": "second_user@example.com",
        }
        status, data, _ = http_request(
            f"{ctx.gateway_url}/api/bookings",
            method="POST",
            payload=payload,
            timeout=ctx.timeout
        )
        if status != 409:
            raise TestFailure(f"Duplicate booking must return HTTP 409 Conflict, but received HTTP {status}")
        ctx.record_pass(test_name, f"HTTP {status} Conflict cleanly rejected duplicate seats ['A1', 'A2']")
    except Exception as e:
        ctx.record_fail(test_name, str(e))

    # 3.2 Overbooking Exceeding Total Capacity
    test_name = "3.2 Overbooking Beyond Capacity (Assert 400 or 409)"
    try:
        overbook_seats = [f"OVER_SEAT_{i}" for i in range(10000)]
        payload = {
            "eventId": event_id,
            "event_id": event_id,
            "seats": overbook_seats,
            "seat_ids": overbook_seats,
            "ticketCount": 10000,
            "quantity": 10000,
            "customerName": "Excessive Requester",
            "customer_name": "Excessive Requester",
            "customerEmail": "excessive@example.com",
            "customer_email": "excessive@example.com",
        }
        status, data, _ = http_request(
            f"{ctx.gateway_url}/api/bookings",
            method="POST",
            payload=payload,
            timeout=ctx.timeout
        )
        if status not in (400, 409):
            raise TestFailure(f"Overbooking exceeding capacity must return 400 or 409, but got HTTP {status}")
        ctx.record_pass(test_name, f"HTTP {status} successfully blocked overbooking request (10,000 seats)")
    except Exception as e:
        ctx.record_fail(test_name, str(e))

    # 3.3 Nonexistent Event Query (Assert 404)
    test_name = "3.3 Nonexistent Event Lookup (Assert HTTP 404)"
    try:
        nonexistent_id = "nonexistent-event-uuid-999999"
        status, data, _ = http_request(
            f"{ctx.gateway_url}/api/inventory/{nonexistent_id}",
            method="GET",
            timeout=ctx.timeout
        )
        if status != 404:
            raise TestFailure(f"Querying nonexistent event inventory expected 404, but got HTTP {status}")
        ctx.record_pass(test_name, f"HTTP 404 Not Found returned for missing event '{nonexistent_id}'")
    except Exception as e:
        ctx.record_fail(test_name, str(e))

    # 3.4 Malformed Booking Request (Assert HTTP 400)
    test_name = "3.4 Malformed / Incomplete Booking Payload (Assert HTTP 400)"
    try:
        payload = {
            "eventId": "",
            "seats": [],
            "customerEmail": "invalid"
        }
        status, data, _ = http_request(
            f"{ctx.gateway_url}/api/bookings",
            method="POST",
            payload=payload,
            timeout=ctx.timeout
        )
        if status not in (400, 422):
            raise TestFailure(f"Malformed request expected 400 Bad Request, but got HTTP {status}")
        ctx.record_pass(test_name, f"HTTP {status} correctly rejected malformed booking payload")
    except Exception as e:
        ctx.record_fail(test_name, str(e))


# ==============================================================================
# Tier 4: High-Concurrency Race Contention
# ==============================================================================

def run_tier_4_concurrency(ctx: TestContext, event_id: Optional[str]):
    ctx.log_section("TIER 4: High-Concurrency Anti-Overselling Contention")

    if not event_id:
        ctx.record_fail("Tier 4 Prerequisite", "No valid eventId available from Tier 2")
        return

    test_name = "4.1 High-Concurrency Single Seat Contention (Zero Oversell)"
    target_race_seat = "A3"
    num_concurrent_requests = 10
    ctx.log_info(
        f"Simulating {num_concurrent_requests} concurrent threads contending for seat '{target_race_seat}' on event '{event_id}'..."
    )

    def attempt_booking(worker_idx: int) -> Tuple[int, Any]:
        payload = {
            "eventId": event_id,
            "event_id": event_id,
            "seats": [target_race_seat],
            "seat_ids": [target_race_seat],
            "ticketCount": 1,
            "quantity": 1,
            "customerName": f"Concurrent Racer #{worker_idx}",
            "customer_name": f"Concurrent Racer #{worker_idx}",
            "customerEmail": f"racer_{worker_idx}@example.com",
            "customer_email": f"racer_{worker_idx}@example.com",
            "userId": f"usr_race_{worker_idx}",
            "user_id": f"usr_race_{worker_idx}",
        }
        try:
            status, data, _ = http_request(
                f"{ctx.gateway_url}/api/bookings",
                method="POST",
                payload=payload,
                timeout=ctx.timeout
            )
            return status, data
        except TestFailure as tf:
            return 500, str(tf)
        except Exception as ex:
            return 500, str(ex)

    # Fire all concurrent requests simultaneously
    results: List[Tuple[int, Any]] = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=num_concurrent_requests) as executor:
        futures = [executor.submit(attempt_booking, i + 1) for i in range(num_concurrent_requests)]
        for fut in concurrent.futures.as_completed(futures):
            results.append(fut.result())

    confirmed_count = sum(1 for status, data in results if status == 201)
    conflict_count = sum(1 for status, data in results if status == 409)
    other_count = len(results) - (confirmed_count + conflict_count)

    ctx.log_info(
        f"Race results: 201 Created = {confirmed_count}, 409 Conflict = {conflict_count}, Other = {other_count}"
    )

    try:
        if confirmed_count != 1:
            raise TestFailure(
                f"Concurrency failure! Expected exactly 1 winner (201 Created), but observed {confirmed_count}. (Overselling or failure to book)"
            )
        if conflict_count != num_concurrent_requests - 1:
            raise TestFailure(
                f"Expected remaining {num_concurrent_requests - 1} requests to receive 409 Conflict, but got {conflict_count}"
            )
        ctx.record_pass(
            test_name,
            f"Exactly 1 winner (201 CONFIRMED), {conflict_count} cleanly rejected (409 CONFLICT). Zero overselling verified!"
        )
    except Exception as e:
        ctx.record_fail(test_name, str(e))


# ==============================================================================
# Health Check Wait Loop
# ==============================================================================

def wait_for_liveness(ctx: TestContext, max_retries: int = 30, delay: float = 2.0) -> bool:
    ctx.log_info(f"Waiting for Gateway ({ctx.gateway_url}) to report healthy (max {max_retries} attempts)...")
    for attempt in range(1, max_retries + 1):
        try:
            status, data, _ = http_request(f"{ctx.gateway_url}/health", method="GET", timeout=3.0)
            if status == 200:
                ctx.log_info(f"Gateway is UP and healthy on attempt {attempt}!")
                return True
        except Exception:
            pass
        time.sleep(delay)
    return False


# ==============================================================================
# Main Runner & CLI
# ==============================================================================

def main():
    parser = argparse.ArgumentParser(
        description="E2E Verification Harness for Event Ticket Booking System"
    )
    parser.add_argument(
        "--gateway",
        default="http://localhost:8080",
        help="API Gateway base URL (default: http://localhost:8080)"
    )
    parser.add_argument(
        "--frontend",
        default="http://localhost:3000",
        help="React Frontend base URL (default: http://localhost:3000)"
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=10.0,
        help="Per-request HTTP timeout in seconds (default: 10.0)"
    )
    parser.add_argument(
        "--retries",
        type=int,
        default=30,
        help="Liveness health check retry attempts (default: 30)"
    )
    parser.add_argument(
        "--delay",
        type=float,
        default=2.0,
        help="Seconds between health check retries (default: 2.0)"
    )
    parser.add_argument(
        "--tier",
        choices=["all", "1", "2", "3", "4"],
        default="all",
        help="Execute specific test tier (default: all)"
    )
    parser.add_argument(
        "--skip-wait",
        action="store_true",
        help="Skip the initial health check polling"
    )

    args = parser.parse_args()

    ctx = TestContext(gateway_url=args.gateway, frontend_url=args.frontend, timeout=args.timeout)

    print(f"{BOLD}================================================================={RESET}")
    print(f"{BOLD} EVENT TICKET BOOKING SYSTEM — AUTOMATED E2E TEST HARNESS{RESET}")
    print(f"{BOLD} Target Gateway : {args.gateway}{RESET}")
    print(f"{BOLD} Target Frontend: {args.frontend}{RESET}")
    print(f"{BOLD} Selected Tier  : {args.tier}{RESET}")
    print(f"{BOLD}================================================================={RESET}")

    start_time = time.time()

    if not args.skip_wait:
        if not wait_for_liveness(ctx, max_retries=args.retries, delay=args.delay):
            print(f"\n{RED}{BOLD}ERROR: Timed out waiting for API Gateway at {args.gateway}/health{RESET}")
            sys.exit(1)

    event_id: Optional[str] = None

    if args.tier in ("all", "1"):
        run_tier_1_smoke(ctx)

    if args.tier in ("all", "2"):
        event_id, _ = run_tier_2_acceptance(ctx)

    # For standalone tier 3 or 4 runs if tier 2 wasn't run
    if args.tier in ("3", "4") and not event_id:
        try:
            _, catalog_data, _ = http_request(f"{ctx.gateway_url}/api/events", method="GET", timeout=ctx.timeout)
            events = catalog_data if isinstance(catalog_data, list) else catalog_data.get("events", [])
            if events:
                event_id = extract_event_id(events[0])
        except Exception:
            pass

    if args.tier in ("all", "3"):
        run_tier_3_boundaries(ctx, event_id)

    if args.tier in ("all", "4"):
        run_tier_4_concurrency(ctx, event_id)

    elapsed = time.time() - start_time

    # Summary Report
    total_run = ctx.passed_tests + ctx.failed_tests
    print(f"\n{BOLD}================================================================={RESET}")
    print(f"{BOLD} TEST EXECUTION SUMMARY{RESET}")
    print(f"{BOLD}================================================================={RESET}")
    print(f" Total Tests Run : {total_run}")
    print(f" Passed          : {GREEN}{ctx.passed_tests}{RESET}")
    print(f" Failed          : {RED if ctx.failed_tests > 0 else GREEN}{ctx.failed_tests}{RESET}")
    print(f" Total Duration  : {elapsed:.2f}s")
    print(f"{BOLD}================================================================={RESET}")

    if ctx.failed_tests > 0:
        print(f"{RED}{BOLD}[✗] TEST SUITE FAILED with {ctx.failed_tests} failure(s).{RESET}\n")
        sys.exit(1)
    else:
        print(f"{GREEN}{BOLD}[✓] ALL E2E ACCEPTANCE TESTS PASSED SUCCESSFULLY!{RESET}\n")
        sys.exit(0)


if __name__ == "__main__":
    main()
