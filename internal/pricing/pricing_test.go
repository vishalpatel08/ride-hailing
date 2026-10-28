package pricing

import (
	"testing"

	"ridehailing/internal/model"
)

func TestTieredPricingCalculate(t *testing.T) {
	pricing := DefaultTieredPricing()
	tests := []struct {
		name     string
		cabType  model.CabType
		distance float64
		want     int64
	}{
		// 0 km: minimum fare applies for both cab types.
		{name: "hatchback 0 km", cabType: model.CabTypeHatchback, distance: 0, want: 5000},
		{name: "sedan 0 km", cabType: model.CabTypeSedan, distance: 0, want: 7000},
		// 1 km: both raw fares are below their minimums (₹10 and ₹14).
		{name: "hatchback 1 km", cabType: model.CabTypeHatchback, distance: 1, want: 5000},
		{name: "sedan 1 km", cabType: model.CabTypeSedan, distance: 1, want: 7000},
		// 2 km: ₹20 and ₹28, respectively; minimum fares still apply.
		{name: "hatchback 2 km", cabType: model.CabTypeHatchback, distance: 2, want: 5000},
		{name: "sedan 2 km", cabType: model.CabTypeSedan, distance: 2, want: 7000},
		// 3.5 km: hatchback ₹20 + ₹12 = ₹32; sedan ₹28 + ₹16.50 = ₹44.50.
		{name: "hatchback 3.5 km", cabType: model.CabTypeHatchback, distance: 3.5, want: 5000},
		{name: "sedan 3.5 km", cabType: model.CabTypeSedan, distance: 3.5, want: 7000},
		// 5 km: hatchback ₹20 + ₹24 = ₹44; sedan ₹28 + ₹33 = ₹61.
		{name: "hatchback 5 km", cabType: model.CabTypeHatchback, distance: 5, want: 5000},
		{name: "sedan 5 km", cabType: model.CabTypeSedan, distance: 5, want: 7000},
		// 8 km: hatchback ₹20 + ₹24 + ₹15 = ₹59; sedan ₹28 + ₹33 + ₹21 = ₹82.
		{name: "hatchback 8 km", cabType: model.CabTypeHatchback, distance: 8, want: 5900},
		{name: "sedan 8 km", cabType: model.CabTypeSedan, distance: 8, want: 8200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pricing.Calculate(tt.cabType, tt.distance); got != tt.want {
				t.Errorf("Calculate(%q, %.1f) = %d paise, want %d", tt.cabType, tt.distance, got, tt.want)
			}
		})
	}
}
