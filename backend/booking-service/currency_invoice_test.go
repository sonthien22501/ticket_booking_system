package main

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCurrencyConversion(t *testing.T) {
	// 1. USD to EUR: $100 * 0.92 = €92.00
	converted, rate, err := ConvertCurrency(100.0, "USD", "EUR")
	if err != nil {
		t.Fatalf("unexpected error converting USD to EUR: %v", err)
	}
	if rate <= 0 {
		t.Fatalf("expected positive exchange rate, got %f", rate)
	}
	expectedEUR := math.Round(100.0*rate*100) / 100
	if converted != expectedEUR {
		t.Errorf("expected %f EUR, got %f", expectedEUR, converted)
	}

	// 2. Zero-decimal JPY precision: $100 * 155.0 = 15500 JPY (no decimals)
	convertedJPY, rateJPY, err := ConvertCurrency(100.0, "USD", "JPY")
	if err != nil {
		t.Fatalf("unexpected error converting USD to JPY: %v", err)
	}
	if rateJPY <= 0 {
		t.Fatalf("expected positive exchange rate for JPY, got %f", rateJPY)
	}
	if convertedJPY != math.Floor(convertedJPY) {
		t.Errorf("JPY should have 0 decimal places, got %f", convertedJPY)
	}

	// 3. Same currency identity conversion: $50 USD to USD = $50
	convertedSame, rateSame, err := ConvertCurrency(50.0, "USD", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rateSame != 1.0 || convertedSame != 50.0 {
		t.Errorf("expected 1.0 and 50.0, got rate=%f, converted=%f", rateSame, convertedSame)
	}

	// 4. Invalid currency error
	_, _, err = ConvertCurrency(100.0, "USD", "INVALID_XYZ")
	if err == nil {
		t.Errorf("expected error for unsupported currency INVALID_XYZ, got nil")
	}
}

func TestTaxCalculation(t *testing.T) {
	// 1. Standard tax on full subtotal: $100 in EUR (20% VAT) -> Taxable: 100, Tax: 20, Rate: 0.20
	taxRate, taxAmount, taxableAmount := CalculateTax(100.0, 0.0, "EUR")
	if taxRate != 0.20 {
		t.Errorf("expected 0.20 tax rate for EUR, got %f", taxRate)
	}
	if taxableAmount != 100.0 {
		t.Errorf("expected taxable amount 100.0, got %f", taxableAmount)
	}
	if taxAmount != 20.0 {
		t.Errorf("expected 20.0 tax amount, got %f", taxAmount)
	}

	// 2. Tax on voucher-discounted subtotal: Subtotal $100, Discount $30 -> Taxable: $70 -> Tax: $14 in EUR
	taxRateDisc, taxAmountDisc, taxableAmountDisc := CalculateTax(100.0, 30.0, "EUR")
	if taxableAmountDisc != 70.0 {
		t.Errorf("expected taxable amount 70.0 after discount, got %f", taxableAmountDisc)
	}
	if taxAmountDisc != 14.0 {
		t.Errorf("expected tax amount 14.0, got %f", taxAmountDisc)
	}
	if taxRateDisc != 0.20 {
		t.Errorf("expected tax rate 0.20, got %f", taxRateDisc)
	}

	// 3. Discount >= Subtotal clamping: Subtotal $50, Discount $60 -> Taxable: $0 -> Tax: $0
	_, taxAmountZero, taxableZero := CalculateTax(50.0, 60.0, "USD")
	if taxableZero != 0.0 || taxAmountZero != 0.0 {
		t.Errorf("expected 0 tax on fully discounted subtotal, got taxable=%f tax=%f", taxableZero, taxAmountZero)
	}

	// 4. Default currency USD tax rate check
	rateUSD, _, _ := CalculateTax(100.0, 0.0, "USD")
	if rateUSD <= 0 {
		t.Errorf("expected positive tax rate for USD, got %f", rateUSD)
	}
}

func TestCurrencyHTTPHandlers(t *testing.T) {
	srv := &Server{}

	// Test GET /api/currencies/rates
	reqRates := httptest.NewRequest(http.MethodGet, "/api/currencies/rates", nil)
	recRates := httptest.NewRecorder()
	srv.handleCurrencyRates(recRates, reqRates)

	if recRates.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from rates endpoint, got %d", recRates.Code)
	}
	var ratesResp CurrencyRatesResponse
	if err := json.Unmarshal(recRates.Body.Bytes(), &ratesResp); err != nil {
		t.Fatalf("failed to parse rates JSON: %v", err)
	}
	if ratesResp.Base != "USD" {
		t.Errorf("expected base USD, got %s", ratesResp.Base)
	}
	if len(ratesResp.Rates) < 4 {
		t.Errorf("expected at least 4 currency rates, got %d", len(ratesResp.Rates))
	}
	if _, ok := ratesResp.Rates["EUR"]; !ok {
		t.Errorf("expected EUR rate in response")
	}

	// Test POST /api/currencies/convert
	convertPayload := ConvertCurrencyRequest{
		Amount: 150.0,
		From:   "USD",
		To:     "EUR",
	}
	bodyBytes, _ := json.Marshal(convertPayload)
	reqConvert := httptest.NewRequest(http.MethodPost, "/api/currencies/convert", bytes.NewReader(bodyBytes))
	recConvert := httptest.NewRecorder()
	srv.handleCurrencyConvert(recConvert, reqConvert)

	if recConvert.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from convert endpoint, got %d", recConvert.Code)
	}
	var convResp ConvertCurrencyResponse
	if err := json.Unmarshal(recConvert.Body.Bytes(), &convResp); err != nil {
		t.Fatalf("failed to parse convert JSON: %v", err)
	}
	if convResp.From != "USD" || convResp.To != "EUR" {
		t.Errorf("expected USD to EUR conversion, got %s to %s", convResp.From, convResp.To)
	}
	if convResp.ConvertedAmount <= 0 {
		t.Errorf("expected positive converted amount, got %f", convResp.ConvertedAmount)
	}
}

func TestPDFInvoiceGeneration(t *testing.T) {
	booking := &Booking{
		ID:               "bk-test-12345",
		BookingReference: "REF-TEST-12345",
		UserID:           "usr-789",
		CustomerName:     "Acme International Corp",
		CustomerEmail:    "billing@acme.org",
		EventID:          "ev-concert-01",
		EventTitle:       "Cosmic Symphony 2026",
		Quantity:         2,
		SeatIDs:          []string{"A1", "A2"},
		TotalAmount:      184.00,
		Status:           "CONFIRMED",
		PaymentStatus:    "COMPLETED",
		CreatedAt:        time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC),
	}

	tickets := []Ticket{
		{ID: "t-1", BookingID: "bk-test-12345", SeatID: "A1", Price: 80.00, Status: "ISSUED"},
		{ID: "t-2", BookingID: "bk-test-12345", SeatID: "A2", Price: 80.00, Status: "ISSUED"},
	}

	inv := BuildInvoice(booking, tickets, "EUR", 0.92, 0.20, 24.00, "Acme International Corp", "EU-TAX-98765432", "100 Innovation Way, Suite 400")
	if inv == nil {
		t.Fatalf("expected non-nil invoice object")
	}
	if inv.InvoiceNumber == "" {
		t.Errorf("expected non-empty invoice number")
	}
	if inv.CompanyName != "Acme International Corp" {
		t.Errorf("expected company name 'Acme International Corp', got '%s'", inv.CompanyName)
	}
	if inv.TaxID != "EU-TAX-98765432" {
		t.Errorf("expected TaxID 'EU-TAX-98765432', got '%s'", inv.TaxID)
	}
	if len(inv.Items) != 2 {
		t.Errorf("expected 2 invoice items, got %d", len(inv.Items))
	}

	// Generate PDF
	pdfBytes, err := GenerateInvoicePDF(inv)
	if err != nil {
		t.Fatalf("failed to generate invoice PDF: %v", err)
	}
	if len(pdfBytes) < 200 {
		t.Fatalf("PDF byte slice suspiciously short: %d bytes", len(pdfBytes))
	}

	// Verify valid PDF standard header
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-1.4")) {
		t.Errorf("PDF does not start with standard magic bytes %%PDF-1.4")
	}

	// Verify PDF contains EOF marker
	if !strings.Contains(string(pdfBytes), "%%EOF") {
		t.Errorf("PDF output does not contain %%EOF trailer marker")
	}

	// Verify company name & invoice number are embedded in PDF stream
	pdfContent := string(pdfBytes)
	if !strings.Contains(pdfContent, inv.InvoiceNumber) {
		t.Errorf("PDF should contain invoice number %s", inv.InvoiceNumber)
	}
	if !strings.Contains(pdfContent, "Acme International Corp") {
		t.Errorf("PDF should contain company name")
	}
}

func TestInvoiceWithDiscountsAndTax(t *testing.T) {
	// Subtotal: 100 USD, Voucher discount: 20 USD -> Taxable: 80 USD
	// Currency: GBP (Rate: 0.79, VAT: 20%)
	// Subtotal GBP = 79.00, Discount GBP = 15.80, Taxable GBP = 63.20, Tax (20%) = 12.64, Total = 75.84
	booking := &Booking{
		ID:               "bk-disc-001",
		BookingReference: "REF-DISC-001",
		CustomerName:     "Tech Corp UK",
		CustomerEmail:    "accounts@techcorp.co.uk",
		EventTitle:       "DevCon London 2026",
		Quantity:         1,
		SeatIDs:          []string{"B1"},
		TotalAmount:      80.00,
		DiscountAmount:   20.00,
		VoucherCode:      "TECHSAVE20",
		Status:           "CONFIRMED",
		PaymentStatus:    "COMPLETED",
		CreatedAt:        time.Now(),
	}

	inv := BuildInvoice(booking, nil, "GBP", 0.79, 0.20, 12.64, "Tech Corp UK", "GB999888777", "10 Downing Street, London")
	if inv.Discount != 15.80 {
		t.Errorf("expected discount 15.80 GBP, got %f", inv.Discount)
	}
	if inv.TaxableAmount != 63.20 {
		t.Errorf("expected taxable amount 63.20 GBP, got %f", inv.TaxableAmount)
	}
	if inv.TaxAmount != 12.64 {
		t.Errorf("expected tax amount 12.64 GBP, got %f", inv.TaxAmount)
	}
	if inv.TotalAmount != 75.84 {
		t.Errorf("expected total amount 75.84 GBP, got %f", inv.TotalAmount)
	}

	pdfBytes, err := GenerateInvoicePDF(inv)
	if err != nil {
		t.Fatalf("failed to generate PDF: %v", err)
	}
	if !strings.Contains(string(pdfBytes), "TECHSAVE20") && !strings.Contains(string(pdfBytes), "Voucher Discount") {
		t.Errorf("expected PDF to mention voucher discount")
	}
}

func TestCurrencyRegistryCompleteness(t *testing.T) {
	requiredCurrencies := []string{"USD", "EUR", "GBP", "JPY", "CAD", "AUD"}
	for _, code := range requiredCurrencies {
		info, ok := currencyRegistry[code]
		if !ok {
			t.Fatalf("missing required currency in registry: %s", code)
		}
		if info.Symbol == "" {
			t.Errorf("currency %s must have non-empty symbol", code)
		}
		if info.Rate <= 0 {
			t.Errorf("currency %s must have positive exchange rate", code)
		}
		if info.TaxRate < 0 || info.TaxRate > 0.5 {
			t.Errorf("currency %s tax rate %f is out of reasonable range", code, info.TaxRate)
		}
	}
}
