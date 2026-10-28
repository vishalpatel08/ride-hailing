package model

import "time"

type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type CabType string

const (
	CabTypeHatchback CabType = "HATCHBACK"
	CabTypeSedan     CabType = "SEDAN"
)

type DriverStatus string

const (
	DriverStatusAvailable DriverStatus = "AVAILABLE"
	DriverStatusOnRide    DriverStatus = "ON_RIDE"
)

type RideStatus string

const (
	RideStatusOngoing   RideStatus = "ONGOING"
	RideStatusCompleted RideStatus = "COMPLETED"
	RideStatusCancelled RideStatus = "CANCELLED"
)

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type Driver struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`
	Phone    string       `json:"phone"`
	Rating   float64      `json:"rating"`
	Status   DriverStatus `json:"status"`
	CabType  CabType      `json:"cabType"`
	Location Location     `json:"location"`
}

type Ride struct {
	ID                   string     `json:"id"`
	UserID               string     `json:"userId"`
	DriverID             string     `json:"driverId"`
	RequestedCabType     CabType    `json:"requestedCabType"`
	AssignedCabType      CabType    `json:"assignedCabType"`
	Pickup               Location   `json:"pickup"`
	Drop                 Location   `json:"drop"`
	Status               RideStatus `json:"status"`
	StartTime            time.Time  `json:"startTime"`
	EndTime              time.Time  `json:"endTime"`
	DistanceKm           float64    `json:"distanceKm"`
	SurgeMultiplier      float64    `json:"surgeMultiplier"`
	BaseFarePaise        int64      `json:"baseFarePaise"`
	DiscountPaise        int64      `json:"discountPaise"`
	FinalFarePaise       int64      `json:"finalFarePaise"`
	CouponCode           string     `json:"couponCode"`
	CancellationFeePaise int64      `json:"cancellationFeePaise"`
}

type CouponType string

const (
	CouponTypeFlat    CouponType = "FLAT"
	CouponTypePercent CouponType = "PERCENT"
)

type Coupon struct {
	Code             string     `json:"code"`
	Type             CouponType `json:"type"`
	Value            int64      `json:"value"`
	MaxDiscountPaise int64      `json:"maxDiscount"`
	Active           bool       `json:"active"`
}
