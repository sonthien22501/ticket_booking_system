package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	DiscountTypePercentage = "PERCENTAGE"
	DiscountTypeFixed      = "FIXED"
)

type Voucher struct {
	ID            string     `json:"id"`
	Code          string     `json:"code"`
	DiscountType  string     `json:"discountType"`  // PERCENTAGE, FIXED
	DiscountValue float64    `json:"discountValue"` // e.g. 20 (for 20%) or 15 (for $15)
	MinSpend      float64    `json:"minSpend"`      // Minimum subtotal required
	MaxUses       int        `json:"maxUses"`       // 0 = unlimited
	CurrentUses   int        `json:"currentUses"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	Active        bool       `json:"active"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type ValidateVoucherRequest struct {
	Code     string  `json:"code"`
	EventID  string  `json:"eventId,omitempty"`
	Subtotal float64 `json:"subtotal"`
}

type ValidateVoucherResponse struct {
	Valid          bool    `json:"valid"`
	Code           string  `json:"code"`
	DiscountType   string  `json:"discountType"`
	DiscountValue  float64 `json:"discountValue"`
	DiscountAmount float64 `json:"discountAmount"`
	FinalTotal     float64 `json:"finalTotal"`
	Message        string  `json:"message"`
	MinSpend       float64 `json:"minSpend"`
}

type CreateVoucherRequest struct {
	Code          string  `json:"code"`
	DiscountType  string  `json:"discountType"`
	DiscountValue float64 `json:"discountValue"`
	MinSpend      float64 `json:"minSpend"`
	MaxUses       int     `json:"maxUses"`
	ExpiresInDays int     `json:"expiresInDays,omitempty"`
}

var (
	voucherMu       sync.RWMutex
	fallbackVoucher = map[string]*Voucher{
		"EARLYBIRD20": {
			ID:            "vch-earlybird20",
			Code:          "EARLYBIRD20",
			DiscountType:  DiscountTypePercentage,
			DiscountValue: 20.0,
			MinSpend:      50.0,
			MaxUses:       200,
			CurrentUses:   14,
			Active:        true,
			CreatedAt:     time.Now().Add(-7 * 24 * time.Hour),
			UpdatedAt:     time.Now(),
		},
		"SAVE10": {
			ID:            "vch-save10",
			Code:          "SAVE10",
			DiscountType:  DiscountTypeFixed,
			DiscountValue: 10.0,
			MinSpend:      30.0,
			MaxUses:       500,
			CurrentUses:   48,
			Active:        true,
			CreatedAt:     time.Now().Add(-14 * 24 * time.Hour),
			UpdatedAt:     time.Now(),
		},
		"VIP50": {
			ID:            "vch-vip50",
			Code:          "VIP50",
			DiscountType:  DiscountTypePercentage,
			DiscountValue: 50.0,
			MinSpend:      100.0,
			MaxUses:       50,
			CurrentUses:   8,
			Active:        true,
			CreatedAt:     time.Now().Add(-3 * 24 * time.Hour),
			UpdatedAt:     time.Now(),
		},
		"WELCOME5": {
			ID:            "vch-welcome5",
			Code:          "WELCOME5",
			DiscountType:  DiscountTypeFixed,
			DiscountValue: 5.0,
			MinSpend:      0.0,
			MaxUses:       1000,
			CurrentUses:   120,
			Active:        true,
			CreatedAt:     time.Now().Add(-30 * 24 * time.Hour),
			UpdatedAt:     time.Now(),
		},
	}
)

func (v *Voucher) Validate() error {
	if !v.Active {
		return fmt.Errorf("voucher '%s' is inactive", v.Code)
	}
	if v.ExpiresAt != nil && time.Now().After(*v.ExpiresAt) {
		return fmt.Errorf("voucher '%s' expired on %s", v.Code, v.ExpiresAt.Format("2006-01-02"))
	}
	if v.MaxUses > 0 && v.CurrentUses >= v.MaxUses {
		return fmt.Errorf("voucher '%s' usage limit exceeded (%d/%d used)", v.Code, v.CurrentUses, v.MaxUses)
	}
	return nil
}

func (v *Voucher) CalculateDiscount(subtotal float64) (float64, float64, error) {
	if err := v.Validate(); err != nil {
		return 0, subtotal, err
	}
	if v.MinSpend > 0 && subtotal < v.MinSpend {
		return 0, subtotal, fmt.Errorf("minimum spend of $%.2f required for voucher '%s'", v.MinSpend, v.Code)
	}

	var discount float64
	switch strings.ToUpper(v.DiscountType) {
	case DiscountTypePercentage:
		discount = (subtotal * v.DiscountValue) / 100.0
	case DiscountTypeFixed:
		discount = v.DiscountValue
	default:
		discount = 0
	}

	// Clamp discount to subtotal so total never becomes negative
	if discount > subtotal {
		discount = subtotal
	}

	discount = math.Round(discount*100) / 100
	finalTotal := math.Max(0, math.Round((subtotal-discount)*100)/100)
	return discount, finalTotal, nil
}

func (s *Server) initVoucherSchema(ctx context.Context) error {
	if s.db == nil {
		return nil
	}

	query := `
	CREATE TABLE IF NOT EXISTS vouchers (
		id VARCHAR(64) PRIMARY KEY,
		code VARCHAR(64) NOT NULL UNIQUE,
		discount_type VARCHAR(32) NOT NULL,
		discount_value NUMERIC(10, 2) NOT NULL,
		min_spend NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
		max_uses INT NOT NULL DEFAULT 0,
		current_uses INT NOT NULL DEFAULT 0,
		expires_at TIMESTAMP WITH TIME ZONE,
		active BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_vouchers_code ON vouchers(code);
	CREATE INDEX IF NOT EXISTS idx_vouchers_active ON vouchers(active);

	ALTER TABLE bookings ADD COLUMN IF NOT EXISTS voucher_code VARCHAR(64);
	ALTER TABLE bookings ADD COLUMN IF NOT EXISTS discount_amount NUMERIC(10, 2) DEFAULT 0.00;
	`
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed creating vouchers table: %w", err)
	}

	// Seed standard vouchers if table is empty
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM vouchers").Scan(&count); err == nil && count == 0 {
		for _, v := range fallbackVoucher {
			_, _ = s.db.ExecContext(ctx,
				`INSERT INTO vouchers (id, code, discount_type, discount_value, min_spend, max_uses, current_uses, active, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				 ON CONFLICT (code) DO NOTHING`,
				v.ID, v.Code, v.DiscountType, v.DiscountValue, v.MinSpend, v.MaxUses, v.CurrentUses, v.Active, v.CreatedAt, v.UpdatedAt,
			)
		}
	}

	return nil
}

func (s *Server) getVoucherByCode(ctx context.Context, code string) (*Voucher, error) {
	normCode := strings.ToUpper(strings.TrimSpace(code))
	if normCode == "" {
		return nil, fmt.Errorf("voucher code is empty")
	}

	if s.db != nil {
		var v Voucher
		var expiresAt sql.NullTime
		err := s.db.QueryRowContext(ctx,
			`SELECT id, code, discount_type, discount_value, min_spend, max_uses, current_uses, expires_at, active, created_at, updated_at 
			 FROM vouchers WHERE UPPER(code) = $1`, normCode).
			Scan(&v.ID, &v.Code, &v.DiscountType, &v.DiscountValue, &v.MinSpend, &v.MaxUses, &v.CurrentUses, &expiresAt, &v.Active, &v.CreatedAt, &v.UpdatedAt)
		if err == nil {
			if expiresAt.Valid {
				v.ExpiresAt = &expiresAt.Time
			}
			return &v, nil
		}
	}

	voucherMu.RLock()
	defer voucherMu.RUnlock()
	if v, exists := fallbackVoucher[normCode]; exists {
		// Return copy
		copied := *v
		return &copied, nil
	}

	return nil, fmt.Errorf("voucher code '%s' not found", normCode)
}

func (s *Server) ValidateVoucher(code string, subtotal float64) (*ValidateVoucherResponse, error) {
	v, err := s.getVoucherByCode(context.Background(), code)
	if err != nil {
		return &ValidateVoucherResponse{
			Valid:   false,
			Code:    strings.ToUpper(strings.TrimSpace(code)),
			Message: err.Error(),
		}, err
	}

	discount, finalTotal, calcErr := v.CalculateDiscount(subtotal)
	if calcErr != nil {
		return &ValidateVoucherResponse{
			Valid:         false,
			Code:          v.Code,
			DiscountType:  v.DiscountType,
			DiscountValue: v.DiscountValue,
			MinSpend:      v.MinSpend,
			Message:       calcErr.Error(),
		}, calcErr
	}

	return &ValidateVoucherResponse{
		Valid:          true,
		Code:           v.Code,
		DiscountType:   v.DiscountType,
		DiscountValue:  v.DiscountValue,
		DiscountAmount: discount,
		FinalTotal:     finalTotal,
		MinSpend:       v.MinSpend,
		Message:        fmt.Sprintf("Voucher '%s' applied successfully! Saved $%.2f", v.Code, discount),
	}, nil
}

func (s *Server) incrementVoucherUsage(ctx context.Context, code string) {
	normCode := strings.ToUpper(strings.TrimSpace(code))
	if normCode == "" {
		return
	}

	if s.db != nil {
		_, _ = s.db.ExecContext(ctx, "UPDATE vouchers SET current_uses = current_uses + 1, updated_at = NOW() WHERE UPPER(code) = $1", normCode)
	}

	voucherMu.Lock()
	defer voucherMu.Unlock()
	if v, exists := fallbackVoucher[normCode]; exists {
		v.CurrentUses++
		v.UpdatedAt = time.Now()
	}
}

func (s *Server) ListVouchers(ctx context.Context) ([]Voucher, error) {
	if s.db != nil {
		rows, err := s.db.QueryContext(ctx,
			`SELECT id, code, discount_type, discount_value, min_spend, max_uses, current_uses, expires_at, active, created_at, updated_at 
			 FROM vouchers ORDER BY created_at DESC`)
		if err == nil {
			defer rows.Close()
			var list []Voucher
			for rows.Next() {
				var v Voucher
				var expiresAt sql.NullTime
				if err := rows.Scan(&v.ID, &v.Code, &v.DiscountType, &v.DiscountValue, &v.MinSpend, &v.MaxUses, &v.CurrentUses, &expiresAt, &v.Active, &v.CreatedAt, &v.UpdatedAt); err == nil {
					if expiresAt.Valid {
						v.ExpiresAt = &expiresAt.Time
					}
					list = append(list, v)
				}
			}
			if len(list) > 0 {
				return list, nil
			}
		}
	}

	voucherMu.RLock()
	defer voucherMu.RUnlock()
	list := make([]Voucher, 0, len(fallbackVoucher))
	for _, v := range fallbackVoucher {
		list = append(list, *v)
	}
	return list, nil
}

func (s *Server) CreateVoucher(ctx context.Context, req CreateVoucherRequest) (*Voucher, error) {
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		return nil, fmt.Errorf("voucher code is required")
	}
	discType := strings.ToUpper(strings.TrimSpace(req.DiscountType))
	if discType != DiscountTypePercentage && discType != DiscountTypeFixed {
		discType = DiscountTypePercentage
	}
	if req.DiscountValue <= 0 {
		return nil, fmt.Errorf("discount value must be greater than zero")
	}

	v := &Voucher{
		ID:            fmt.Sprintf("vch-%d", time.Now().UnixNano()),
		Code:          code,
		DiscountType:  discType,
		DiscountValue: req.DiscountValue,
		MinSpend:      math.Max(0, req.MinSpend),
		MaxUses:       req.MaxUses,
		CurrentUses:   0,
		Active:        true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if req.ExpiresInDays > 0 {
		exp := time.Now().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)
		v.ExpiresAt = &exp
	}

	if s.db != nil {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO vouchers (id, code, discount_type, discount_value, min_spend, max_uses, current_uses, expires_at, active, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			v.ID, v.Code, v.DiscountType, v.DiscountValue, v.MinSpend, v.MaxUses, v.CurrentUses, v.ExpiresAt, v.Active, v.CreatedAt, v.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed saving voucher: %w", err)
		}
	}

	voucherMu.Lock()
	fallbackVoucher[code] = v
	voucherMu.Unlock()

	return v, nil
}

func (s *Server) handleValidateVoucher(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req ValidateVoucherRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request payload"}`, http.StatusBadRequest)
		return
	}

	resp, err := s.ValidateVoucher(req.Code, req.Subtotal)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleAdminVouchers(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.Method {
	case http.MethodGet:
		list, err := s.ListVouchers(r.Context())
		if err != nil {
			log.Printf("Error listing vouchers: %v", err)
			list = make([]Voucher, 0)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)

	case http.MethodPost:
		var req CreateVoucherRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"Invalid request payload: %v"}`, err), http.StatusBadRequest)
			return
		}
		v, err := s.CreateVoucher(r.Context(), req)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%v"}`, err), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(v)

	default:
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
	}
}
