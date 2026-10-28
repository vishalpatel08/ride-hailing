package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ridehailing/internal/matching"
	"ridehailing/internal/pricing"
	"ridehailing/internal/repository"
	"ridehailing/internal/service"
)

func TestBookEndHistoryFlow(t *testing.T) {
	users := repository.NewInMemoryUserRepo()
	drivers := repository.NewInMemoryDriverRepo()
	rides := repository.NewInMemoryRideRepo()
	coupons := service.NewCouponService(repository.NewInMemoryCouponRepo())
	rideService := service.NewRideService(users, drivers, rides, coupons, pricing.DefaultTieredPricing(), matching.NearestDriverStrategy{}, 5, nil)
	api := New(rideService, coupons).Routes()

	user := performJSONRequest(t, api, http.MethodPost, "/users", `{"name":"Rider","phone":"123"}`, http.StatusCreated)
	var createdUser struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(user, &createdUser); err != nil || createdUser.ID == "" {
		t.Fatalf("user response ID = %q, decode error = %v", createdUser.ID, err)
	}
	performJSONRequest(t, api, http.MethodPost, "/drivers", `{"name":"Driver","phone":"456","cabType":"HATCHBACK","location":{"lat":0,"lng":0}}`, http.StatusCreated)

	booked := performJSONRequest(t, api, http.MethodPost, "/rides", `{"userId":"`+createdUser.ID+`","pickup":{"lat":0,"lng":0},"drop":{"lat":0,"lng":0},"cabType":"HATCHBACK"}`, http.StatusCreated)
	var ride struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(booked, &ride); err != nil || ride.ID == "" || ride.Status != "ONGOING" {
		t.Fatalf("unexpected booked ride response: %s (%v)", booked, err)
	}

	ended := performJSONRequest(t, api, http.MethodPost, "/rides/"+ride.ID+"/end", `{"drop":{"lat":0,"lng":0.1}}`, http.StatusOK)
	var fare struct {
		Status          string `json:"status"`
		BaseFarePaise   int64  `json:"baseFarePaise"`
		BaseFareRupees  string `json:"baseFareRupees"`
		FinalFarePaise  int64  `json:"finalFarePaise"`
		FinalFareRupees string `json:"finalFareRupees"`
	}
	if err := json.Unmarshal(ended, &fare); err != nil {
		t.Fatal(err)
	}
	if fare.Status != "COMPLETED" || fare.BaseFarePaise == 0 || fare.FinalFarePaise != fare.BaseFarePaise || fare.BaseFareRupees != "₹74.60" || fare.FinalFareRupees != "₹74.60" {
		t.Fatalf("unexpected fare response: %s", ended)
	}

	history := performJSONRequest(t, api, http.MethodGet, "/users/"+createdUser.ID+"/rides?status=COMPLETED", "", http.StatusOK)
	var ridesHistory []struct {
		ID             string `json:"id"`
		Status         string `json:"status"`
		FinalFarePaise int64  `json:"finalFarePaise"`
	}
	if err := json.Unmarshal(history, &ridesHistory); err != nil {
		t.Fatal(err)
	}
	if len(ridesHistory) != 1 || ridesHistory[0].ID != ride.ID || ridesHistory[0].Status != "COMPLETED" || ridesHistory[0].FinalFarePaise != fare.FinalFarePaise {
		t.Fatalf("unexpected history response: %s", history)
	}
}

func TestRejectsUnknownJSONFields(t *testing.T) {
	users := repository.NewInMemoryUserRepo()
	drivers := repository.NewInMemoryDriverRepo()
	rides := repository.NewInMemoryRideRepo()
	coupons := service.NewCouponService(repository.NewInMemoryCouponRepo())
	rideService := service.NewRideService(users, drivers, rides, coupons, pricing.DefaultTieredPricing(), matching.NearestDriverStrategy{}, 5, nil)
	api := New(rideService, coupons).Routes()
	performJSONRequest(t, api, http.MethodPost, "/users", `{"name":"Rider","phone":"123","extra":true}`, http.StatusBadRequest)
}

func performJSONRequest(t *testing.T, handler http.Handler, method, path, body string, wantStatus int) []byte {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body: %s", method, path, response.Code, wantStatus, response.Body.String())
	}
	return response.Body.Bytes()
}
