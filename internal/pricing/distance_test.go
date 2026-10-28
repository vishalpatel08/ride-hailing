package pricing

import (
	"math"
	"testing"

	"ridehailing/internal/model"
)

func TestDistance(t *testing.T) {
	tests := []struct {
		name string
		a    model.Location
		b    model.Location
		want float64
	}{
		{name: "New York to London", a: model.Location{Lat: 40.7128, Lng: -74.0060}, b: model.Location{Lat: 51.5074, Lng: -0.1278}, want: 5570},
		{name: "Paris to Berlin", a: model.Location{Lat: 48.8566, Lng: 2.3522}, b: model.Location{Lat: 52.5200, Lng: 13.4050}, want: 878},
		{name: "Mumbai to Delhi", a: model.Location{Lat: 19.0760, Lng: 72.8777}, b: model.Location{Lat: 28.6139, Lng: 77.2090}, want: 1150},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Distance(tt.a, tt.b)
			if math.Abs(got-tt.want)/tt.want > 0.01 {
				t.Errorf("Distance() = %.2f km, want within 1%% of %.0f km", got, tt.want)
			}
		})
	}
}
