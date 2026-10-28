package matching

import (
	"sort"

	"ridehailing/internal/model"
	"ridehailing/internal/pricing"
)

type DriverMatchingStrategy interface {
	// Rank returns candidates within radiusKm, ordered by preference.
	Rank(candidates []model.Driver, pickup model.Location, radiusKm float64) []model.Driver
}

type NearestDriverStrategy struct{}

func (NearestDriverStrategy) Rank(candidates []model.Driver, pickup model.Location, radiusKm float64) []model.Driver {
	drivers := inRadius(candidates, pickup, radiusKm)
	sort.Slice(drivers, func(i, j int) bool {
		distanceI := pricing.Distance(drivers[i].Location, pickup)
		distanceJ := pricing.Distance(drivers[j].Location, pickup)
		if distanceI == distanceJ {
			return drivers[i].ID < drivers[j].ID
		}
		return distanceI < distanceJ
	})
	return drivers
}

type HighestRatedStrategy struct{}

func (HighestRatedStrategy) Rank(candidates []model.Driver, pickup model.Location, radiusKm float64) []model.Driver {
	drivers := inRadius(candidates, pickup, radiusKm)
	sort.Slice(drivers, func(i, j int) bool {
		if drivers[i].Rating != drivers[j].Rating {
			return drivers[i].Rating > drivers[j].Rating
		}
		distanceI := pricing.Distance(drivers[i].Location, pickup)
		distanceJ := pricing.Distance(drivers[j].Location, pickup)
		if distanceI == distanceJ {
			return drivers[i].ID < drivers[j].ID
		}
		return distanceI < distanceJ
	})
	return drivers
}

func inRadius(candidates []model.Driver, pickup model.Location, radiusKm float64) []model.Driver {
	drivers := make([]model.Driver, 0, len(candidates))
	for _, driver := range candidates {
		if pricing.Distance(driver.Location, pickup) <= radiusKm {
			drivers = append(drivers, driver)
		}
	}
	return drivers
}
