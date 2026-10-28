package matching

import (
	"reflect"
	"testing"

	"ridehailing/internal/model"
)

func TestNearestDriverStrategyRank(t *testing.T) {
	pickup := model.Location{Lat: 0, Lng: 0}
	tests := []struct {
		name       string
		candidates []model.Driver
		radiusKm   float64
		wantIDs    []string
	}{
		{
			name: "distance order, ID breaks ties, filter outside radius",
			candidates: []model.Driver{
				{ID: "far", Location: model.Location{Lat: 0, Lng: 1}},
				{ID: "near-b", Location: model.Location{Lat: 0, Lng: 0.01}},
				{ID: "near-a", Location: model.Location{Lat: 0, Lng: -0.01}},
			},
			radiusKm: 2,
			wantIDs:  []string{"near-a", "near-b"},
		},
		{name: "empty input", radiusKm: 5, wantIDs: []string{}},
		{
			name:       "everyone out of range",
			candidates: []model.Driver{{ID: "far", Location: model.Location{Lat: 0, Lng: 1}}},
			radiusKm:   1,
			wantIDs:    []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driverIDs((NearestDriverStrategy{}).Rank(tt.candidates, pickup, tt.radiusKm))
			if !reflect.DeepEqual(got, tt.wantIDs) {
				t.Errorf("Rank() IDs = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func TestHighestRatedStrategyRank(t *testing.T) {
	pickup := model.Location{Lat: 0, Lng: 0}
	tests := []struct {
		name       string
		candidates []model.Driver
		radiusKm   float64
		wantIDs    []string
	}{
		{
			name: "rating then distance then ID, filter outside radius",
			candidates: []model.Driver{
				{ID: "far", Rating: 5, Location: model.Location{Lat: 0, Lng: 1}},
				{ID: "low", Rating: 4.8, Location: model.Location{Lat: 0, Lng: 0.001}},
				{ID: "five-b", Rating: 5, Location: model.Location{Lat: 0, Lng: 0.02}},
				{ID: "five-a", Rating: 5, Location: model.Location{Lat: 0, Lng: -0.01}},
			},
			radiusKm: 3,
			wantIDs:  []string{"five-a", "five-b", "low"},
		},
		{name: "empty input", radiusKm: 5, wantIDs: []string{}},
		{
			name:       "everyone out of range",
			candidates: []model.Driver{{ID: "far", Rating: 5, Location: model.Location{Lat: 0, Lng: 1}}},
			radiusKm:   1,
			wantIDs:    []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driverIDs((HighestRatedStrategy{}).Rank(tt.candidates, pickup, tt.radiusKm))
			if !reflect.DeepEqual(got, tt.wantIDs) {
				t.Errorf("Rank() IDs = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func driverIDs(drivers []model.Driver) []string {
	ids := make([]string, len(drivers))
	for i, driver := range drivers {
		ids[i] = driver.ID
	}
	return ids
}
