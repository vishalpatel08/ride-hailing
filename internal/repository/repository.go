package repository

import (
	"sync"

	"ridehailing/internal/errs"
	"ridehailing/internal/model"
)

type UserRepo interface {
	Save(user model.User) (model.User, error)
	Get(id string) (model.User, error)
}

type DriverRepo interface {
	Save(driver model.Driver) (model.Driver, error)
	Get(id string) (model.Driver, error)
	ListAvailableByCabType(cabType model.CabType) []model.Driver
	UpdateLocation(id string, location model.Location) error
	TryReserve(driverID string) bool
	Release(driverID string)
}

type RideRepo interface {
	Save(ride model.Ride) (model.Ride, error)
	Get(id string) (model.Ride, error)
	ListByUser(userID string, status ...model.RideStatus) []model.Ride
	ListByDriver(driverID string, status ...model.RideStatus) []model.Ride
	ListAll(status ...model.RideStatus) []model.Ride
	HasActiveRide(userID string) bool
	SaveIfNoActiveRide(ride model.Ride) (model.Ride, error)
}

type CouponRepo interface {
	Save(coupon model.Coupon) (model.Coupon, error)
	Get(code string) (model.Coupon, error)
	Delete(code string) error
}

type InMemoryUserRepo struct {
	mu    sync.RWMutex
	users map[string]model.User
}

func NewInMemoryUserRepo() *InMemoryUserRepo {
	return &InMemoryUserRepo{users: make(map[string]model.User)}
}

func (r *InMemoryUserRepo) Save(user model.User) (model.User, error) {
	if user.ID == "" {
		user.ID = newID("user")
	}
	r.mu.Lock()
	r.users[user.ID] = user
	r.mu.Unlock()
	return user, nil
}

func (r *InMemoryUserRepo) Get(id string) (model.User, error) {
	r.mu.RLock()
	user, ok := r.users[id]
	r.mu.RUnlock()
	if !ok {
		return model.User{}, errs.ErrNotFound
	}
	return user, nil
}

type InMemoryDriverRepo struct {
	mu      sync.RWMutex
	drivers map[string]model.Driver
}

func NewInMemoryDriverRepo() *InMemoryDriverRepo {
	return &InMemoryDriverRepo{drivers: make(map[string]model.Driver)}
}

func (r *InMemoryDriverRepo) Save(driver model.Driver) (model.Driver, error) {
	if driver.ID == "" {
		driver.ID = newID("driver")
	}
	if driver.Status == "" {
		driver.Status = model.DriverStatusAvailable
	}
	r.mu.Lock()
	r.drivers[driver.ID] = driver
	r.mu.Unlock()
	return driver, nil
}

func (r *InMemoryDriverRepo) Get(id string) (model.Driver, error) {
	r.mu.RLock()
	driver, ok := r.drivers[id]
	r.mu.RUnlock()
	if !ok {
		return model.Driver{}, errs.ErrNotFound
	}
	return driver, nil
}

func (r *InMemoryDriverRepo) ListAvailableByCabType(cabType model.CabType) []model.Driver {
	r.mu.RLock()
	drivers := make([]model.Driver, 0)
	for _, driver := range r.drivers {
		if driver.Status == model.DriverStatusAvailable && driver.CabType == cabType {
			drivers = append(drivers, driver)
		}
	}
	r.mu.RUnlock()
	return drivers
}

func (r *InMemoryDriverRepo) UpdateLocation(id string, location model.Location) error {
	r.mu.Lock()
	driver, ok := r.drivers[id]
	if ok {
		driver.Location = location
		r.drivers[id] = driver
	}
	r.mu.Unlock()
	if !ok {
		return errs.ErrNotFound
	}
	return nil
}

func (r *InMemoryDriverRepo) TryReserve(driverID string) bool {
	r.mu.Lock()
	driver, ok := r.drivers[driverID]
	if !ok || driver.Status != model.DriverStatusAvailable {
		r.mu.Unlock()
		return false
	}
	driver.Status = model.DriverStatusOnRide
	r.drivers[driverID] = driver
	r.mu.Unlock()
	return true
}

func (r *InMemoryDriverRepo) Release(driverID string) {
	r.mu.Lock()
	if driver, ok := r.drivers[driverID]; ok {
		driver.Status = model.DriverStatusAvailable
		r.drivers[driverID] = driver
	}
	r.mu.Unlock()
}

type InMemoryRideRepo struct {
	mu    sync.RWMutex
	rides map[string]model.Ride
}

func NewInMemoryRideRepo() *InMemoryRideRepo {
	return &InMemoryRideRepo{rides: make(map[string]model.Ride)}
}

func (r *InMemoryRideRepo) Save(ride model.Ride) (model.Ride, error) {
	if ride.ID == "" {
		ride.ID = newID("ride")
	}
	r.mu.Lock()
	r.rides[ride.ID] = ride
	r.mu.Unlock()
	return ride, nil
}

func (r *InMemoryRideRepo) SaveIfNoActiveRide(ride model.Ride) (model.Ride, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.rides {
		if existing.UserID == ride.UserID && existing.Status == model.RideStatusOngoing {
			return model.Ride{}, errs.ErrUserHasActiveRide
		}
	}
	if ride.ID == "" {
		ride.ID = newID("ride")
	}
	r.rides[ride.ID] = ride
	return ride, nil
}

func (r *InMemoryRideRepo) Get(id string) (model.Ride, error) {
	r.mu.RLock()
	ride, ok := r.rides[id]
	r.mu.RUnlock()
	if !ok {
		return model.Ride{}, errs.ErrNotFound
	}
	return ride, nil
}

func (r *InMemoryRideRepo) ListByUser(userID string, status ...model.RideStatus) []model.Ride {
	return r.list(func(ride model.Ride) bool { return ride.UserID == userID }, status)
}

func (r *InMemoryRideRepo) ListByDriver(driverID string, status ...model.RideStatus) []model.Ride {
	return r.list(func(ride model.Ride) bool { return ride.DriverID == driverID }, status)
}

func (r *InMemoryRideRepo) ListAll(status ...model.RideStatus) []model.Ride {
	return r.list(func(model.Ride) bool { return true }, status)
}

func (r *InMemoryRideRepo) HasActiveRide(userID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, ride := range r.rides {
		if ride.UserID == userID && ride.Status == model.RideStatusOngoing {
			return true
		}
	}
	return false
}

func (r *InMemoryRideRepo) list(matches func(model.Ride) bool, statuses []model.RideStatus) []model.Ride {
	r.mu.RLock()
	rides := make([]model.Ride, 0)
	for _, ride := range r.rides {
		if matches(ride) && (len(statuses) == 0 || hasStatus(ride.Status, statuses)) {
			rides = append(rides, ride)
		}
	}
	r.mu.RUnlock()
	return rides
}

func hasStatus(status model.RideStatus, statuses []model.RideStatus) bool {
	for _, candidate := range statuses {
		if status == candidate {
			return true
		}
	}
	return false
}

type InMemoryCouponRepo struct {
	mu      sync.RWMutex
	coupons map[string]model.Coupon
}

func NewInMemoryCouponRepo() *InMemoryCouponRepo {
	return &InMemoryCouponRepo{coupons: make(map[string]model.Coupon)}
}

func (r *InMemoryCouponRepo) Save(coupon model.Coupon) (model.Coupon, error) {
	r.mu.Lock()
	if _, exists := r.coupons[coupon.Code]; exists {
		r.mu.Unlock()
		return model.Coupon{}, errs.ErrAlreadyExists
	}
	r.coupons[coupon.Code] = coupon
	r.mu.Unlock()
	return coupon, nil
}

func (r *InMemoryCouponRepo) Get(code string) (model.Coupon, error) {
	r.mu.RLock()
	coupon, ok := r.coupons[code]
	r.mu.RUnlock()
	if !ok {
		return model.Coupon{}, errs.ErrNotFound
	}
	return coupon, nil
}

func (r *InMemoryCouponRepo) Delete(code string) error {
	r.mu.Lock()
	if _, exists := r.coupons[code]; !exists {
		r.mu.Unlock()
		return errs.ErrNotFound
	}
	delete(r.coupons, code)
	r.mu.Unlock()
	return nil
}
