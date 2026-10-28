package service

import (
	"ridehailing/internal/errs"
	"ridehailing/internal/model"
	"ridehailing/internal/repository"
)

type CouponService struct {
	coupons repository.CouponRepo
}

func NewCouponService(coupons repository.CouponRepo) *CouponService {
	return &CouponService{coupons: coupons}
}

func (s *CouponService) AddCoupon(code string, couponType model.CouponType, value, maxDiscountPaise int64) error {
	if code == "" || value <= 0 || (couponType != model.CouponTypeFlat && couponType != model.CouponTypePercent) ||
		(couponType == model.CouponTypePercent && value > 100) {
		return errs.ErrInvalidInput
	}
	_, err := s.coupons.Save(model.Coupon{
		Code:             code,
		Type:             couponType,
		Value:            value,
		MaxDiscountPaise: maxDiscountPaise,
		Active:           true,
	})
	return err
}

func (s *CouponService) DeleteCoupon(code string) error {
	return s.coupons.Delete(code)
}

func (s *CouponService) Validate(code string) (model.Coupon, error) {
	coupon, err := s.coupons.Get(code)
	if err != nil || !coupon.Active {
		return model.Coupon{}, errs.ErrInvalidCoupon
	}
	return coupon, nil
}

func (s *CouponService) ComputeDiscount(coupon model.Coupon, farePaise int64) int64 {
	if farePaise <= 0 {
		return 0
	}

	var discount int64
	switch coupon.Type {
	case model.CouponTypeFlat:
		discount = coupon.Value
	case model.CouponTypePercent:
		// Splitting the division avoids overflowing farePaise*Value for valid percentages.
		discount = farePaise/100*coupon.Value + farePaise%100*coupon.Value/100
		if coupon.MaxDiscountPaise > 0 && discount > coupon.MaxDiscountPaise {
			discount = coupon.MaxDiscountPaise
		}
	default:
		return 0
	}
	if discount > farePaise {
		return farePaise
	}
	return discount
}
