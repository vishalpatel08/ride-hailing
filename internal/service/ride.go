package service

import (
	"math"
	"math/big"
	"time"

	"ridehailing/internal/errs"
	"ridehailing/internal/matching"
	"ridehailing/internal/model"
	"ridehailing/internal/pricing"
	"ridehailing/internal/repository"
)

const DefaultSearchRadiusKm = 5

type RideService struct {
	users     repository.UserRepo
	drivers   repository.DriverRepo
	rides     repository.RideRepo
	coupons   *CouponService
	pricing   pricing.PricingStrategy
	matching  matching.DriverMatchingStrategy
	radiusKm  float64
	clock     func() time.Time
	surge     pricing.SurgeStrategy
	cancelFee CancellationFeePolicy
}

type RideOption func(*RideService)

func WithSurgeStrategy(strategy pricing.SurgeStrategy) RideOption {
	return func(service *RideService) { service.surge = strategy }
}

func WithCancellationFeePolicy(policy CancellationFeePolicy) RideOption {
	return func(service *RideService) { service.cancelFee = policy }
}

func NewRideService(
	users repository.UserRepo,
	drivers repository.DriverRepo,
	rides repository.RideRepo,
	coupons *CouponService,
	fareStrategy pricing.PricingStrategy,
	driverStrategy matching.DriverMatchingStrategy,
	radiusKm float64,
	clock func() time.Time,
	options ...RideOption,
) *RideService {
	if radiusKm <= 0 {
		radiusKm = DefaultSearchRadiusKm
	}
	if clock == nil {
		clock = time.Now
	}
	service := &RideService{
		users: users, drivers: drivers, rides: rides, coupons: coupons,
		pricing: fareStrategy, matching: driverStrategy, radiusKm: radiusKm, clock: clock,
		surge: staticSurge(1), cancelFee: NewTimeBasedCancellationFeePolicy(),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type staticSurge float64

func (s staticSurge) Multiplier(model.Location) float64 { return float64(s) }

func (s *RideService) RegisterUser(user model.User) (model.User, error) {
	return s.users.Save(user)
}

func (s *RideService) RegisterDriver(driver model.Driver) (model.Driver, error) {
	return s.drivers.Save(driver)
}

func (s *RideService) UpdateDriverLocation(driverID string, location model.Location) error {
	return s.drivers.UpdateLocation(driverID, location)
}

func (s *RideService) UserRides(userID string, status ...model.RideStatus) []model.Ride {
	return s.rides.ListByUser(userID, status...)
}

func (s *RideService) DriverRides(driverID string, status ...model.RideStatus) []model.Ride {
	return s.rides.ListByDriver(driverID, status...)
}

func (s *RideService) BookRide(userID string, pickup, drop model.Location, requestedCabType model.CabType, couponCode string) (model.Ride, error) {
	if _, err := s.users.Get(userID); err != nil {
		return model.Ride{}, err
	}
	if s.rides.HasActiveRide(userID) {
		return model.Ride{}, errs.ErrUserHasActiveRide
	}

	if couponCode != "" {
		_, err := s.coupons.Validate(couponCode)
		if err != nil {
			return model.Ride{}, err
		}
	}
	surgeMultiplier := s.surge.Multiplier(pickup)

	cabType := requestedCabType
	candidates := s.drivers.ListAvailableByCabType(requestedCabType)
	ranked := s.matching.Rank(candidates, pickup, s.radiusKm)
	if len(ranked) == 0 && requestedCabType == model.CabTypeHatchback {
		cabType = model.CabTypeSedan
		candidates = s.drivers.ListAvailableByCabType(cabType)
		ranked = s.matching.Rank(candidates, pickup, s.radiusKm)
	}

	var selected model.Driver
	reserved := false
	for _, candidate := range ranked {
		if s.drivers.TryReserve(candidate.ID) {
			selected = candidate
			reserved = true
			break
		}
	}
	if !reserved {
		return model.Ride{}, errs.ErrNoDriverAvailable
	}

	ride := model.Ride{
		UserID: userID, DriverID: selected.ID,
		RequestedCabType: requestedCabType, AssignedCabType: cabType,
		Pickup: pickup, Drop: drop, Status: model.RideStatusOngoing,
		StartTime: s.clock(), CouponCode: couponCode, SurgeMultiplier: surgeMultiplier,
	}
	saved, err := s.rides.SaveIfNoActiveRide(ride)
	if err != nil {
		s.drivers.Release(selected.ID)
		return model.Ride{}, err
	}
	return saved, nil
}

func (s *RideService) EndRide(rideID string, dropLocation model.Location) (model.Ride, error) {
	ride, err := s.rides.Get(rideID)
	if err != nil {
		return model.Ride{}, err
	}
	if ride.Status != model.RideStatusOngoing {
		return model.Ride{}, errs.ErrRideNotOngoing
	}

	ride.Drop = dropLocation
	ride.DistanceKm = pricing.Distance(ride.Pickup, dropLocation)
	ride.BaseFarePaise = applySurge(s.pricing.Calculate(ride.RequestedCabType, ride.DistanceKm), ride.SurgeMultiplier)
	if ride.CouponCode != "" {
		coupon, err := s.coupons.Validate(ride.CouponCode)
		if err != nil {
			return model.Ride{}, err
		}
		ride.DiscountPaise = s.coupons.ComputeDiscount(coupon, ride.BaseFarePaise)
	}
	ride.FinalFarePaise = ride.BaseFarePaise - ride.DiscountPaise
	ride.Status = model.RideStatusCompleted
	ride.EndTime = s.clock()

	saved, err := s.rides.Save(ride)
	if err != nil {
		return model.Ride{}, err
	}
	locationErr := s.drivers.UpdateLocation(ride.DriverID, dropLocation)
	s.drivers.Release(ride.DriverID)
	if locationErr != nil {
		return saved, locationErr
	}
	return saved, nil
}

func applySurge(farePaise int64, multiplier float64) int64 {
	if farePaise <= 0 || math.IsNaN(multiplier) || math.IsInf(multiplier, 0) || multiplier <= 0 {
		return 0
	}
	const scale = int64(10000)
	multiplierUnits := int64(math.Round(multiplier * float64(scale)))
	if multiplierUnits <= 0 {
		return 0
	}
	numerator := new(big.Int).Mul(big.NewInt(farePaise), big.NewInt(multiplierUnits))
	numerator.Add(numerator, big.NewInt(scale/2))
	numerator.Quo(numerator, big.NewInt(scale))
	if !numerator.IsInt64() {
		return math.MaxInt64
	}
	return numerator.Int64()
}

func (s *RideService) CancelRide(rideID string) (model.Ride, error) {
	ride, err := s.rides.Get(rideID)
	if err != nil {
		return model.Ride{}, err
	}
	if ride.Status != model.RideStatusOngoing {
		return model.Ride{}, errs.ErrRideNotOngoing
	}
	now := s.clock()
	ride.CancellationFeePaise = s.cancelFee.Fee(ride.StartTime, now)
	ride.Status = model.RideStatusCancelled
	ride.EndTime = now
	saved, err := s.rides.Save(ride)
	if err != nil {
		return model.Ride{}, err
	}
	s.drivers.Release(ride.DriverID)
	return saved, nil
}
