#!/usr/bin/env python3
"""
Adversarial & Boundary Stress Test Suite for Event Ticket Booking System
Challenger 2 — Boundary, Malformed Payload, Overbooking, & Failure Modes
"""

import sys
import os
import json
import time
import urllib.request
import urllib.error
import urllib.parse
from concurrent.futures import ThreadPoolExecutor, as_completed

GATEWAY_URL = os.environ.get("GATEWAY_URL", "http://localhost:8080")
FRONTEND_URL = os.environ.get("FRONTEND_URL", "http://localhost:3000")

class Colors:
    GREEN = "\033[92m"
    RED = "\033[91m"
    YELLOW = "\033[93m"
    BLUE = "\033[94m"
    CYAN = "\033[96m"
    BOLD = "\033[1m"
    RESET = "\033[0m"

results = {
    "total": 0,
    "passed": 0,
    "failed": 0,
    "details": []
}

def log_test(test_id, name, passed, status_code, details, error=None):
    results["total"] += 1
    if passed:
        results["passed"] += 1
        print(f" {Colors.GREEN}✓ PASS{Colors.RESET} [{test_id}] {name} (HTTP {status_code}) — {details}")
    else:
        results["failed"] += 1
        print(f" {Colors.RED}✗ FAIL{Colors.RESET} [{test_id}] {name} (HTTP {status_code}) — {details}")
        if error:
            print(f"   {Colors.RED}Details:{Colors.RESET} {error}")
    results["details"].append({
        "id": test_id,
        "name": name,
        "passed": passed,
        "statusCode": status_code,
        "details": details,
        "error": str(error) if error else None
    })

def make_request(url, method="GET", headers=None, data=None, timeout=10.0):
    if headers is None:
        headers = {}
    if data is not None and isinstance(data, (dict, list)):
        body = json.dumps(data).encode("utf-8")
        if "Content-Type" not in headers:
            headers["Content-Type"] = "application/json"
    elif data is not None and isinstance(data, str):
        body = data.encode("utf-8")
    elif data is not None and isinstance(data, bytes):
        body = data
    else:
        body = None

    req = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            resp_body = resp.read().decode("utf-8", errors="replace")
            return resp.status, resp_body, None
    except urllib.error.HTTPError as e:
        resp_body = e.read().decode("utf-8", errors="replace")
        return e.code, resp_body, None
    except urllib.error.URLError as e:
        return 0, "", str(e.reason)
    except Exception as e:
        return 0, "", str(e)

def run_tests():
    print(f"{Colors.BOLD}{Colors.CYAN}{'='*70}{Colors.RESET}")
    print(f"{Colors.BOLD}{Colors.CYAN} CHALLENGER 2: ADVERSARIAL BOUNDARY & FAILURE MODE TEST SUITE{Colors.RESET}")
    print(f" Gateway URL : {GATEWAY_URL}")
    print(f" Frontend URL: {FRONTEND_URL}")
    print(f"{Colors.BOLD}{Colors.CYAN}{'='*70}{Colors.RESET}\n")

    # ---------------------------------------------------------
    # SUITE 1: Boundary & Overbooking Limits
    # ---------------------------------------------------------
    print(f"{Colors.BOLD}--- SUITE 1: Boundary & Overbooking Limits ---{Colors.RESET}")

    # 1.1 Massive overbooking ticketCount = 10,000
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "evt-101", "ticketCount": 10000, "customerEmail": "overbook@test.com"}
    )
    passed = status in (400, 409) and "OVERBOOKING_LIMIT_EXCEEDED" in body
    log_test("B-01", "Massive Overbooking Limit (10,000 tickets)", passed, status, "Expected 400/409 limit rejection")

    # 1.2 Boundary overbooking ticketCount = 501 (exceeds limit 500)
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "evt-101", "ticketCount": 501, "customerEmail": "overbook501@test.com"}
    )
    passed = status == 400 and "OVERBOOKING_LIMIT_EXCEEDED" in body
    log_test("B-02", "Exact Upper Boundary Overbooking (501 tickets)", passed, status, "Expected 400 limit rejection")

    # 1.3 Negative ticket count (ticketCount = -5)
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "evt-101", "ticketCount": -5, "seats": ["B10"], "customerEmail": "neg@test.com"}
    )
    # Server falls back to len(seats) = 1 and safely books or rejects without 500 panic
    passed = status in (201, 400, 409) and status != 500
    log_test("B-03", "Negative Ticket Count (-5) Handled Safely", passed, status, "No 500 crash or panic")

    # 1.4 Invalid seat ID format (non-existent seat 'Z999')
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "evt-101", "seats": ["Z999"], "customerEmail": "z999@test.com"}
    )
    passed = status == 409 and "SEAT_UNAVAILABLE" in body
    log_test("B-04", "Invalid Seat ID ('Z999') Clean Conflict Rejection", passed, status, "Expected 409 SEAT_UNAVAILABLE")

    # 1.5 Special characters & SQL injection string in seat ID
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "evt-101", "seats": ["' OR '1'='1"], "customerEmail": "sqli@test.com"}
    )
    passed = status == 409 and "SEAT_UNAVAILABLE" in body
    log_test("B-05", "SQL Injection Vector in Seat ID Handled Safely", passed, status, "Expected 409 conflict, no SQL injection")

    # 1.6 Unicode / Emoji in seat ID
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "evt-101", "seats": ["🪑_SEAT_🦄"], "customerEmail": "emoji@test.com"}
    )
    passed = status in (400, 409)
    log_test("B-06", "Unicode/Emoji in Seat ID Handled Safely", passed, status, "Expected 400/409 clean rejection")

    # 1.7 Ultra-long seat ID string (5000 characters)
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "evt-101", "seats": ["A" * 5000], "customerEmail": "long@test.com"}
    )
    passed = status in (400, 409)
    log_test("B-07", "Ultra-Long Seat ID (5000 chars) Handled Gracefully", passed, status, "Clean rejection without crash")

    # 1.8 Exceeding tier capacity (request 50 seats when tier only has 40 seats)
    # Event evt-102 tier-keynote has capacity 40
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/inventory/reserve",
        method="POST",
        data={"eventId": "evt-102", "tierId": "tier-keynote", "ticketCount": 50}
    )
    passed = status == 409 and "SEAT_UNAVAILABLE" in body
    log_test("B-08", "Request Exceeding Tier Total Capacity (50 > 40)", passed, status, "Expected 409 SEAT_UNAVAILABLE")

    # ---------------------------------------------------------
    # SUITE 2: Malformed Payloads & Type Poisoning
    # ---------------------------------------------------------
    print(f"\n{Colors.BOLD}--- SUITE 2: Malformed Payloads & Type Poisoning ---{Colors.RESET}")

    # 2.1 Truncated / Invalid JSON Syntax
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data='{"eventId": "evt-101", "seats": ["B1"'
    )
    passed = status == 400
    log_test("M-01", "Truncated / Invalid JSON Syntax", passed, status, "Expected 400 Bad Request")

    # 2.2 Completely Empty Body
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data=""
    )
    passed = status == 400
    log_test("M-02", "Completely Empty Body", passed, status, "Expected 400 Bad Request")

    # 2.3 Empty JSON Object {}
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={}
    )
    passed = status == 400
    log_test("M-03", "Empty JSON Object (Missing Required Fields)", passed, status, "Expected 400 Bad Request")

    # 2.4 Wrong Data Type: eventId as Integer instead of String
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": 12345, "seats": ["B1"]}
    )
    passed = status == 400
    log_test("M-04", "Type Poisoning: eventId as Integer", passed, status, "Expected 400 Bad Request")

    # 2.5 Wrong Data Type: seats as String instead of Array
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "evt-101", "seats": "B1"}
    )
    passed = status == 400
    log_test("M-05", "Type Poisoning: seats as String instead of Array", passed, status, "Expected 400 Bad Request")

    # 2.6 Wrong Data Type: ticketCount as String instead of Integer
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "evt-101", "ticketCount": "three"}
    )
    passed = status == 400
    log_test("M-06", "Type Poisoning: ticketCount as String", passed, status, "Expected 400 Bad Request")

    # 2.7 Array with Null Elements in seats
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "evt-101", "seats": [None, 123, "B1"]}
    )
    passed = status == 400
    log_test("M-07", "Array Containing Nulls & Integers in seats", passed, status, "Expected 400 Bad Request")

    # 2.8 Arbitrary non-JSON MIME Content-Type
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        headers={"Content-Type": "text/plain"},
        data="eventId=evt-101&seats=B1"
    )
    passed = status == 400
    log_test("M-08", "Unsupported Content-Type (text/plain)", passed, status, "Expected 400 Bad Request")

    # 2.9 Unsupported HTTP Methods
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="DELETE"
    )
    passed = status in (405, 400, 404)
    log_test("M-09", "Method Not Allowed (DELETE /api/bookings)", passed, status, "Expected 405 Method Not Allowed")

    # 2.10 Unknown / Invalid API Gateway Route
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/nonexistent_service/foobar",
        method="GET"
    )
    passed = status == 404
    log_test("M-10", "Gateway Unknown Route (/api/nonexistent_service)", passed, status, "Expected 404 Not Found")

    # ---------------------------------------------------------
    # SUITE 3: Non-Existent Resources
    # ---------------------------------------------------------
    print(f"\n{Colors.BOLD}--- SUITE 3: Non-Existent Resource Lookups ---{Colors.RESET}")

    # 3.1 Non-Existent Event Lookup
    status, body, err = make_request(f"{GATEWAY_URL}/api/events/nonexistent-evt-999999")
    passed = status == 404
    log_test("N-01", "GET /api/events/{nonexistent}", passed, status, "Expected 404 Not Found")

    # 3.2 Non-Existent Event Inventory
    status, body, err = make_request(f"{GATEWAY_URL}/api/inventory/nonexistent-evt-999999")
    passed = status == 404
    log_test("N-02", "GET /api/inventory/{nonexistent}", passed, status, "Expected 404 Not Found")

    # 3.3 Non-Existent Event Seats Query
    status, body, err = make_request(f"{GATEWAY_URL}/api/inventory/nonexistent-evt-999999/seats")
    passed = status in (200, 404) and ("seats" in body or status == 404)
    log_test("N-03", "GET /api/inventory/{nonexistent}/seats", passed, status, "Returns empty seat list or 404, no crash")

    # 3.4 Non-Existent Booking Query
    status, body, err = make_request(f"{GATEWAY_URL}/api/bookings/nonexistent-booking-999999")
    passed = status == 404
    log_test("N-04", "GET /api/bookings/{nonexistent}", passed, status, "Expected 404 Not Found")

    # 3.5 Path Traversal Attempt
    status, body, err = make_request(f"{GATEWAY_URL}/api/events/../../../../etc/passwd")
    passed = status in (400, 404)
    log_test("N-05", "Path Traversal in URL Path", passed, status, "Clean 400/404 rejection, no traversal leak")

    # 3.6 Booking Attempt on Non-Existent Event
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "nonexistent-evt-999999", "seats": ["A1"], "customerEmail": "test@test.com"}
    )
    passed = status in (404, 409, 400)
    log_test("N-06", "POST /api/bookings for Non-Existent Event", passed, status, "Clean 409/404/400 conflict, no 500 panic")

    # ---------------------------------------------------------
    # SUITE 4: Concurrency, Contention, & Saga Compensation
    # ---------------------------------------------------------
    print(f"\n{Colors.BOLD}--- SUITE 4: Concurrency, Contention, & Saga Compensation ---{Colors.RESET}")

    # 4.1 Rapid Burst Contention (20 Threads Contending for Single Seat B20)
    target_seat = "B20"
    num_threads = 20
    print(f" [*] Firing {num_threads} simultaneous requests contending for single seat '{target_seat}'...")

    def book_single_seat(idx):
        return make_request(
            f"{GATEWAY_URL}/api/bookings",
            method="POST",
            data={
                "eventId": "evt-101",
                "seats": [target_seat],
                "customerName": f"Racer {idx}",
                "customerEmail": f"racer{idx}@test.com"
            }
        )

    with ThreadPoolExecutor(max_workers=num_threads) as executor:
        futures = [executor.submit(book_single_seat, i) for i in range(num_threads)]
        race_results = [f.result() for f in as_completed(futures)]

    success_count = sum(1 for status, _, _ in race_results if status == 201)
    conflict_count = sum(1 for status, _, _ in race_results if status == 409)
    other_count = len(race_results) - (success_count + conflict_count)

    passed_race = (success_count == 1 and conflict_count == (num_threads - 1) and other_count == 0)
    log_test(
        "C-01",
        f"Single Seat Race ({num_threads} threads for '{target_seat}')",
        passed_race,
        201 if success_count == 1 else 0,
        f"Winners: {success_count}, 409 Conflicts: {conflict_count}, Others: {other_count} (Zero Oversell)"
    )

    # 4.2 Re-booking Already Booked Seat B20 (Idempotent 409 Conflict)
    status, body, err = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={"eventId": "evt-101", "seats": [target_seat], "customerEmail": "late@test.com"}
    )
    passed = status == 409 and "SEAT_UNAVAILABLE" in body
    log_test("C-02", f"Subsequent Booking of Taken Seat '{target_seat}' (Assert 409)", passed, status, "Expected 409 SEAT_UNAVAILABLE")

    # 4.3 Multi-Seat Contention (20 Threads Contending for 5 Available Seats: B31..B35)
    target_pool = [f"B{i}" for i in range(31, 36)] # 5 seats
    print(f" [*] Firing 20 threads contending for 5 seats {target_pool}...")

    def book_from_pool(idx):
        seat = target_pool[idx % len(target_pool)]
        return make_request(
            f"{GATEWAY_URL}/api/bookings",
            method="POST",
            data={
                "eventId": "evt-101",
                "seats": [seat],
                "customerName": f"Pool Racer {idx}",
                "customerEmail": f"pool{idx}@test.com"
            }
        )

    with ThreadPoolExecutor(max_workers=20) as executor:
        futures = [executor.submit(book_from_pool, i) for i in range(20)]
        pool_results = [f.result() for f in as_completed(futures)]

    pool_success = sum(1 for status, _, _ in pool_results if status == 201)
    pool_conflict = sum(1 for status, _, _ in pool_results if status == 409)
    pool_other = len(pool_results) - (pool_success + pool_conflict)

    passed_pool = (pool_success == len(target_pool) and pool_conflict == (20 - len(target_pool)) and pool_other == 0)
    log_test(
        "C-03",
        f"Multi-Seat Pool Contention (20 threads for {len(target_pool)} seats)",
        passed_pool,
        201,
        f"Winners: {pool_success}, Conflicts: {pool_conflict}, Others: {pool_other}"
    )

    # 4.4 Payment Failure Simulation & Saga Compensation Release
    # Book seat B40 with failing payment token -> expect 402 Payment Required
    comp_seat = "B40"
    status_fail, body_fail, _ = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={
            "eventId": "evt-101",
            "seats": [comp_seat],
            "customerEmail": "failpay@test.com",
            "paymentInfo": {"cardToken": "tok_fail"}
        }
    )
    passed_comp_fail = (status_fail == 402 and "PAYMENT_FAILED" in body_fail)
    log_test("C-04a", f"Payment Decline Simulation on Seat '{comp_seat}'", passed_comp_fail, status_fail, "Expected 402 PAYMENT_FAILED")

    # Immediate second booking with valid payment -> MUST succeed because hold was released!
    time.sleep(0.5)
    status_succ, body_succ, _ = make_request(
        f"{GATEWAY_URL}/api/bookings",
        method="POST",
        data={
            "eventId": "evt-101",
            "seats": [comp_seat],
            "customerEmail": "retrypay@test.com"
        }
    )
    passed_comp_succ = (status_succ == 201 and "CONFIRMED" in body_succ)
    log_test("C-04b", f"Saga Compensation Hold Release Verified for Seat '{comp_seat}'", passed_comp_succ, status_succ, "Seat re-booked cleanly after payment failure")

    # ---------------------------------------------------------
    # SUITE 5: Frontend Resilience & Web Integrity (:3000)
    # ---------------------------------------------------------
    print(f"\n{Colors.BOLD}--- SUITE 5: Frontend Resilience & Web Integrity (:3000) ---{Colors.RESET}")

    # 5.1 Frontend Root Liveness
    status, body, err = make_request(f"{FRONTEND_URL}/")
    passed = status == 200 and "<div id=\"root\">" in body
    log_test("F-01", "Frontend Root HTML Serving (port 3000)", passed, status, "Valid HTML single-page app root rendered")

    # 5.2 Frontend Proxy to Events API
    status, body, err = make_request(f"{FRONTEND_URL}/api/events")
    passed = status == 200 and "evt-101" in body
    log_test("F-02", "Frontend Reverse Proxy to /api/events", passed, status, "Frontend reverse proxy delivers catalog")

    # 5.3 Frontend Proxy to Inventory API
    status, body, err = make_request(f"{FRONTEND_URL}/api/inventory/evt-101")
    passed = status == 200 and "available_seats" in body
    log_test("F-03", "Frontend Reverse Proxy to /api/inventory", passed, status, "Frontend reverse proxy delivers inventory")

    # 5.4 Frontend SPA Client-Side Route Fallback
    status, body, err = make_request(f"{FRONTEND_URL}/events/evt-101")
    passed = status == 200 and "<div id=\"root\">" in body
    log_test("F-04", "Frontend SPA Routing Fallback (/events/evt-101)", passed, status, "try_files correctly serves index.html for client route")

    # 5.5 Frontend Non-Existent Client Route Fallback
    status, body, err = make_request(f"{FRONTEND_URL}/any/arbitrary/nested/path")
    passed = status == 200 and "<div id=\"root\">" in body
    log_test("F-05", "Frontend Deep SPA Fallback (/any/arbitrary/nested/path)", passed, status, "Clean SPA index.html returned")

    # 5.6 Frontend Health Check
    status, body, err = make_request(f"{FRONTEND_URL}/health")
    passed = status == 200 and "frontend" in body
    log_test("F-06", "Frontend Health Check (/health)", passed, status, "HTTP 200 UP")

    # ---------------------------------------------------------
    # SUITE 6: Rapid Burst Load / Stress
    # ---------------------------------------------------------
    print(f"\n{Colors.BOLD}--- SUITE 6: Rapid Burst Load & Gateway Stress ---{Colors.RESET}")

    burst_count = 50
    print(f" [*] Sending {burst_count} rapid GET /api/events requests concurrently...")

    def burst_fetch(idx):
        return make_request(f"{GATEWAY_URL}/api/events")

    with ThreadPoolExecutor(max_workers=10) as executor:
        futures = [executor.submit(burst_fetch, i) for i in range(burst_count)]
        burst_results = [f.result() for f in as_completed(futures)]

    burst_200 = sum(1 for status, _, _ in burst_results if status == 200)
    passed_burst = (burst_200 == burst_count)
    log_test("S-01", f"Rapid Gateway Query Burst ({burst_count} reqs)", passed_burst, 200, f"{burst_200}/{burst_count} succeeded with HTTP 200")

    # ---------------------------------------------------------
    # SUMMARY & FINAL VERDICT
    # ---------------------------------------------------------
    print(f"\n{Colors.BOLD}{Colors.CYAN}{'='*70}{Colors.RESET}")
    print(f"{Colors.BOLD} ADVERSARIAL TEST RESULTS SUMMARY{Colors.RESET}")
    print(f" Total Tests Run : {results['total']}")
    print(f" Passed          : {Colors.GREEN}{results['passed']}{Colors.RESET}")
    print(f" Failed          : {Colors.RED if results['failed'] > 0 else Colors.GREEN}{results['failed']}{Colors.RESET}")
    print(f"{Colors.BOLD}{Colors.CYAN}{'='*70}{Colors.RESET}")

    if results["failed"] == 0:
        print(f"\n{Colors.BOLD}{Colors.GREEN}[✓] VERDICT: APPROVE — ALL ADVERSARIAL TESTS PASSED!{Colors.RESET}")
        return 0
    else:
        print(f"\n{Colors.BOLD}{Colors.RED}[✗] VERDICT: CHALLENGE_FAILED — {results['failed']} test(s) failed.{Colors.RESET}")
        return 1

if __name__ == "__main__":
    sys.exit(run_tests())
