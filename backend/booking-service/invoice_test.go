package main

import (
	"bytes"
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
