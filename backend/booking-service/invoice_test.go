package main

import (
	"bytes"
	"math"
	"regexp"
	"testing"
	"time"
)

func TestInvoicePDFTextMatrixPositioning(t *testing.T) {
	inv := &Invoice{
		InvoiceNumber:    "INV-TEST-001",
		BookingReference: "REF-TEST-001",
		CustomerName:     "Alice Tester",
		CustomerEmail:    "alice@example.com",
		EventTitle:       "Symphony Night",
		Currency:         "USD",
		CurrencySymbol:   "$",
		ExchangeRate:     1.0,
		TaxRate:          0.085,
		TaxAmount:        12.75,
		Subtotal:         150.00,
		TaxableAmount:    150.00,
		TotalAmount:      162.75,
		IssueDate:        time.Now().Format("2006-01-02"),
		DueDate:          time.Now().Format("2006-01-02"),
		Status:           "PAID",
		Items: []InvoiceItem{
			{
				Description: "VIP Tier",
				SeatID:      "A6",
				Quantity:    1,
				UnitPrice:   150.00,
				TotalPrice:  150.00,
			},
		},
	}

	pdf, err := GenerateInvoicePDF(inv)
	if err != nil {
		t.Fatalf("GenerateInvoicePDF failed: %v", err)
	}

	// Practical assertion 1: Tm operators are used for absolute positioning of columns & summary
	if !bytes.Contains(pdf, []byte("1 0 0 1 470")) {
		t.Errorf("expected PDF to contain '1 0 0 1 470' Tm positioning operator")
	}
	if !bytes.Contains(pdf, []byte("1 0 0 1 268")) {
		t.Errorf("expected PDF to contain '1 0 0 1 268' Tm positioning operator")
	}

	// Practical assertion 2: Absolute Td for those columns inside BT...ET is absent
	re := regexp.MustCompile(`\n(268|340|400|470|460) -?\d+ Td`)
	if re.Match(pdf) {
		t.Errorf("found offending absolute Td operator in PDF stream: %s", re.Find(pdf))
	}

	// Practical assertion 3: Formatted prices exist
	if !bytes.Contains(pdf, []byte("($150.00) Tj")) {
		t.Errorf("expected PDF to contain '($150.00) Tj'")
	}
	if !bytes.Contains(pdf, []byte("($162.75) Tj")) {
		t.Errorf("expected PDF to contain '($162.75) Tj'")
	}
}

func TestBuildInvoiceNoDoubleTax_USD(t *testing.T) {
	booking := &Booking{
		ID:               "bkg-demo-001",
		BookingReference: "REF-NEON-2026-001",
		CustomerName:     "Alex Morgan",
		CustomerEmail:    "user@tickets.com",
		EventTitle:       "Neon Symphony 2026",
		Quantity:         1,
		SeatIDs:          []string{"B1"},
		TotalAmount:      162.75, // Grand total actually paid (tax-inclusive)
		DiscountAmount:   0.00,
		Status:           "CONFIRMED",
		PaymentStatus:    "PAID",
		CreatedAt:        time.Now(),
	}

	inv := BuildInvoice(booking, nil, "USD", 1.0, 0.085, 12.75, "", "", "")
	if inv.Subtotal != 150.00 {
		t.Errorf("expected subtotal 150.00 USD, got %f", inv.Subtotal)
	}
	if inv.TaxAmount != 12.75 {
		t.Errorf("expected taxAmount 12.75 USD, got %f", inv.TaxAmount)
	}
	if inv.TotalAmount != 162.75 {
		t.Errorf("expected totalAmount 162.75 USD, got %f", inv.TotalAmount)
	}
	// Invariant: Subtotal - Discount + TaxAmount == TotalAmount (±0.01)
	diff := math.Abs((inv.Subtotal - inv.Discount + inv.TaxAmount) - inv.TotalAmount)
	if diff > 0.01 {
		t.Errorf("invariant violated: Subtotal (%f) - Discount (%f) + Tax (%f) != Total (%f), diff=%f",
			inv.Subtotal, inv.Discount, inv.TaxAmount, inv.TotalAmount, diff)
	}
}

func TestBuildInvoiceNoDoubleTax_WithVoucher(t *testing.T) {
	booking := &Booking{
		ID:               "bkg-chen-007",
		BookingReference: "REF-NEON-2026-007",
		CustomerName:     "Alex Chen",
		CustomerEmail:    "alex.chen@technova.com",
		EventTitle:       "Neon Symphony 2026",
		Quantity:         2,
		SeatIDs:          []string{"C1", "C2"},
		TotalAmount:      104.16, // Grand total actually paid (120 pre-tax - 24 discount + 8.16 tax)
		DiscountAmount:   24.00,
		VoucherCode:      "EARLYBIRD20",
		Status:           "CONFIRMED",
		PaymentStatus:    "PAID",
		CreatedAt:        time.Now(),
	}

	inv := BuildInvoice(booking, nil, "USD", 1.0, 0.085, 8.16, "", "", "")
	if inv.Subtotal != 120.00 {
		t.Errorf("expected subtotal 120.00 USD, got %f", inv.Subtotal)
	}
	if inv.Discount != 24.00 {
		t.Errorf("expected discount 24.00 USD, got %f", inv.Discount)
	}
	if inv.TaxAmount != 8.16 {
		t.Errorf("expected taxAmount 8.16 USD, got %f", inv.TaxAmount)
	}
	if inv.TotalAmount != 104.16 {
		t.Errorf("expected totalAmount 104.16 USD, got %f", inv.TotalAmount)
	}
	diff := math.Abs((inv.Subtotal - inv.Discount + inv.TaxAmount) - inv.TotalAmount)
	if diff > 0.01 {
		t.Errorf("invariant violated: Subtotal (%f) - Discount (%f) + Tax (%f) != Total (%f), diff=%f",
			inv.Subtotal, inv.Discount, inv.TaxAmount, inv.TotalAmount, diff)
	}
}

func TestApplyTax_BookingCreation(t *testing.T) {
	// A $150 VIP seat in USD with 8.5% tax:
	totalInclusive, taxRate, taxAmount := applyTax(150.00, "USD")
	if taxRate != 0.085 {
		t.Errorf("expected taxRate 0.085, got %f", taxRate)
	}
	if taxAmount != 12.75 {
		t.Errorf("expected taxAmount 12.75, got %f", taxAmount)
	}
	if totalInclusive != 162.75 {
		t.Errorf("expected tax-inclusive total 162.75, got %f", totalInclusive)
	}
}
