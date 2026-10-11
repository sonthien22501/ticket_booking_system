package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

type AuthResponse struct {
	Token string `json:"token"`
	User  struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		Role      string `json:"role"`
		CreatedAt string `json:"createdAt"`
	} `json:"user"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

type VerifyResponse struct {
	Valid bool `json:"valid"`
	User  struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	} `json:"user"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

func TestAuthRegistration(t *testing.T) {
	uniqueEmail := fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())

	// 1. Successful registration
	payload := map[string]string{
		"email":    uniqueEmail,
		"password": "securepassword123",
		"name":     "E2E Test User",
		"role":     "customer",
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(gatewayURL+"/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Register request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201 Created, got %d", resp.StatusCode)
	}

	var authRes AuthResponse
	json.NewDecoder(resp.Body).Decode(&authRes)

	if authRes.Token == "" {
		t.Errorf("Expected non-empty JWT token upon registration")
	}
	if authRes.User.Email != uniqueEmail {
		t.Errorf("Expected email %s, got %s", uniqueEmail, authRes.User.Email)
	}
	if authRes.User.Role != "customer" {
		t.Errorf("Expected role 'customer', got %s", authRes.User.Role)
	}

	// 2. Duplicate registration attempt should return 409 Conflict
	respDup, err := http.Post(gatewayURL+"/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Duplicate register request failed: %v", err)
	}
	defer respDup.Body.Close()

	if respDup.StatusCode != http.StatusConflict {
		t.Errorf("Expected status 409 Conflict for duplicate email, got %d", respDup.StatusCode)
	}

	// 3. Weak password rejection
	weakPayload := map[string]string{
		"email":    fmt.Sprintf("weak-%d@example.com", time.Now().UnixNano()),
		"password": "123",
		"name":     "Weak User",
	}
	weakBody, _ := json.Marshal(weakPayload)
	respWeak, _ := http.Post(gatewayURL+"/auth/register", "application/json", bytes.NewReader(weakBody))
	defer respWeak.Body.Close()

	if respWeak.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request for short password, got %d", respWeak.StatusCode)
	}
}

func TestAuthLogin(t *testing.T) {
	// 1. Successful login with seeded default user
	loginPayload := map[string]string{
		"email":    "user@tickets.com",
		"password": "user123",
	}
	body, _ := json.Marshal(loginPayload)

	resp, err := http.Post(gatewayURL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", resp.StatusCode)
	}

	var authRes AuthResponse
	json.NewDecoder(resp.Body).Decode(&authRes)

	if authRes.Token == "" {
		t.Errorf("Expected JWT token from login")
	}
	if authRes.User.Email != "user@tickets.com" {
		t.Errorf("Expected email 'user@tickets.com', got %s", authRes.User.Email)
	}

	// 2. Failed login with invalid password
	invalidPayload := map[string]string{
		"email":    "user@tickets.com",
		"password": "wrongpassword!",
	}
	invBody, _ := json.Marshal(invalidPayload)

	respInv, err := http.Post(gatewayURL+"/auth/login", "application/json", bytes.NewReader(invBody))
	if err != nil {
		t.Fatalf("Failed to execute invalid login request: %v", err)
	}
	defer respInv.Body.Close()

	if respInv.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized for wrong password, got %d", respInv.StatusCode)
	}

	// 3. Failed login with nonexistent user
	noUserPayload := map[string]string{
		"email":    "nonexistent-user@tickets.com",
		"password": "password123",
	}
	noUserBody, _ := json.Marshal(noUserPayload)

	respNoUser, _ := http.Post(gatewayURL+"/auth/login", "application/json", bytes.NewReader(noUserBody))
	defer respNoUser.Body.Close()

	if respNoUser.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized for nonexistent user, got %d", respNoUser.StatusCode)
	}
}

func TestTokenVerification(t *testing.T) {
	// Login to get a valid token
	loginPayload := map[string]string{
		"email":    "user@tickets.com",
		"password": "user123",
	}
	body, _ := json.Marshal(loginPayload)
	resp, _ := http.Post(gatewayURL+"/auth/login", "application/json", bytes.NewReader(body))
	var authRes AuthResponse
	json.NewDecoder(resp.Body).Decode(&authRes)
	resp.Body.Close()

	validToken := authRes.Token

	// 1. Verify with valid token
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, gatewayURL+"/auth/verify", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)

	verifyResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Verify request failed: %v", err)
	}
	defer verifyResp.Body.Close()

	if verifyResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK for valid token, got %d", verifyResp.StatusCode)
	}

	var verifyResult VerifyResponse
	json.NewDecoder(verifyResp.Body).Decode(&verifyResult)

	if !verifyResult.Valid {
		t.Errorf("Expected valid == true")
	}
	if verifyResult.User.Email != "user@tickets.com" {
		t.Errorf("Expected user email 'user@tickets.com', got %s", verifyResult.User.Email)
	}

	// 2. Verify with missing token
	reqNoToken, _ := http.NewRequest(http.MethodGet, gatewayURL+"/auth/verify", nil)
	respNoToken, _ := client.Do(reqNoToken)
	defer respNoToken.Body.Close()

	if respNoToken.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized for missing token, got %d", respNoToken.StatusCode)
	}

	// 3. Verify with invalid/tampered token
	reqTampered, _ := http.NewRequest(http.MethodGet, gatewayURL+"/auth/verify", nil)
	reqTampered.Header.Set("Authorization", "Bearer invalid.jwt.token.string")
	respTampered, _ := client.Do(reqTampered)
	defer respTampered.Body.Close()

	if respTampered.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized for tampered token, got %d", respTampered.StatusCode)
	}
}

func TestRoleBasedAuthorization(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// 1. Get customer token
	custLogin, _ := json.Marshal(map[string]string{"email": "user@tickets.com", "password": "user123"})
	custResp, _ := http.Post(gatewayURL+"/auth/login", "application/json", bytes.NewReader(custLogin))
	var custAuth AuthResponse
	json.NewDecoder(custResp.Body).Decode(&custAuth)
	custResp.Body.Close()

	// 2. Get admin token
	adminLogin, _ := json.Marshal(map[string]string{"email": "admin@tickets.com", "password": "admin123"})
	adminResp, _ := http.Post(gatewayURL+"/auth/login", "application/json", bytes.NewReader(adminLogin))
	var adminAuth AuthResponse
	json.NewDecoder(adminResp.Body).Decode(&adminAuth)
	adminResp.Body.Close()

	// 3. Customer attempts to access admin-only endpoint: /api/auth/users
	reqCust, _ := http.NewRequest(http.MethodGet, gatewayURL+"/auth/users", nil)
	reqCust.Header.Set("Authorization", "Bearer "+custAuth.Token)
	respCust, err := client.Do(reqCust)
	if err != nil {
		t.Fatalf("Customer request to admin endpoint failed: %v", err)
	}
	defer respCust.Body.Close()

	if respCust.StatusCode != http.StatusForbidden {
		t.Errorf("Expected status 403 Forbidden for customer, got %d", respCust.StatusCode)
	}

	// 4. Admin accesses admin-only endpoint: /api/auth/users
	reqAdmin, _ := http.NewRequest(http.MethodGet, gatewayURL+"/auth/users", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+adminAuth.Token)
	respAdmin, err := client.Do(reqAdmin)
	if err != nil {
		t.Fatalf("Admin request to admin endpoint failed: %v", err)
	}
	defer respAdmin.Body.Close()

	if respAdmin.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK for admin, got %d", respAdmin.StatusCode)
	}

	var users []map[string]interface{}
	json.NewDecoder(respAdmin.Body).Decode(&users)

	if len(users) < 2 {
		t.Errorf("Expected at least 2 users returned, got %d", len(users))
	}
}

func TestAuthenticatedBookingFlow(t *testing.T) {
	// Register a dedicated user for this test
	uniqueEmail := fmt.Sprintf("booker-%d@example.com", time.Now().UnixNano())
	regBody, _ := json.Marshal(map[string]string{
		"email":    uniqueEmail,
		"password": "password123",
		"name":     "Jane Booker",
		"role":     "customer",
	})
	regResp, err := http.Post(gatewayURL+"/auth/register", "application/json", bytes.NewReader(regBody))
	if err != nil {
		t.Fatalf("Registration failed: %v", err)
	}
	var authRes AuthResponse
	json.NewDecoder(regResp.Body).Decode(&authRes)
	regResp.Body.Close()

	userToken := authRes.Token
	userID := authRes.User.ID

	// Find an available seat
	eventID := getBaseEvent(t)
	seats := getAvailableSeats(t, eventID)
	if len(seats) == 0 {
		t.Skip("No available seats found for event")
	}
	targetSeat := seats[0]

	// Book seat with JWT token
	bookingPayload := map[string]interface{}{
		"eventId": eventID,
		"seats":   []string{targetSeat},
		"paymentInfo": map[string]string{
			"cardToken": "tok_visa",
		},
	}
	bookBody, _ := json.Marshal(bookingPayload)

	req, _ := http.NewRequest(http.MethodPost, gatewayURL+"/bookings", bytes.NewReader(bookBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)

	client := &http.Client{Timeout: 30 * time.Second}
	bookResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Booking request failed: %v", err)
	}
	defer bookResp.Body.Close()

	if bookResp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201 Created, got %d", bookResp.StatusCode)
	}

	var booking map[string]interface{}
	json.NewDecoder(bookResp.Body).Decode(&booking)

	bookingID, _ := booking["bookingId"].(string)
	if bookingID == "" {
		bookingID, _ = booking["id"].(string)
	}

	// Verify that the booking is securely attached to user identity
	bUserID, _ := booking["userId"].(string)
	bEmail, _ := booking["customerEmail"].(string)
	bName, _ := booking["customerName"].(string)

	if bUserID != userID {
		t.Errorf("Expected booking userId %s, got %s", userID, bUserID)
	}
	if !strings.EqualFold(bEmail, uniqueEmail) {
		t.Errorf("Expected booking customerEmail %s, got %s", uniqueEmail, bEmail)
	}
	if bName != "Jane Booker" {
		t.Errorf("Expected booking customerName 'Jane Booker', got %s", bName)
	}

	// Fetch user's bookings using token and verify this booking is returned
	reqList, _ := http.NewRequest(http.MethodGet, gatewayURL+"/bookings", nil)
	reqList.Header.Set("Authorization", "Bearer "+userToken)
	listResp, err := client.Do(reqList)
	if err != nil {
		t.Fatalf("Failed to fetch user bookings: %v", err)
	}
	defer listResp.Body.Close()

	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200 OK, got %d", listResp.StatusCode)
	}

	var userBookings []map[string]interface{}
	json.NewDecoder(listResp.Body).Decode(&userBookings)

	found := false
	for _, ub := range userBookings {
		if ub["id"] == bookingID || ub["bookingId"] == bookingID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to find booking %s in user's booking list", bookingID)
	}
}
