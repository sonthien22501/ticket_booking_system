package main

import (
	"math"
	"testing"
)

func TestSurgeMultiplierScarcity(t *testing.T) {
	tests := []struct {
		name               string
		capacity           int
		available          int
		velocity           float64
		expectedMultiplier float64
		expectedFactor     string
	}{
		{
			name:               "Standard demand (10% sold)",
			capacity:           100,
			available:          90,
			velocity:           1.0,
			expectedMultiplier: 1.0,
			expectedFactor:     "STANDARD",
		},
		{
			name:               "Moderate demand (75% sold)",
			capacity:           100,
			available:          25,
			velocity:           2.0,
			expectedMultiplier: 1.15,
			expectedFactor:     "MODERATE_DEMAND",
		},
		{
			name:               "High demand (88% sold)",
			capacity:           100,
			available:          12,
			velocity:           3.0,
			expectedMultiplier: 1.30,
			expectedFactor:     "HIGH_DEMAND",
		},
		{
			name:               "Critical scarcity (96% sold)",
			capacity:           100,
			available:          4,
			velocity:           2.0,
			expectedMultiplier: 1.50,
			expectedFactor:     "CRITICAL_SCARCITY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := CalculateSurgePricing("evt-test", 100.0, tt.capacity, tt.available, tt.velocity)
			if pricing.SurgeMultiplier != tt.expectedMultiplier {
				t.Errorf("expected multiplier %.2f, got %.2f", tt.expectedMultiplier, pricing.SurgeMultiplier)
			}
			if pricing.SurgeFactor != tt.expectedFactor {
				t.Errorf("expected factor %s, got %s", tt.expectedFactor, pricing.SurgeFactor)
			}
			expectedPrice := 100.0 * tt.expectedMultiplier
			if math.Abs(pricing.CurrentPrice-expectedPrice) > 0.001 {
				t.Errorf("expected current price %.2f, got %.2f", expectedPrice, pricing.CurrentPrice)
			}
		})
	}
}

func TestSurgeMultiplierVelocity(t *testing.T) {
	// 50% sold + High Velocity (> 5/min) -> 1.0 (base) + 0.15 (velocity) = 1.15
	pricing := CalculateSurgePricing("evt-test", 100.0, 100, 50, 8.5)
	if pricing.SurgeMultiplier != 1.15 {
		t.Errorf("expected multiplier with velocity 1.15, got %.2f", pricing.SurgeMultiplier)
	}

	// 96% sold (1.50) + High Velocity (> 5/min, +0.15) -> 1.65
	pricingHigh := CalculateSurgePricing("evt-test", 100.0, 100, 4, 10.0)
	if pricingHigh.SurgeMultiplier != 1.65 {
		t.Errorf("expected combined multiplier 1.65, got %.2f", pricingHigh.SurgeMultiplier)
	}

	// Multiplier cap test: even under extreme conditions, capped at 2.00
	pricingCapped := CalculateSurgePricing("evt-test", 100.0, 100, 1, 50.0)
	if pricingCapped.SurgeMultiplier > 2.00 {
		t.Errorf("expected surge multiplier to be capped at 2.00, got %.2f", pricingCapped.SurgeMultiplier)
	}
}

func TestServiceFaultToleranceFallback(t *testing.T) {
	// Downstream dependency failure simulation (e.g. invalid capacity or negative availability)
	pricing := CalculateSurgePricingFallback("evt-failed-dependency", 120.0)
	if pricing.CurrentPrice != 120.0 {
		t.Errorf("expected fallback price to equal base price 120.0, got %.2f", pricing.CurrentPrice)
	}
	if pricing.SurgeMultiplier != 1.0 {
		t.Errorf("expected fallback multiplier 1.0, got %.2f", pricing.SurgeMultiplier)
	}
	if !pricing.Degraded {
		t.Errorf("expected degraded flag to be true on fallback")
	}
}
