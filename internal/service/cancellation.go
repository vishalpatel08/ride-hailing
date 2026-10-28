package service

import "time"

type CancellationFeePolicy interface {
	Fee(bookedAt, cancelledAt time.Time) int64
}

type TimeBasedCancellationFeePolicy struct {
	GracePeriod  time.Duration
	FlatFeePaise int64
}

func NewTimeBasedCancellationFeePolicy() TimeBasedCancellationFeePolicy {
	return TimeBasedCancellationFeePolicy{GracePeriod: 2 * time.Minute, FlatFeePaise: 3000}
}

func (p TimeBasedCancellationFeePolicy) Fee(bookedAt, cancelledAt time.Time) int64 {
	if cancelledAt.Sub(bookedAt) <= p.GracePeriod {
		return 0
	}
	return p.FlatFeePaise
}
