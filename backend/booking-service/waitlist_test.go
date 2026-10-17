package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func getTestDB(t *testing.T) *sql.DB {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5433"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}
	pass := os.Getenv("DB_PASSWORD")
	if pass == "" {
		pass = "postgrespassword"
	}
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "ticket_db"
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, pass, dbname)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Skipf("Skipping DB test: unable to open postgres connection: %v", err)
		return nil
	}
	if err := db.Ping(); err != nil {
		t.Skipf("Skipping DB test: postgres unreachable at %s:%s: %v", host, port, err)
		return nil
	}
	return db
}

func TestWaitlistRepositoryFIFOAndStatusProgression(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	srv := &Server{db: db}
	ctx := context.Background()

	// Ensure waitlist schema is initialized
	if err := srv.initWaitlistSchema(ctx); err != nil {
		t.Fatalf("Failed to init waitlist schema: %v", err)
	}

	testEventID := "evt-101"
	testTierID := "tier-vip"

	// Clean up any leftovers from previous runs
	_, _ = db.Exec("DELETE FROM waitlist WHERE user_id LIKE 'user-fifo-%'")
	defer func() {
		_, _ = db.Exec("DELETE FROM waitlist WHERE user_id LIKE 'user-fifo-%'")
	}()

	// Create 3 waitlist entries in sequence
	entry1 := &WaitlistEntry{
		EventID:       testEventID,
		TierID:        testTierID,
		UserID:        "user-fifo-1",
		CustomerName:  "User One",
		CustomerEmail: "user1@example.com",
		Quantity:      1,
	}
	e1, err := srv.CreateWaitlistEntry(ctx, entry1)
	if err != nil {
		t.Fatalf("Failed to create waitlist entry 1: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // Ensure distinct created_at
	entry2 := &WaitlistEntry{
		EventID:       testEventID,
		TierID:        testTierID,
		UserID:        "user-fifo-2",
		CustomerName:  "User Two",
		CustomerEmail: "user2@example.com",
		Quantity:      1,
	}
	e2, err := srv.CreateWaitlistEntry(ctx, entry2)
	if err != nil {
		t.Fatalf("Failed to create waitlist entry 2: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	entry3 := &WaitlistEntry{
		EventID:       testEventID,
		TierID:        testTierID,
		UserID:        "user-fifo-3",
		CustomerName:  "User Three",
		CustomerEmail: "user3@example.com",
		Quantity:      1,
	}
	e3, err := srv.CreateWaitlistEntry(ctx, entry3)
	if err != nil {
		t.Fatalf("Failed to create waitlist entry 3: %v", err)
	}

	// Verify FIFO positions: e1 should be ahead of e2, e2 ahead of e3
	pos1, err := srv.GetWaitlistPosition(ctx, e1)
	if err != nil {
		t.Fatalf("Failed to get position 1: %v", err)
	}
	pos2, err := srv.GetWaitlistPosition(ctx, e2)
	if err != nil {
		t.Fatalf("Failed to get position 2: %v", err)
	}
	pos3, err := srv.GetWaitlistPosition(ctx, e3)
	if err != nil {
		t.Fatalf("Failed to get position 3: %v", err)
	}

	if pos1 >= pos2 || pos2 >= pos3 {
		t.Errorf("Expected FIFO ordering pos1 < pos2 < pos3, got pos1=%d, pos2=%d, pos3=%d", pos1, pos2, pos3)
	}

	// Verify FindNextWaitlistEntryForPromotion returns e1 (the earliest pending)
	nextEntry, err := srv.FindNextWaitlistEntryForPromotion(ctx, testEventID, testTierID)
	if err != nil {
		t.Fatalf("Failed to find next entry for promotion: %v", err)
	}
	if nextEntry == nil {
		t.Fatalf("Expected nextEntry to be non-nil")
	}
	if nextEntry.ID != e1.ID {
		t.Errorf("Expected e1 (%s) to be promoted first, got %s", e1.ID, nextEntry.ID)
	}

	// Promote e1
	fakeResID := "res-test-waitlist-1"
	expiresAt := time.Now().Add(10 * time.Minute)
	err = srv.PromoteWaitlistEntry(ctx, e1.ID, fakeResID, expiresAt)
	if err != nil {
		t.Fatalf("Failed to promote waitlist entry e1: %v", err)
	}

	// Check status progression
	fetchedE1, err := srv.GetWaitlistEntry(ctx, e1.ID)
	if err != nil {
		t.Fatalf("Failed to get e1: %v", err)
	}
	if fetchedE1.Status != WaitlistStatusPromoted {
		t.Errorf("Expected status %s, got %s", WaitlistStatusPromoted, fetchedE1.Status)
	}
	if fetchedE1.ReservationID != fakeResID {
		t.Errorf("Expected reservationId %s, got %s", fakeResID, fetchedE1.ReservationID)
	}

	// Now e2 should be next for promotion!
	nextEntry2, err := srv.FindNextWaitlistEntryForPromotion(ctx, testEventID, testTierID)
	if err != nil {
		t.Fatalf("Failed to find next entry for promotion after e1 promoted: %v", err)
	}
	if nextEntry2 == nil {
		t.Fatalf("Expected nextEntry2 to be non-nil")
	}
	if nextEntry2.ID != e2.ID {
		t.Errorf("Expected e2 (%s) to be next for promotion, got %s", e2.ID, nextEntry2.ID)
	}
}

func TestWaitlistHTTP(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	srv := &Server{
		db:         db,
		httpClient: &http.Client{},
	}
	ctx := context.Background()
	_ = srv.initWaitlistSchema(ctx)

	// Clean up any test users
	_, _ = db.Exec("DELETE FROM waitlist WHERE user_id LIKE 'user-http-%'")
	defer func() {
		_, _ = db.Exec("DELETE FROM waitlist WHERE user_id LIKE 'user-http-%'")
	}()

	// 1. Test POST /waitlist
	payload := map[string]interface{}{
		"eventId":       "evt-101",
		"tierId":        "tier-vip",
		"userId":        "user-http-1",
		"customerName":  "HTTP Tester",
		"customerEmail": "http@test.com",
		"quantity":      1,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/waitlist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	srv.handleWaitlist(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Expected status 201 Created on POST /waitlist, got %d: %s", rr.Code, rr.Body.String())
	}

	var created WaitlistEntry
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if created.ID == "" || created.Position < 1 || created.Status != WaitlistStatusPending {
		t.Errorf("Unexpected created entry: %+v", created)
	}

	// 2. Test GET /waitlist?eventId=evt-101
	getReq := httptest.NewRequest(http.MethodGet, "/waitlist?eventId=evt-101&userId=user-http-1", nil)
	getRR := httptest.NewRecorder()
	srv.handleWaitlist(getRR, getReq)

	if getRR.Code != http.StatusOK {
		t.Fatalf("Expected status 200 on GET /waitlist, got %d: %s", getRR.Code, getRR.Body.String())
	}
	var list []WaitlistEntry
	if err := json.NewDecoder(getRR.Body).Decode(&list); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("Expected at least 1 entry in list, got 0")
	}

	// 3. Test GET /waitlist/{id}
	getOneReq := httptest.NewRequest(http.MethodGet, "/waitlist/"+created.ID, nil)
	getOneRR := httptest.NewRecorder()
	srv.handleWaitlistByID(getOneRR, getOneReq)

	if getOneRR.Code != http.StatusOK {
		t.Fatalf("Expected status 200 on GET /waitlist/{id}, got %d: %s", getOneRR.Code, getOneRR.Body.String())
	}
	var fetched WaitlistEntry
	if err := json.NewDecoder(getOneRR.Body).Decode(&fetched); err != nil {
		t.Fatalf("Failed to parse getOne response: %v", err)
	}
	if fetched.ID != created.ID || fetched.Position < 1 {
		t.Errorf("Unexpected fetched entry: %+v", fetched)
	}

	// 4. Test Claiming unpromoted entry -> 400 Bad Request
	claimReq := httptest.NewRequest(http.MethodPost, "/waitlist/"+created.ID+"/claim", nil)
	claimRR := httptest.NewRecorder()
	srv.handleWaitlistClaim(claimRR, claimReq)

	if claimRR.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 when claiming unpromoted entry, got %d", claimRR.Code)
	}

	// 5. Test DELETE /waitlist/{id}
	delReq := httptest.NewRequest(http.MethodDelete, "/waitlist/"+created.ID, nil)
	delRR := httptest.NewRecorder()
	srv.handleWaitlistByID(delRR, delReq)

	if delRR.Code != http.StatusOK {
		t.Fatalf("Expected status 200 on DELETE /waitlist/{id}, got %d: %s", delRR.Code, delRR.Body.String())
	}
}

func TestWaitlistPromotionTrigger(t *testing.T) {
	db := getTestDB(t)
	if db == nil {
		return
	}
	defer db.Close()

	srv := &Server{
		db:         db,
		httpClient: &http.Client{},
	}
	ctx := context.Background()
	_ = srv.initWaitlistSchema(ctx)

	// Clean up any test users
	_, _ = db.Exec("DELETE FROM waitlist WHERE user_id LIKE 'user-trigger-%'")
	defer func() {
		_, _ = db.Exec("DELETE FROM waitlist WHERE user_id LIKE 'user-trigger-%'")
	}()

	// 1. Join waitlist as user-trigger-1
	entry := &WaitlistEntry{
		EventID:       "evt-101",
		TierID:        "tier-vip",
		UserID:        "user-trigger-1",
		CustomerName:  "Trigger Tester",
		CustomerEmail: "trigger@test.com",
		Quantity:      1,
		Status:        WaitlistStatusPending,
	}
	created, err := srv.CreateWaitlistEntry(ctx, entry)
	if err != nil {
		t.Fatalf("Failed to create waitlist entry: %v", err)
	}

	so := &SagaOrchestrator{
		srv: srv,
		rmq: nil,
	}

	// 2. Invoke TriggerWaitlistPromotion
	err = so.TriggerWaitlistPromotion(ctx, "corr-trigger-test", "evt-101", "tier-vip", []string{"A1"})
	if err != nil {
		t.Fatalf("TriggerWaitlistPromotion failed: %v", err)
	}

	// 3. Verify user-trigger-1 entry is now PROMOTED with a valid reservation ID
	promoted, err := srv.GetWaitlistEntry(ctx, created.ID)
	if err != nil {
		t.Fatalf("Failed to fetch waitlist entry after trigger: %v", err)
	}
	if promoted.Status != WaitlistStatusPromoted {
		t.Errorf("Expected status %s, got %s", WaitlistStatusPromoted, promoted.Status)
	}
	if promoted.ReservationID == "" {
		t.Errorf("Expected non-empty ReservationID on promoted entry")
	}
}


