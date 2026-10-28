package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"ridehailing/internal/errs"
	"ridehailing/internal/model"
	"ridehailing/internal/service"
)

const maxRequestBodyBytes = 1 << 20

type Handler struct {
	rides   *service.RideService
	coupons *service.CouponService
}

func New(rides *service.RideService, coupons *service.CouponService) *Handler {
	return &Handler{rides: rides, coupons: coupons}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", h.createUser)
	mux.HandleFunc("POST /drivers", h.createDriver)
	mux.HandleFunc("PUT /drivers/{id}/location", h.updateDriverLocation)
	mux.HandleFunc("POST /rides", h.bookRide)
	mux.HandleFunc("POST /rides/{id}/end", h.endRide)
	mux.HandleFunc("POST /rides/{id}/cancel", h.cancelRide)
	mux.HandleFunc("GET /users/{id}/rides", h.userRides)
	mux.HandleFunc("GET /drivers/{id}/rides", h.driverRides)
	mux.HandleFunc("POST /coupons", h.createCoupon)
	mux.HandleFunc("DELETE /coupons/{code}", h.deleteCoupon)
	return mux
}

type createUserRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

type createDriverRequest struct {
	Name     string         `json:"name"`
	Phone    string         `json:"phone"`
	CabType  model.CabType  `json:"cabType"`
	Location model.Location `json:"location"`
}

type updateLocationRequest struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type bookRideRequest struct {
	UserID     string         `json:"userId"`
	Pickup     model.Location `json:"pickup"`
	Drop       model.Location `json:"drop"`
	CabType    model.CabType  `json:"cabType"`
	CouponCode string         `json:"couponCode"`
}

type endRideRequest struct {
	Drop model.Location `json:"drop"`
}

type createCouponRequest struct {
	Code             string           `json:"code"`
	Type             model.CouponType `json:"type"`
	Value            int64            `json:"value"`
	MaxDiscountPaise int64            `json:"maxDiscount"`
}

type rideResponse struct {
	ID                    string           `json:"id"`
	UserID                string           `json:"userId"`
	DriverID              string           `json:"driverId"`
	RequestedCabType      model.CabType    `json:"requestedCabType"`
	AssignedCabType       model.CabType    `json:"assignedCabType"`
	Pickup                model.Location   `json:"pickup"`
	Drop                  model.Location   `json:"drop"`
	Status                model.RideStatus `json:"status"`
	StartTime             string           `json:"startTime"`
	EndTime               string           `json:"endTime,omitempty"`
	DistanceKm            float64          `json:"distanceKm"`
	SurgeMultiplier       float64          `json:"surgeMultiplier"`
	BaseFarePaise         int64            `json:"baseFarePaise"`
	BaseFareRupees        string           `json:"baseFareRupees"`
	DiscountPaise         int64            `json:"discountPaise"`
	DiscountRupees        string           `json:"discountRupees"`
	FinalFarePaise        int64            `json:"finalFarePaise"`
	FinalFareRupees       string           `json:"finalFareRupees"`
	CouponCode            string           `json:"couponCode,omitempty"`
	CancellationFeePaise  int64            `json:"cancellationFeePaise"`
	CancellationFeeRupees string           `json:"cancellationFeeRupees"`
}

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var request createUserRequest
	if err := decodeJSON(w, r, &request); err != nil {
		respond(w, http.StatusOK, nil, errs.ErrInvalidInput)
		return
	}
	user, err := h.rides.RegisterUser(model.User{Name: request.Name, Phone: request.Phone})
	respond(w, http.StatusCreated, user, err)
}

func (h *Handler) createDriver(w http.ResponseWriter, r *http.Request) {
	var request createDriverRequest
	if err := decodeJSON(w, r, &request); err != nil {
		respond(w, http.StatusOK, nil, errs.ErrInvalidInput)
		return
	}
	driver, err := h.rides.RegisterDriver(model.Driver{
		Name: request.Name, Phone: request.Phone, CabType: request.CabType, Location: request.Location,
	})
	respond(w, http.StatusCreated, driver, err)
}

func (h *Handler) updateDriverLocation(w http.ResponseWriter, r *http.Request) {
	var request updateLocationRequest
	if err := decodeJSON(w, r, &request); err != nil {
		respond(w, http.StatusOK, nil, errs.ErrInvalidInput)
		return
	}
	err := h.rides.UpdateDriverLocation(r.PathValue("id"), model.Location{Lat: request.Lat, Lng: request.Lng})
	respond(w, http.StatusOK, map[string]bool{"updated": err == nil}, err)
}

func (h *Handler) bookRide(w http.ResponseWriter, r *http.Request) {
	var request bookRideRequest
	if err := decodeJSON(w, r, &request); err != nil {
		respond(w, http.StatusOK, nil, errs.ErrInvalidInput)
		return
	}
	ride, err := h.rides.BookRide(request.UserID, request.Pickup, request.Drop, request.CabType, request.CouponCode)
	respond(w, http.StatusCreated, toRideResponse(ride), err)
}

func (h *Handler) endRide(w http.ResponseWriter, r *http.Request) {
	var request endRideRequest
	if err := decodeJSON(w, r, &request); err != nil {
		respond(w, http.StatusOK, nil, errs.ErrInvalidInput)
		return
	}
	ride, err := h.rides.EndRide(r.PathValue("id"), request.Drop)
	respond(w, http.StatusOK, toRideResponse(ride), err)
}

func (h *Handler) cancelRide(w http.ResponseWriter, r *http.Request) {
	ride, err := h.rides.CancelRide(r.PathValue("id"))
	respond(w, http.StatusOK, toRideResponse(ride), err)
}

func (h *Handler) userRides(w http.ResponseWriter, r *http.Request) {
	status, err := parseStatus(r.URL.Query().Get("status"))
	if err != nil {
		respond(w, http.StatusOK, nil, err)
		return
	}
	rides := h.rides.UserRides(r.PathValue("id"), status...)
	respond(w, http.StatusOK, toRideResponses(rides), nil)
}

func (h *Handler) driverRides(w http.ResponseWriter, r *http.Request) {
	status, err := parseStatus(r.URL.Query().Get("status"))
	if err != nil {
		respond(w, http.StatusOK, nil, err)
		return
	}
	rides := h.rides.DriverRides(r.PathValue("id"), status...)
	respond(w, http.StatusOK, toRideResponses(rides), nil)
}

func (h *Handler) createCoupon(w http.ResponseWriter, r *http.Request) {
	var request createCouponRequest
	if err := decodeJSON(w, r, &request); err != nil {
		respond(w, http.StatusOK, nil, errs.ErrInvalidInput)
		return
	}
	err := h.coupons.AddCoupon(request.Code, request.Type, request.Value, request.MaxDiscountPaise)
	respond(w, http.StatusCreated, map[string]bool{"created": err == nil}, err)
}

func (h *Handler) deleteCoupon(w http.ResponseWriter, r *http.Request) {
	err := h.coupons.DeleteCoupon(r.PathValue("code"))
	respond(w, http.StatusOK, map[string]bool{"deleted": err == nil}, err)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request must contain one JSON value")
	}
	return nil
}

func respond(w http.ResponseWriter, successStatus int, value any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, errs.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(err, errs.ErrInvalidInput), errors.Is(err, errs.ErrInvalidCoupon):
			status = http.StatusBadRequest
		case errors.Is(err, errs.ErrNoDriverAvailable), errors.Is(err, errs.ErrUserHasActiveRide):
			status = http.StatusConflict
		}
		w.WriteHeader(status)
		if encodeErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); encodeErr != nil {
			log.Printf("write error response: %v", encodeErr)
		}
		return
	}
	w.WriteHeader(successStatus)
	if encodeErr := json.NewEncoder(w).Encode(value); encodeErr != nil {
		log.Printf("write JSON response: %v", encodeErr)
	}
}

func parseStatus(raw string) ([]model.RideStatus, error) {
	if raw == "" {
		return nil, nil
	}
	status := model.RideStatus(raw)
	switch status {
	case model.RideStatusOngoing, model.RideStatusCompleted, model.RideStatusCancelled:
		return []model.RideStatus{status}, nil
	default:
		return nil, errs.ErrInvalidInput
	}
}

func toRideResponse(ride model.Ride) rideResponse {
	response := rideResponse{
		ID: ride.ID, UserID: ride.UserID, DriverID: ride.DriverID,
		RequestedCabType: ride.RequestedCabType, AssignedCabType: ride.AssignedCabType,
		Pickup: ride.Pickup, Drop: ride.Drop, Status: ride.Status,
		StartTime:  ride.StartTime.Format("2006-01-02T15:04:05.999999999Z07:00"),
		DistanceKm: ride.DistanceKm, SurgeMultiplier: ride.SurgeMultiplier, BaseFarePaise: ride.BaseFarePaise,
		BaseFareRupees: formatRupees(ride.BaseFarePaise), DiscountPaise: ride.DiscountPaise,
		DiscountRupees: formatRupees(ride.DiscountPaise), FinalFarePaise: ride.FinalFarePaise,
		FinalFareRupees: formatRupees(ride.FinalFarePaise), CouponCode: ride.CouponCode,
		CancellationFeePaise: ride.CancellationFeePaise, CancellationFeeRupees: formatRupees(ride.CancellationFeePaise),
	}
	if !ride.EndTime.IsZero() {
		response.EndTime = ride.EndTime.Format("2006-01-02T15:04:05.999999999Z07:00")
	}
	return response
}

func toRideResponses(rides []model.Ride) []rideResponse {
	responses := make([]rideResponse, len(rides))
	for i, ride := range rides {
		responses[i] = toRideResponse(ride)
	}
	return responses
}

func formatRupees(paise int64) string {
	return fmt.Sprintf("₹%d.%02d", paise/100, paise%100)
}
