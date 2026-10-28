package service

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"ridehailing/internal/errs"
	"ridehailing/internal/matching"
	"ridehailing/internal/model"
	"ridehailing/internal/pricing"
	"ridehailing/internal/repository"
)

func newTestRideService(radius float64) (*RideService, *repository.InMemoryUserRepo, *repository.InMemoryDriverRepo, *repository.InMemoryRideRepo) {
	users := repository.NewInMemoryUserRepo()
	drivers := repository.NewInMemoryDriverRepo()
	rides := repository.NewInMemoryRideRepo()
	coupons := NewCouponService(repository.NewInMemoryCouponRepo())
	clock := func() time.Time { return time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC) }
	svc := NewRideService(users, drivers, rides, coupons, pricing.DefaultTieredPricing(), matching.NearestDriverStrategy{}, radius, clock)
	return svc, users, drivers, rides
}

func TestBookAndEndRide(t *testing.T) {
	svc, _, _, _ := newTestRideService(5)
	user, err := svc.RegisterUser(model.User{Name: "Rider"})
	if err != nil {
		t.Fatal(err)
	}
	driver, err := svc.RegisterDriver(model.Driver{ID: "driver-1", CabType: model.CabTypeHatchback, Location: model.Location{Lat: 0, Lng: 0}})
	if err != nil {
		t.Fatal(err)
	}
	ride, err := svc.BookRide(user.ID, model.Location{Lat: 0, Lng: 0}, model.Location{}, model.CabTypeHatchback, "")
	if err != nil {
		t.Fatalf("BookRide() error = %v", err)
	}
	if ride.Status != model.RideStatusOngoing || ride.DriverID != driver.ID {
		t.Fatalf("unexpected booked ride: %+v", ride)
	}
	ended, err := svc.EndRide(ride.ID, model.Location{Lat: 0, Lng: 0.1})
	if err != nil {
		t.Fatalf("EndRide() error = %v", err)
	}
	if ended.Status != model.RideStatusCompleted || ended.DistanceKm <= 0 || ended.BaseFarePaise != 7460 || ended.FinalFarePaise != 7460 || ended.EndTime.IsZero() {
		t.Fatalf("unexpected ended ride: %+v", ended)
	}
	updatedDriver, _ := svc.drivers.Get(driver.ID)
	if updatedDriver.Status != model.DriverStatusAvailable || updatedDriver.Location != (model.Location{Lat: 0, Lng: 0.1}) {
		t.Fatalf("driver not released at destination: %+v", updatedDriver)
	}
}

func TestBookRideUpgradesAndPricesRequestedType(t *testing.T) {
	svc, _, _, _ := newTestRideService(5)
	user, _ := svc.RegisterUser(model.User{Name: "Rider"})
	_, _ = svc.RegisterDriver(model.Driver{ID: "sedan", CabType: model.CabTypeSedan, Location: model.Location{Lat: 0, Lng: 0}})
	ride, err := svc.BookRide(user.ID, model.Location{}, model.Location{}, model.CabTypeHatchback, "")
	if err != nil {
		t.Fatal(err)
	}
	if ride.RequestedCabType != model.CabTypeHatchback || ride.AssignedCabType != model.CabTypeSedan {
		t.Fatalf("unexpected cab types: requested %s assigned %s", ride.RequestedCabType, ride.AssignedCabType)
	}
	ended, err := svc.EndRide(ride.ID, model.Location{Lat: 0, Lng: 0.1})
	if err != nil {
		t.Fatal(err)
	}
	if ended.BaseFarePaise != 7460 {
		t.Fatalf("upgraded ride base fare = %d paise, want hatchback rate 7460", ended.BaseFarePaise)
	}
}

func TestBookRideNoDriverInRange(t *testing.T) {
	svc, _, _, _ := newTestRideService(5)
	user, _ := svc.RegisterUser(model.User{Name: "Rider"})
	_, _ = svc.RegisterDriver(model.Driver{ID: "far", CabType: model.CabTypeSedan, Location: model.Location{Lat: 0, Lng: 1}})
	if _, err := svc.BookRide(user.ID, model.Location{}, model.Location{}, model.CabTypeSedan, ""); err != errs.ErrNoDriverAvailable {
		t.Fatalf("BookRide() error = %v, want %v", err, errs.ErrNoDriverAvailable)
	}
}

func TestUserWithActiveRideCannotBookAgain(t *testing.T) {
	svc, _, _, _ := newTestRideService(5)
	user, _ := svc.RegisterUser(model.User{Name: "Rider"})
	_, _ = svc.RegisterDriver(model.Driver{ID: "driver", CabType: model.CabTypeHatchback})
	if _, err := svc.BookRide(user.ID, model.Location{}, model.Location{}, model.CabTypeHatchback, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BookRide(user.ID, model.Location{}, model.Location{}, model.CabTypeHatchback, ""); err != errs.ErrUserHasActiveRide {
		t.Fatalf("second BookRide() error = %v, want %v", err, errs.ErrUserHasActiveRide)
	}
}

func TestConcurrentBookingsOneDriver(t *testing.T) {
	svc, _, _, _ := newTestRideService(5)
	_, _ = svc.RegisterDriver(model.Driver{ID: "only-driver", CabType: model.CabTypeHatchback})
	const bookings = 50
	for i := 0; i < bookings; i++ {
		if _, err := svc.RegisterUser(model.User{ID: fmt.Sprintf("user-%d", i)}); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes, unavailable, unexpected := 0, 0, []error{}
	for i := 0; i < bookings; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := svc.BookRide(fmt.Sprintf("user-%d", index), model.Location{}, model.Location{}, model.CabTypeHatchback, "")
			mu.Lock()
			defer mu.Unlock()
			switch err {
			case nil:
				successes++
			case errs.ErrNoDriverAvailable:
				unavailable++
			default:
				unexpected = append(unexpected, err)
			}
		}(i)
	}
	wg.Wait()
	if successes != 1 || unavailable != bookings-1 || len(unexpected) != 0 {
		t.Fatalf("successes=%d unavailable=%d unexpected=%v", successes, unavailable, unexpected)
	}
}

func TestConcurrentBookingsNeverDoubleBookDriver(t *testing.T) {
	svc, _, _, _ := newTestRideService(5)
	const driverCount, userCount = 10, 100
	for i := 0; i < driverCount; i++ {
		_, _ = svc.RegisterDriver(model.Driver{ID: fmt.Sprintf("driver-%02d", i), CabType: model.CabTypeHatchback})
	}
	for i := 0; i < userCount; i++ {
		_, _ = svc.RegisterUser(model.User{ID: fmt.Sprintf("user-%03d", i)})
	}

	var wg sync.WaitGroup
	for i := 0; i < userCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, _ = svc.BookRide(fmt.Sprintf("user-%03d", index), model.Location{}, model.Location{}, model.CabTypeHatchback, "")
		}(i)
	}
	wg.Wait()

	ongoingPerDriver := make(map[string]int)
	for i := 0; i < driverCount; i++ {
		driverID := fmt.Sprintf("driver-%02d", i)
		for _, ride := range svc.DriverRides(driverID, model.RideStatusOngoing) {
			ongoingPerDriver[ride.DriverID]++
		}
	}
	for driverID, count := range ongoingPerDriver {
		if count > 1 {
			t.Errorf("driver %s has %d ongoing rides", driverID, count)
		}
	}
}

func TestConcurrentBookingsSameUserCreateOnlyOneActiveRide(t *testing.T) {
	svc, _, drivers, _ := newTestRideService(5)
	user, _ := svc.RegisterUser(model.User{Name: "Rider"})
	for i := 0; i < 10; i++ {
		_, _ = svc.RegisterDriver(model.Driver{ID: fmt.Sprintf("driver-%02d", i), CabType: model.CabTypeHatchback})
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes, activeRideErrors := 0, 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.BookRide(user.ID, model.Location{}, model.Location{}, model.CabTypeHatchback, "")
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successes++
			} else if err == errs.ErrUserHasActiveRide {
				activeRideErrors++
			} else {
				t.Errorf("BookRide() unexpected error = %v", err)
			}
		}()
	}
	wg.Wait()
	if successes != 1 || activeRideErrors != 9 {
		t.Fatalf("successes=%d active-ride errors=%d, want 1 and 9", successes, activeRideErrors)
	}
	reserved := 0
	for i := 0; i < 10; i++ {
		driver, _ := drivers.Get(fmt.Sprintf("driver-%02d", i))
		if driver.Status == model.DriverStatusOnRide {
			reserved++
		}
	}
	if reserved != 1 {
		t.Fatalf("reserved drivers = %d, want 1", reserved)
	}
}

func TestCouponDiscountAppliedAtEnd(t *testing.T) {
	users := repository.NewInMemoryUserRepo()
	drivers := repository.NewInMemoryDriverRepo()
	rides := repository.NewInMemoryRideRepo()
	couponRepo := repository.NewInMemoryCouponRepo()
	couponService := NewCouponService(couponRepo)
	if err := couponService.AddCoupon("HALF", model.CouponTypePercent, 50, 1000); err != nil {
		t.Fatal(err)
	}
	svc := NewRideService(users, drivers, rides, couponService, pricing.DefaultTieredPricing(), matching.NearestDriverStrategy{}, 5, nil)
	user, _ := svc.RegisterUser(model.User{Name: "Rider"})
	_, _ = svc.RegisterDriver(model.Driver{ID: "driver", CabType: model.CabTypeHatchback})
	ride, err := svc.BookRide(user.ID, model.Location{}, model.Location{}, model.CabTypeHatchback, "HALF")
	if err != nil {
		t.Fatal(err)
	}
	ended, err := svc.EndRide(ride.ID, model.Location{Lat: 0, Lng: 0.1})
	if err != nil {
		t.Fatal(err)
	}
	if ended.DiscountPaise != 1000 || ended.FinalFarePaise != 6460 {
		t.Fatalf("discount/final fare = %d/%d, want 1000/6460", ended.DiscountPaise, ended.FinalFarePaise)
	}
}

func TestEndRideRejectsCompletedRide(t *testing.T) {
	svc, _, _, rides := newTestRideService(5)
	ride, err := rides.Save(model.Ride{Status: model.RideStatusCompleted})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EndRide(ride.ID, model.Location{}); !errors.Is(err, errs.ErrRideNotOngoing) {
		t.Fatalf("EndRide() error = %v, want %v", err, errs.ErrRideNotOngoing)
	}
}

func TestCancelRideGracePeriodAndFlatFee(t *testing.T) {
	tests := []struct {
		name    string
		elapsed time.Duration
		wantFee int64
	}{
		{name: "inside grace period", elapsed: time.Minute, wantFee: 0},
		{name: "outside grace period", elapsed: 3 * time.Minute, wantFee: 3000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
			users := repository.NewInMemoryUserRepo()
			drivers := repository.NewInMemoryDriverRepo()
			rides := repository.NewInMemoryRideRepo()
			coupons := NewCouponService(repository.NewInMemoryCouponRepo())
			svc := NewRideService(users, drivers, rides, coupons, pricing.DefaultTieredPricing(), matching.NearestDriverStrategy{}, 5, func() time.Time { return now })
			user, _ := svc.RegisterUser(model.User{Name: "Rider"})
			driver, _ := svc.RegisterDriver(model.Driver{ID: "driver", CabType: model.CabTypeHatchback})
			ride, err := svc.BookRide(user.ID, model.Location{}, model.Location{}, model.CabTypeHatchback, "")
			if err != nil {
				t.Fatal(err)
			}
			now = now.Add(tt.elapsed)
			cancelled, err := svc.CancelRide(ride.ID)
			if err != nil {
				t.Fatalf("CancelRide() error = %v", err)
			}
			if cancelled.Status != model.RideStatusCancelled || cancelled.CancellationFeePaise != tt.wantFee {
				t.Fatalf("status/fee = %s/%d, want CANCELLED/%d", cancelled.Status, cancelled.CancellationFeePaise, tt.wantFee)
			}
			updatedDriver, _ := drivers.Get(driver.ID)
			if updatedDriver.Status != model.DriverStatusAvailable {
				t.Fatalf("driver status = %s, want AVAILABLE", updatedDriver.Status)
			}
		})
	}
}

func TestApplySurgeUsesIntegerPaiseRounding(t *testing.T) {
	if got := applySurge(101, 1.25); got != 126 {
		t.Fatalf("applySurge(101, 1.25) = %d, want 126", got)
	}
	if got := applySurge(100, 1.5); got != 150 {
		t.Fatalf("applySurge(100, 1.5) = %d, want 150", got)
	}
}
