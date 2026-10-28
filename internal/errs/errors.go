package errs

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrNoDriverAvailable = errors.New("no driver available")
	ErrInvalidCoupon     = errors.New("invalid coupon")
	ErrUserHasActiveRide = errors.New("user already has an active ride")
	ErrRideNotOngoing    = errors.New("ride is not ongoing")
	ErrInvalidInput      = errors.New("invalid input")
	ErrAlreadyExists     = errors.New("already exists")
)
