package pricing

import (
	"math"

	"ridehailing/internal/model"
)

const earthRadiusKm = 6371

// Distance returns the great-circle distance between two locations in kilometers.
func Distance(a, b model.Location) float64 {
	lat1 := radians(a.Lat)
	lat2 := radians(b.Lat)
	dLat := radians(b.Lat - a.Lat)
	dLng := radians(b.Lng - a.Lng)

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusKm * math.Asin(math.Sqrt(h))
}

func radians(degrees float64) float64 {
	return degrees * math.Pi / 180
}
