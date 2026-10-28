package pricing

import (
	"testing"

	"ridehailing/internal/model"
	"ridehailing/internal/repository"
)

func TestDemandSupplyDecoratorMultiplier(t *testing.T) {
	tests := []struct {
		name         string
		openRequests int
		ongoingRides int
		available    int
		want         float64
	}{
		{name: "ratio below one", openRequests: 1, available: 2, want: 1},
		{name: "ratio one", openRequests: 1, ongoingRides: 1, available: 2, want: 1.25},
		{name: "ratio between one and two", openRequests: 2, ongoingRides: 1, available: 2, want: 1.25},
		{name: "ratio exactly two", openRequests: 3, ongoingRides: 1, available: 2, want: 1.25},
		{name: "ratio above two", openRequests: 4, ongoingRides: 1, available: 2, want: 1.5},
		{name: "no available drivers with demand", openRequests: 1, want: 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rides := repository.NewInMemoryRideRepo()
			drivers := repository.NewInMemoryDriverRepo()
			for i := 0; i < tt.ongoingRides; i++ {
				_, _ = rides.Save(model.Ride{Status: model.RideStatusOngoing, Pickup: model.Location{Lat: 0, Lng: 0}})
			}
			for i := 0; i < tt.available; i++ {
				_, _ = drivers.Save(model.Driver{Status: model.DriverStatusAvailable, CabType: model.CabTypeHatchback, Location: model.Location{Lat: 0, Lng: 0}})
			}
			strategy := NewDemandSupplyDecorator(DefaultTieredPricing(), rides, drivers, 5,
				func(model.Location, float64) int { return tt.openRequests }, DefaultSurgeTiers())
			if got := strategy.Multiplier(model.Location{}); got != tt.want {
				t.Errorf("Multiplier() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}
