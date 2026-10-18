package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// Invoice represents a complete, audit-compliant digital tax invoice and admission receipt.
type Invoice struct {
	InvoiceNumber    string        `json:"invoiceNumber"`
	BookingID        string        `json:"bookingId"`
	BookingReference string        `json:"bookingReference"`
	IssueDate        string        `json:"issueDate"`
	DueDate          string        `json:"dueDate"`
	Status           string        `json:"status"` // PAID, PENDING, CANCELLED
	PaymentStatus    string        `json:"paymentStatus"`
	CustomerName     string        `json:"customerName"`
	CustomerEmail    string        `json:"customerEmail"`
	CompanyName      string        `json:"companyName,omitempty"`
	TaxID            string        `json:"taxId,omitempty"`
	BillingAddress   string        `json:"billingAddress,omitempty"`
	EventID          string        `json:"eventId"`
	EventTitle       string        `json:"eventTitle"`
	Currency         string        `json:"currency"`
	CurrencySymbol   string        `json:"currencySymbol"`
	ExchangeRate     float64       `json:"exchangeRate"`
	Subtotal         float64       `json:"subtotal"`
	Discount         float64       `json:"discount"`
	TaxableAmount    float64       `json:"taxableAmount"`
	TaxRate          float64       `json:"taxRate"`
	TaxAmount        float64       `json:"taxAmount"`
	TotalAmount      float64       `json:"totalAmount"`
	IssuerName       string        `json:"issuerName"`
	IssuerTaxID      string        `json:"issuerTaxId"`
	IssuerAddress    string        `json:"issuerAddress"`
	Items            []InvoiceItem `json:"items"`
}

// InvoiceItem represents an individual line item on the invoice.
type InvoiceItem struct {
	Description string  `json:"description"`
	SeatID      string  `json:"seatId"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unitPrice"`
	TotalPrice  float64 `json:"totalPrice"`
}

// BuildInvoice constructs a populated Invoice object from booking details.
func BuildInvoice(booking *Booking, tickets []Ticket, currency string, rate, taxRate, taxAmount float64, companyName, taxID, billingAddress string) *Invoice {
	if currency == "" {
		currency = "USD"
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if rate <= 0 {
		rate = 1.0
	}

	info, ok := currencyRegistry[currency]
	if !ok {
		info = currencyRegistry["USD"]
	}

	// Invoice Number format: INV-YYYYMMDD-<REF>
	datePrefix := booking.CreatedAt.Format("20060102")
	refSuffix := booking.BookingReference
	if len(refSuffix) > 12 {
		refSuffix = refSuffix[len(refSuffix)-8:]
	}
	invoiceNum := fmt.Sprintf("INV-%s-%s", datePrefix, refSuffix)

	subtotalUSD := booking.TotalAmount
	if booking.DiscountAmount > 0 {
		subtotalUSD += booking.DiscountAmount
	}
	subtotalCurr := subtotalUSD * rate

	discountUSD := booking.DiscountAmount
	discountCurr := discountUSD * rate

	taxableCurr := subtotalCurr - discountCurr
	if taxableCurr < 0 {
		taxableCurr = 0
	}

	totalCurr := taxableCurr + taxAmount

	if info.Decimals == 0 {
		subtotalCurr = math.Round(subtotalCurr)
		discountCurr = math.Round(discountCurr)
		taxableCurr = math.Round(taxableCurr)
		taxAmount = math.Round(taxAmount)
		totalCurr = math.Round(totalCurr)
	} else {
		subtotalCurr = math.Round(subtotalCurr*100) / 100
		discountCurr = math.Round(discountCurr*100) / 100
		taxableCurr = math.Round(taxableCurr*100) / 100
		taxAmount = math.Round(taxAmount*100) / 100
		totalCurr = math.Round(totalCurr*100) / 100
	}

	var items []InvoiceItem
	if len(tickets) > 0 {
		for _, t := range tickets {
			itemPriceUSD := t.Price
			if itemPriceUSD <= 0 {
				itemPriceUSD = subtotalUSD / float64(len(tickets))
			}
			itemPriceCurr := itemPriceUSD * rate
			if info.Decimals == 0 {
				itemPriceCurr = math.Round(itemPriceCurr)
			} else {
				itemPriceCurr = math.Round(itemPriceCurr*100) / 100
			}
			desc := fmt.Sprintf("Event Admission: %s (Seat %s)", booking.EventTitle, t.SeatID)
			items = append(items, InvoiceItem{
				Description: desc,
				SeatID:      t.SeatID,
				Quantity:    1,
				UnitPrice:   itemPriceCurr,
				TotalPrice:  itemPriceCurr,
			})
		}
	} else if len(booking.SeatIDs) > 0 {
		unitPrice := subtotalCurr / float64(len(booking.SeatIDs))
		for _, seat := range booking.SeatIDs {
			items = append(items, InvoiceItem{
				Description: fmt.Sprintf("Event Admission: %s (Seat %s)", booking.EventTitle, seat),
				SeatID:      seat,
				Quantity:    1,
				UnitPrice:   unitPrice,
				TotalPrice:  unitPrice,
			})
		}
	} else {
		items = append(items, InvoiceItem{
			Description: fmt.Sprintf("General Admission: %s", booking.EventTitle),
			SeatID:      "GA",
			Quantity:    booking.Quantity,
			UnitPrice:   subtotalCurr / float64(math.Max(1, float64(booking.Quantity))),
			TotalPrice:  subtotalCurr,
		})
	}

	return &Invoice{
		InvoiceNumber:    invoiceNum,
		BookingID:        booking.ID,
		BookingReference: booking.BookingReference,
		IssueDate:        booking.CreatedAt.Format("2006-01-02 15:04:05 UTC"),
		DueDate:          booking.CreatedAt.Format("2006-01-02"),
		Status:           booking.Status,
		PaymentStatus:    booking.PaymentStatus,
		CustomerName:     booking.CustomerName,
		CustomerEmail:    booking.CustomerEmail,
		CompanyName:      companyName,
		TaxID:            taxID,
		BillingAddress:   billingAddress,
		EventID:          booking.EventID,
		EventTitle:       booking.EventTitle,
		Currency:         currency,
		CurrencySymbol:   info.Symbol,
		ExchangeRate:     rate,
		Subtotal:         subtotalCurr,
		Discount:         discountCurr,
		TaxableAmount:    taxableCurr,
		TaxRate:          taxRate,
		TaxAmount:        taxAmount,
		TotalAmount:      totalCurr,
		IssuerName:       "TicketVerse Global Entertainment Inc.",
		IssuerTaxID:      "EU-VAT-987654321",
		IssuerAddress:    "100 Innovation Plaza, Suite 800, Tech City",
		Items:            items,
	}
}

// escapePDF escapes parentheses and backslashes in PDF string literals.
func escapePDF(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	return s
}

// GenerateInvoicePDF produces a valid, spec-compliant PDF 1.4 binary document.
func GenerateInvoicePDF(inv *Invoice) ([]byte, error) {
	var streamBuf bytes.Buffer

	// Page coordinates: A4 is 595.28 x 841.89 points
	// Top margin: 800 down to 50
	streamBuf.WriteString("q\n")

	// Header background banner (dark navy accent: 0.08 0.11 0.18)
	streamBuf.WriteString("0.08 0.11 0.18 rg\n")
	streamBuf.WriteString("40 730 515 75 re\n")
	streamBuf.WriteString("f\n")

	// Header Text in White (1.0 1.0 1.0)
	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F2 18 Tf\n")
	streamBuf.WriteString("1.0 1.0 1.0 rg\n")
	streamBuf.WriteString("55 772 Td\n")
	streamBuf.WriteString("(" + escapePDF(inv.IssuerName) + ") Tj\n")
	streamBuf.WriteString("ET\n")

	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F1 10 Tf\n")
	streamBuf.WriteString("0.8 0.85 0.9 rg\n")
	streamBuf.WriteString("55 754 Td\n")
	streamBuf.WriteString("(OFFICIAL TAX INVOICE & ADMISSION RECEIPT) Tj\n")
	streamBuf.WriteString("ET\n")

	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F1 8 Tf\n")
	streamBuf.WriteString("0.7 0.75 0.8 rg\n")
	streamBuf.WriteString("55 740 Td\n")
	streamBuf.WriteString("(" + escapePDF(inv.IssuerAddress) + " | VAT ID: " + escapePDF(inv.IssuerTaxID) + ") Tj\n")
	streamBuf.WriteString("ET\n")

	// Metadata columns (Invoice Details & Customer / Corporate Billed To)
	// Left column: Bill To
	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F2 11 Tf\n")
	streamBuf.WriteString("0.1 0.15 0.25 rg\n")
	streamBuf.WriteString("45 700 Td\n")
	streamBuf.WriteString("(BILLED TO:) Tj\n")
	streamBuf.WriteString("ET\n")

	yPos := 685
	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F1 10 Tf\n")
	streamBuf.WriteString("0.15 0.15 0.15 rg\n")
	streamBuf.WriteString(fmt.Sprintf("45 %d Td\n", yPos))
	streamBuf.WriteString("(" + escapePDF(inv.CustomerName) + ") Tj\n")
	streamBuf.WriteString("ET\n")

	if inv.CompanyName != "" {
		yPos -= 14
		streamBuf.WriteString("BT\n")
		streamBuf.WriteString("/F2 9 Tf\n")
		streamBuf.WriteString("0.2 0.3 0.6 rg\n")
		streamBuf.WriteString(fmt.Sprintf("45 %d Td\n", yPos))
		streamBuf.WriteString("(Company: " + escapePDF(inv.CompanyName) + ") Tj\n")
		streamBuf.WriteString("ET\n")
	}

	if inv.TaxID != "" {
		yPos -= 14
		streamBuf.WriteString("BT\n")
		streamBuf.WriteString("/F1 9 Tf\n")
		streamBuf.WriteString("0.3 0.3 0.3 rg\n")
		streamBuf.WriteString(fmt.Sprintf("45 %d Td\n", yPos))
		streamBuf.WriteString("(Tax / VAT ID: " + escapePDF(inv.TaxID) + ") Tj\n")
		streamBuf.WriteString("ET\n")
	}

	yPos -= 14
	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F1 9 Tf\n")
	streamBuf.WriteString("0.3 0.3 0.3 rg\n")
	streamBuf.WriteString(fmt.Sprintf("45 %d Td\n", yPos))
	streamBuf.WriteString("(Email: " + escapePDF(inv.CustomerEmail) + ") Tj\n")
	streamBuf.WriteString("ET\n")

	if inv.BillingAddress != "" {
		yPos -= 14
		streamBuf.WriteString("BT\n")
		streamBuf.WriteString("/F1 8 Tf\n")
		streamBuf.WriteString("0.4 0.4 0.4 rg\n")
		streamBuf.WriteString(fmt.Sprintf("45 %d Td\n", yPos))
		streamBuf.WriteString("(Address: " + escapePDF(inv.BillingAddress) + ") Tj\n")
		streamBuf.WriteString("ET\n")
	}

	// Right column: Invoice Metadata
	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F2 10 Tf\n")
	streamBuf.WriteString("0.1 0.15 0.25 rg\n")
	streamBuf.WriteString("330 700 Td\n")
	streamBuf.WriteString("(INVOICE NUMBER: " + escapePDF(inv.InvoiceNumber) + ") Tj\n")
	streamBuf.WriteString("ET\n")

	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F1 9 Tf\n")
	streamBuf.WriteString("0.3 0.3 0.3 rg\n")
	streamBuf.WriteString("330 686 Td\n")
	streamBuf.WriteString("(Booking Ref: " + escapePDF(inv.BookingReference) + ") Tj\n")
	streamBuf.WriteString("ET\n")

	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F1 9 Tf\n")
	streamBuf.WriteString("0.3 0.3 0.3 rg\n")
	streamBuf.WriteString("330 672 Td\n")
	streamBuf.WriteString("(Issue Date: " + escapePDF(inv.IssueDate) + ") Tj\n")
	streamBuf.WriteString("ET\n")

	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F1 9 Tf\n")
	streamBuf.WriteString("0.3 0.3 0.3 rg\n")
	streamBuf.WriteString("330 658 Td\n")
	streamBuf.WriteString("(Status: " + escapePDF(inv.Status) + " - Payment: " + escapePDF(inv.PaymentStatus) + ") Tj\n")
	streamBuf.WriteString("ET\n")

	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F1 9 Tf\n")
	streamBuf.WriteString("0.3 0.3 0.3 rg\n")
	streamBuf.WriteString("330 644 Td\n")
	streamBuf.WriteString("(Currency: " + escapePDF(inv.Currency) + " [Rate: " + fmt.Sprintf("%.4f", inv.ExchangeRate) + "]) Tj\n")
	streamBuf.WriteString("ET\n")

	// Table Divider line
	tableY := 615
	streamBuf.WriteString("0.8 0.85 0.9 RG\n")
	streamBuf.WriteString(fmt.Sprintf("40 %d 515 1 re\n", tableY+15))
	streamBuf.WriteString("f\n")

	// Table Header Bar (pale blue-grey: 0.94 0.96 0.98)
	streamBuf.WriteString("0.94 0.96 0.98 rg\n")
	streamBuf.WriteString(fmt.Sprintf("40 %d 515 22 re\n", tableY-10))
	streamBuf.WriteString("f\n")

	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F2 9 Tf\n")
	streamBuf.WriteString("0.2 0.25 0.35 rg\n")
	streamBuf.WriteString(fmt.Sprintf("48 %d Td\n", tableY-4))
	streamBuf.WriteString("(DESCRIPTION) Tj\n")
	streamBuf.WriteString("220 0 Td\n")
	streamBuf.WriteString("(SEAT) Tj\n")
	streamBuf.WriteString("70 0 Td\n")
	streamBuf.WriteString("(QTY) Tj\n")
	streamBuf.WriteString("60 0 Td\n")
	streamBuf.WriteString("(UNIT PRICE) Tj\n")
	streamBuf.WriteString("70 0 Td\n")
	streamBuf.WriteString("(TOTAL) Tj\n")
	streamBuf.WriteString("ET\n")

	// Line Items
	itemY := tableY - 26
	for idx, itm := range inv.Items {
		if idx%2 == 1 {
			streamBuf.WriteString("0.98 0.98 0.99 rg\n")
			streamBuf.WriteString(fmt.Sprintf("40 %d 515 18 re\n", itemY-5))
			streamBuf.WriteString("f\n")
		}

		streamBuf.WriteString("BT\n")
		streamBuf.WriteString("/F1 9 Tf\n")
		streamBuf.WriteString("0.15 0.15 0.15 rg\n")
		streamBuf.WriteString(fmt.Sprintf("48 %d Td\n", itemY))
		streamBuf.WriteString("(" + escapePDF(itm.Description) + ") Tj\n")
		streamBuf.WriteString(fmt.Sprintf("268 %d Td\n", itemY))
		streamBuf.WriteString("(" + escapePDF(itm.SeatID) + ") Tj\n")
		streamBuf.WriteString(fmt.Sprintf("340 %d Td\n", itemY))
		streamBuf.WriteString(fmt.Sprintf("(%d) Tj\n", itm.Quantity))
		streamBuf.WriteString(fmt.Sprintf("400 %d Td\n", itemY))
		streamBuf.WriteString("(" + escapePDF(inv.CurrencySymbol) + fmt.Sprintf("%.2f", itm.UnitPrice) + ") Tj\n")
		streamBuf.WriteString(fmt.Sprintf("470 %d Td\n", itemY))
		streamBuf.WriteString("(" + escapePDF(inv.CurrencySymbol) + fmt.Sprintf("%.2f", itm.TotalPrice) + ") Tj\n")
		streamBuf.WriteString("ET\n")

		itemY -= 20
	}

	// Financial Summary Box (Right aligned)
	summaryY := itemY - 15
	streamBuf.WriteString("0.85 0.88 0.92 RG\n")
	streamBuf.WriteString(fmt.Sprintf("310 %d 245 1 re\n", summaryY+10))
	streamBuf.WriteString("f\n")

	// Subtotal
	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F1 9 Tf\n")
	streamBuf.WriteString("0.3 0.3 0.3 rg\n")
	streamBuf.WriteString(fmt.Sprintf("320 %d Td\n", summaryY-4))
	streamBuf.WriteString("(Subtotal:) Tj\n")
	streamBuf.WriteString(fmt.Sprintf("470 %d Td\n", summaryY-4))
	streamBuf.WriteString("(" + escapePDF(inv.CurrencySymbol) + fmt.Sprintf("%.2f", inv.Subtotal) + ") Tj\n")
	streamBuf.WriteString("ET\n")

	// Voucher Discount (if any)
	if inv.Discount > 0 {
		summaryY -= 16
		streamBuf.WriteString("BT\n")
		streamBuf.WriteString("/F1 9 Tf\n")
		streamBuf.WriteString("0.1 0.6 0.2 rg\n")
		streamBuf.WriteString(fmt.Sprintf("320 %d Td\n", summaryY-4))
		streamBuf.WriteString("(Voucher Discount:) Tj\n")
		streamBuf.WriteString(fmt.Sprintf("470 %d Td\n", summaryY-4))
		streamBuf.WriteString("(-" + escapePDF(inv.CurrencySymbol) + fmt.Sprintf("%.2f", inv.Discount) + ") Tj\n")
		streamBuf.WriteString("ET\n")
	}

	// Tax / VAT
	summaryY -= 16
	taxLabel := fmt.Sprintf("Tax / VAT (%.1f%%):", inv.TaxRate*100)
	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F1 9 Tf\n")
	streamBuf.WriteString("0.3 0.3 0.3 rg\n")
	streamBuf.WriteString(fmt.Sprintf("320 %d Td\n", summaryY-4))
	streamBuf.WriteString("(" + escapePDF(taxLabel) + ") Tj\n")
	streamBuf.WriteString(fmt.Sprintf("470 %d Td\n", summaryY-4))
	streamBuf.WriteString("(" + escapePDF(inv.CurrencySymbol) + fmt.Sprintf("%.2f", inv.TaxAmount) + ") Tj\n")
	streamBuf.WriteString("ET\n")

	// Total Final Bar
	summaryY -= 24
	streamBuf.WriteString("0.08 0.11 0.18 rg\n")
	streamBuf.WriteString(fmt.Sprintf("310 %d 245 24 re\n", summaryY-6))
	streamBuf.WriteString("f\n")

	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F2 11 Tf\n")
	streamBuf.WriteString("1.0 1.0 1.0 rg\n")
	streamBuf.WriteString(fmt.Sprintf("320 %d Td\n", summaryY))
	streamBuf.WriteString("(TOTAL DUE / PAID:) Tj\n")
	streamBuf.WriteString(fmt.Sprintf("460 %d Td\n", summaryY))
	streamBuf.WriteString("(" + escapePDF(inv.CurrencySymbol) + fmt.Sprintf("%.2f", inv.TotalAmount) + ") Tj\n")
	streamBuf.WriteString("ET\n")

	// Footer / Tax Compliance Legal Note
	streamBuf.WriteString("BT\n")
	streamBuf.WriteString("/F1 8 Tf\n")
	streamBuf.WriteString("0.5 0.55 0.6 rg\n")
	streamBuf.WriteString("45 90 Td\n")
	streamBuf.WriteString("(This is an electronically generated and certified digital tax receipt issued by TicketVerse Global Inc.) Tj\n")
	streamBuf.WriteString("45 78 Td\n")
	streamBuf.WriteString("(Valid for business expense claims, sales tax reporting, and official gate admission with QR verification.) Tj\n")
	streamBuf.WriteString("45 66 Td\n")
	streamBuf.WriteString("(Customer Support: support@ticketverse.io | Web: https://ticketverse.io) Tj\n")
	streamBuf.WriteString("ET\n")

	streamBuf.WriteString("Q\n")

	contentBytes := streamBuf.Bytes()

	// Assemble valid PDF 1.4 document with precise xref offsets
	var doc bytes.Buffer
	doc.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")

	var offsets []int

	// Object 1: Catalog
	offsets = append(offsets, doc.Len())
	doc.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages
	offsets = append(offsets, doc.Len())
	doc.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3: Page
	offsets = append(offsets, doc.Len())
	doc.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595.28 841.89] /Resources << /Font << /F1 4 0 R /F2 5 0 R >> >> /Contents 6 0 R >>\nendobj\n")

	// Object 4: Font F1 (Helvetica)
	offsets = append(offsets, doc.Len())
	doc.WriteString("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	// Object 5: Font F2 (Helvetica-Bold)
	offsets = append(offsets, doc.Len())
	doc.WriteString("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>\nendobj\n")

	// Object 6: Contents Stream
	offsets = append(offsets, doc.Len())
	doc.WriteString(fmt.Sprintf("6 0 obj\n<< /Length %d >>\nstream\n", len(contentBytes)))
	doc.Write(contentBytes)
	doc.WriteString("\nendstream\nendobj\n")

	// Cross-reference table
	startxref := doc.Len()
	doc.WriteString(fmt.Sprintf("xref\n0 %d\n", len(offsets)+1))
	doc.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		doc.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}

	// Trailer
	doc.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, startxref))

	return doc.Bytes(), nil
}

// handleBookingInvoice handles requests for structured JSON invoice or binary PDF download.
func (s *Server) handleBookingInvoice(w http.ResponseWriter, r *http.Request, bookingID string, isPDF bool) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var b Booking
	var seatsRaw string
	var tierID, resID, idemKey, compName, taxID, billAddr, curr sql.NullString
	var exRate, tRate, tAmt sql.NullFloat64

	// Query booking with corporate billing and currency fields
	err := s.db.QueryRow(
		`SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, idempotency_key, created_at, updated_at,
		        currency, exchange_rate, tax_rate, tax_amount, company_name, tax_id, billing_address
		 FROM bookings WHERE id = $1 OR booking_reference = $1`,
		bookingID,
	).Scan(
		&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
		&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
		&b.Status, &resID, &b.PaymentStatus, &idemKey, &b.CreatedAt, &b.UpdatedAt,
		&curr, &exRate, &tRate, &tAmt, &compName, &taxID, &billAddr,
	)

	if err != nil {
		// Fallback query if columns not present yet
		err = s.db.QueryRow(
			`SELECT id, booking_reference, user_id, customer_name, customer_email, event_id, event_title, tier_id, quantity, seat_ids, total_amount, status, reservation_id, payment_status, idempotency_key, created_at, updated_at 
			 FROM bookings WHERE id = $1 OR booking_reference = $1`,
			bookingID,
		).Scan(
			&b.ID, &b.BookingReference, &b.UserID, &b.CustomerName, &b.CustomerEmail,
			&b.EventID, &b.EventTitle, &tierID, &b.Quantity, &seatsRaw, &b.TotalAmount,
			&b.Status, &resID, &b.PaymentStatus, &idemKey, &b.CreatedAt, &b.UpdatedAt,
		)
	}

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Booking not found"})
		return
	}

	_ = json.Unmarshal([]byte(seatsRaw), &b.SeatIDs)
	tickets := s.fetchTicketsForBooking(b.ID)

	bookingCurr := "USD"
	if curr.Valid && curr.String != "" {
		bookingCurr = curr.String
	}
	rate := 1.0
	if exRate.Valid && exRate.Float64 > 0 {
		rate = exRate.Float64
	}
	taxRate := 0.085
	if tRate.Valid && tRate.Float64 > 0 {
		taxRate = tRate.Float64
	}
	taxAmount := b.TotalAmount * taxRate
	if tAmt.Valid {
		taxAmount = tAmt.Float64
	}

	company := ""
	if compName.Valid {
		company = compName.String
	}
	taxNum := ""
	if taxID.Valid {
		taxNum = taxID.String
	}
	address := ""
	if billAddr.Valid {
		address = billAddr.String
	}

	inv := BuildInvoice(&b, tickets, bookingCurr, rate, taxRate, taxAmount, company, taxNum, address)

	if isPDF || r.URL.Query().Get("format") == "pdf" {
		pdfBytes, pdfErr := GenerateInvoicePDF(inv)
		if pdfErr != nil {
			http.Error(w, fmt.Sprintf("Failed to generate PDF: %v", pdfErr), http.StatusInternalServerError)
			return
		}

		filename := fmt.Sprintf("invoice-%s.pdf", inv.BookingReference)
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pdfBytes)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(inv)
}
