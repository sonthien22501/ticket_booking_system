package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CurrencyInfo holds metadata about a supported currency.
type CurrencyInfo struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Symbol   string  `json:"symbol"`
	Rate     float64 `json:"rate"`     // Exchange rate relative to 1.0 USD
	TaxRate  float64 `json:"taxRate"`  // Standard regional sales tax / VAT rate
	Decimals int     `json:"decimals"` // Decimal display places (0 for JPY, 2 for others)
}

var (
	currencyRegistry = map[string]CurrencyInfo{
		"USD": {Code: "USD", Name: "US Dollar", Symbol: "$", Rate: 1.0, TaxRate: 0.085, Decimals: 2},
		"EUR": {Code: "EUR", Name: "Euro", Symbol: "€", Rate: 0.92, TaxRate: 0.20, Decimals: 2},
		"GBP": {Code: "GBP", Name: "British Pound", Symbol: "£", Rate: 0.79, TaxRate: 0.20, Decimals: 2},
		"JPY": {Code: "JPY", Name: "Japanese Yen", Symbol: "¥", Rate: 155.0, TaxRate: 0.10, Decimals: 0},
		"CAD": {Code: "CAD", Name: "Canadian Dollar", Symbol: "CA$", Rate: 1.38, TaxRate: 0.13, Decimals: 2},
		"AUD": {Code: "AUD", Name: "Australian Dollar", Symbol: "A$", Rate: 1.52, TaxRate: 0.10, Decimals: 2},
	}
	currencyMu sync.RWMutex
)

// CurrencyRatesResponse represents the payload returned by GET /api/currencies/rates.
type CurrencyRatesResponse struct {
	Base       string             `json:"base"`
	Rates      map[string]float64 `json:"rates"`
	Symbols    map[string]string  `json:"symbols"`
	TaxRates   map[string]float64 `json:"taxRates"`
	Decimals   map[string]int     `json:"decimals"`
	Currencies []CurrencyInfo     `json:"currencies"`
	UpdatedAt  string             `json:"updatedAt"`
}

// ConvertCurrencyRequest represents payload for POST /api/currencies/convert.
type ConvertCurrencyRequest struct {
	Amount float64 `json:"amount"`
	From   string  `json:"from"`
	To     string  `json:"to"`
}

// ConvertCurrencyResponse represents result of currency conversion.
type ConvertCurrencyResponse struct {
	OriginalAmount  float64 `json:"originalAmount"`
	ConvertedAmount float64 `json:"convertedAmount"`
	From            string  `json:"from"`
	To              string  `json:"to"`
	Rate            float64 `json:"rate"`
	Symbol          string  `json:"symbol"`
	Decimals        int     `json:"decimals"`
}

// ConvertCurrency converts an amount between two supported currencies.
func ConvertCurrency(amount float64, from, to string) (float64, float64, error) {
	currencyMu.RLock()
	defer currencyMu.RUnlock()

	fromCode := strings.ToUpper(strings.TrimSpace(from))
	toCode := strings.ToUpper(strings.TrimSpace(to))

	if fromCode == "" {
		fromCode = "USD"
	}
	if toCode == "" {
		toCode = "USD"
	}

	fromInfo, okFrom := currencyRegistry[fromCode]
	if !okFrom {
		return 0, 0, fmt.Errorf("unsupported source currency: %s", fromCode)
	}

	toInfo, okTo := currencyRegistry[toCode]
	if !okTo {
		return 0, 0, fmt.Errorf("unsupported target currency: %s", toCode)
	}

	if fromCode == toCode {
		return amount, 1.0, nil
	}

	// Calculate cross-rate: (Target Rate / Source Rate)
	rate := toInfo.Rate / fromInfo.Rate
	rawConverted := amount * rate

	// Round according to target currency decimals
	var finalAmount float64
	if toInfo.Decimals == 0 {
		finalAmount = math.Round(rawConverted)
	} else {
		factor := math.Pow10(toInfo.Decimals)
		finalAmount = math.Round(rawConverted*factor) / factor
	}

	return finalAmount, rate, nil
}

// CalculateTax computes sales tax or VAT based on currency and net taxable subtotal.
// Ensures tax is applied strictly AFTER voucher discounts.
func CalculateTax(subtotal, discount float64, currency string) (taxRate float64, taxAmount float64, taxableAmount float64) {
	currencyMu.RLock()
	defer currencyMu.RUnlock()

	code := strings.ToUpper(strings.TrimSpace(currency))
	if code == "" {
		code = "USD"
	}

	info, ok := currencyRegistry[code]
	if !ok {
		info = currencyRegistry["USD"]
	}

	taxRate = info.TaxRate

	taxableAmount = subtotal - discount
	if taxableAmount < 0 {
		taxableAmount = 0
	}

	rawTax := taxableAmount * taxRate

	if info.Decimals == 0 {
		taxAmount = math.Round(rawTax)
		taxableAmount = math.Round(taxableAmount)
	} else {
		factor := math.Pow10(info.Decimals)
		taxAmount = math.Round(rawTax*factor) / factor
		taxableAmount = math.Round(taxableAmount*factor) / factor
	}

	return taxRate, taxAmount, taxableAmount
}

// handleCurrencyRates handles GET /api/currencies/rates.
func (s *Server) handleCurrencyRates(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	currencyMu.RLock()
	defer currencyMu.RUnlock()

	rates := make(map[string]float64)
	symbols := make(map[string]string)
	taxRates := make(map[string]float64)
	decimals := make(map[string]int)
	var list []CurrencyInfo

	for code, info := range currencyRegistry {
		rates[code] = info.Rate
		symbols[code] = info.Symbol
		taxRates[code] = info.TaxRate
		decimals[code] = info.Decimals
		list = append(list, info)
	}

	resp := CurrencyRatesResponse{
		Base:       "USD",
		Rates:      rates,
		Symbols:    symbols,
		TaxRates:   taxRates,
		Decimals:   decimals,
		Currencies: list,
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleCurrencyConvert handles POST /api/currencies/convert.
func (s *Server) handleCurrencyConvert(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req ConvertCurrencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid payload: %v"}`, err), http.StatusBadRequest)
		return
	}

	converted, rate, err := ConvertCurrency(req.Amount, req.From, req.To)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	toCode := strings.ToUpper(strings.TrimSpace(req.To))
	if toCode == "" {
		toCode = "USD"
	}
	info := currencyRegistry[toCode]

	resp := ConvertCurrencyResponse{
		OriginalAmount:  req.Amount,
		ConvertedAmount: converted,
		From:            strings.ToUpper(req.From),
		To:              toCode,
		Rate:            rate,
		Symbol:          info.Symbol,
		Decimals:        info.Decimals,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
