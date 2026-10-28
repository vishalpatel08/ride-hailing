package service

import (
	"testing"

	"ridehailing/internal/errs"
	"ridehailing/internal/model"
	"ridehailing/internal/repository"
)

func TestCouponService(t *testing.T) {
	repo := repository.NewInMemoryCouponRepo()
	service := NewCouponService(repo)

	t.Run("add validates input and duplicates", func(t *testing.T) {
		if err := service.AddCoupon("", model.CouponTypeFlat, 100, 0); err != errs.ErrInvalidInput {
			t.Fatalf("empty code error = %v, want %v", err, errs.ErrInvalidInput)
		}
		if err := service.AddCoupon("ZERO", model.CouponTypeFlat, 0, 0); err != errs.ErrInvalidInput {
			t.Fatalf("non-positive value error = %v, want %v", err, errs.ErrInvalidInput)
		}
		if err := service.AddCoupon("TOO-MUCH", model.CouponTypePercent, 101, 0); err != errs.ErrInvalidInput {
			t.Fatalf("percent over 100 error = %v, want %v", err, errs.ErrInvalidInput)
		}
		if err := service.AddCoupon("UNKNOWN", "BOGO", 1, 0); err != errs.ErrInvalidInput {
			t.Fatalf("unknown type error = %v, want %v", err, errs.ErrInvalidInput)
		}
		if err := service.AddCoupon("FLAT", model.CouponTypeFlat, 2000, 0); err != nil {
			t.Fatalf("AddCoupon() error = %v", err)
		}
		if err := service.AddCoupon("FLAT", model.CouponTypeFlat, 2000, 0); err != errs.ErrAlreadyExists {
			t.Fatalf("duplicate error = %v, want %v", err, errs.ErrAlreadyExists)
		}
	})

	tests := []struct {
		name   string
		coupon model.Coupon
		fare   int64
		want   int64
	}{
		{name: "flat discount is capped at fare", coupon: model.Coupon{Type: model.CouponTypeFlat, Value: 5000}, fare: 3000, want: 3000},
		{name: "percent discount has cap", coupon: model.Coupon{Type: model.CouponTypePercent, Value: 50, MaxDiscountPaise: 1000}, fare: 5000, want: 1000},
		{name: "percent discount without cap", coupon: model.Coupon{Type: model.CouponTypePercent, Value: 25}, fare: 5000, want: 1250},
		{name: "discount never exceeds fare", coupon: model.Coupon{Type: model.CouponTypePercent, Value: 100}, fare: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := service.ComputeDiscount(tt.coupon, tt.fare); got != tt.want {
				t.Errorf("ComputeDiscount() = %d, want %d", got, tt.want)
			}
		})
	}

	t.Run("validate deleted coupon", func(t *testing.T) {
		if err := service.DeleteCoupon("FLAT"); err != nil {
			t.Fatalf("DeleteCoupon() error = %v", err)
		}
		if _, err := service.Validate("FLAT"); err != errs.ErrInvalidCoupon {
			t.Fatalf("Validate() error = %v, want %v", err, errs.ErrInvalidCoupon)
		}
		if err := service.DeleteCoupon("MISSING"); err != errs.ErrNotFound {
			t.Fatalf("DeleteCoupon() missing error = %v, want %v", err, errs.ErrNotFound)
		}
	})

	t.Run("validate inactive coupon", func(t *testing.T) {
		if _, err := repo.Save(model.Coupon{Code: "INACTIVE", Type: model.CouponTypeFlat, Value: 1, Active: false}); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		if _, err := service.Validate("INACTIVE"); err != errs.ErrInvalidCoupon {
			t.Fatalf("Validate() error = %v, want %v", err, errs.ErrInvalidCoupon)
		}
	})
}
