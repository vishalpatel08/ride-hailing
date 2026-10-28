package pricing

import (
	"math"

	"ridehailing/internal/model"
	"ridehailing/internal/repository"
)

type SurgeStrategy interface {
	Multiplier(pickup model.Location) float64
}

type SurgeTier struct {
	MaxRatio   float64
	Multiplier float64
}

type DemandSupplyDecorator struct {
	Base             PricingStrategy
	Rides            repository.RideRepo
	Drivers          repository.DriverRepo
	RadiusKm         float64
	OpenRideRequests func(pickup model.Location, radiusKm float64) int
	Tiers            []SurgeTier
}

func NewDemandSupplyDecorator(base PricingStrategy, rides repository.RideRepo, drivers repository.DriverRepo, radiusKm float64, openRequests func(model.Location, float64) int, tiers []SurgeTier) *DemandSupplyDecorator {
	return &DemandSupplyDecorator{
		Base: base, Rides: rides, Drivers: drivers, RadiusKm: radiusKm,
		OpenRideRequests: openRequests, Tiers: append([]SurgeTier(nil), tiers...),
	}
}

func DefaultSurgeTiers() []SurgeTier {
	// Keep ratio 1 in the middle tier while MaxRatio remains inclusive.
	return []SurgeTier{{MaxRatio: math.Nextafter(1, 0), Multiplier: 1}, {MaxRatio: 2, Multiplier: 1.25}, {MaxRatio: 0, Multiplier: 1.5}}
}

func (d *DemandSupplyDecorator) Calculate(cabType model.CabType, distanceKm float64) int64 {
	return d.Base.Calculate(cabType, distanceKm)
}

func (d *DemandSupplyDecorator) Multiplier(pickup model.Location) float64 {
	requests := 0
	if d.OpenRideRequests != nil {
		requests = d.OpenRideRequests(pickup, d.RadiusKm)
	}
	for _, ride := range d.Rides.ListAll(model.RideStatusOngoing) {
		if Distance(pickup, ride.Pickup) <= d.RadiusKm {
			requests++
		}
	}

	available := 0
	for _, cabType := range []model.CabType{model.CabTypeHatchback, model.CabTypeSedan} {
		for _, driver := range d.Drivers.ListAvailableByCabType(cabType) {
			if Distance(pickup, driver.Location) <= d.RadiusKm {
				available++
			}
		}
	}
	ratio := float64(requests)
	if available > 0 {
		ratio /= float64(available)
	} else if requests > 0 {
		ratio = math.Inf(1)
	}

	for _, tier := range d.Tiers {
		if tier.MaxRatio <= 0 || ratio <= tier.MaxRatio {
			return tier.Multiplier
		}
	}
	return 1
}
