package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCalculateDiscount_Percentage(t *testing.T) {
	v := &Voucher{
		Code:          "EARLYBIRD20",
		DiscountType:  DiscountTypePercentage,
		DiscountValue: 20.0,
		MinSpend:      50.0,
		Active:        true,
	}

	discount, finalTotal, err := v.CalculateDiscount(150.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if discount != 30.0 {
		t.Errorf("expected discount 30.0, got %f", discount)
	}
	if finalTotal != 120.0 {
		t.Errorf("expected finalTotal 120.0, got %f", finalTotal)
	}
}

func TestCalculateDiscount_Fixed(t *testing.T) {
	v := &Voucher{
		Code:          "SAVE15",
		DiscountType:  DiscountTypeFixed,
		DiscountValue: 15.0,
		MinSpend:      30.0,
		Active:        true,
	}

	discount, finalTotal, err := v.CalculateDiscount(60.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if discount != 15.0 {
		t.Errorf("expected discount 15.0, got %f", discount)
	}
	if finalTotal != 45.0 {
		t.Errorf("expected finalTotal 45.0, got %f", finalTotal)
	}
}

func TestCalculateDiscount_Clamped(t *testing.T) {
	v := &Voucher{
		Code:          "BIGOFF100",
		DiscountType:  DiscountTypeFixed,
		DiscountValue: 100.0,
		MinSpend:      0.0,
		Active:        true,
	}

	discount, finalTotal, err := v.CalculateDiscount(40.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if discount != 40.0 {
		t.Errorf("expected discount clamped to 40.0, got %f", discount)
	}
	if finalTotal != 0.0 {
		t.Errorf("expected finalTotal 0.0, got %f", finalTotal)
	}
}

func TestValidateVoucher_MinSpendNotMet(t *testing.T) {
	v := &Voucher{
		Code:          "MINSPEND50",
		DiscountType:  DiscountTypePercentage,
		DiscountValue: 10.0,
		MinSpend:      50.0,
		Active:        true,
	}

	_, _, err := v.CalculateDiscount(35.0)
	if err == nil {
		t.Fatalf("expected error for subtotal below minSpend, got nil")
	}
}

func TestValidateVoucher_Expired(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	v := &Voucher{
		Code:          "EXPIRED",
		DiscountType:  DiscountTypeFixed,
		DiscountValue: 10.0,
		Active:        true,
		ExpiresAt:     &past,
	}

	if err := v.Validate(); err == nil {
		t.Fatalf("expected validation error for expired voucher, got nil")
	}
}

func TestValidateVoucher_MaxUsesExceeded(t *testing.T) {
	v := &Voucher{
		Code:          "LIMITREACHED",
		DiscountType:  DiscountTypeFixed,
		DiscountValue: 10.0,
		MaxUses:       5,
		CurrentUses:   5,
		Active:        true,
	}

	if err := v.Validate(); err == nil {
		t.Fatalf("expected validation error for max uses exceeded, got nil")
	}
}

func TestHandleValidateVoucher_HTTP(t *testing.T) {
	s := &Server{db: nil}

	payload := ValidateVoucherRequest{
		Code:     "EARLYBIRD20",
		Subtotal: 100.0,
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/vouchers/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	s.handleValidateVoucher(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d, body: %s", rr.Code, rr.Body.String())
	}

	var res ValidateVoucherResponse
	if err := json.NewDecoder(rr.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if !res.Valid {
		t.Errorf("expected voucher to be valid, got false")
	}
	if res.DiscountAmount != 20.0 {
		t.Errorf("expected discountAmount 20.0, got %f", res.DiscountAmount)
	}
	if res.FinalTotal != 80.0 {
		t.Errorf("expected finalTotal 80.0, got %f", res.FinalTotal)
	}
}

func TestHandleAdminVouchers_HTTP(t *testing.T) {
	s := &Server{db: nil}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/vouchers", nil)
	rr := httptest.NewRecorder()

	s.handleAdminVouchers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", rr.Code)
	}

	var list []Voucher
	if err := json.NewDecoder(rr.Body).Decode(&list); err != nil {
		t.Fatalf("failed to decode json response: %v", err)
	}

	if len(list) == 0 {
		t.Errorf("expected default seeded vouchers in fallback, got 0")
	}
}
