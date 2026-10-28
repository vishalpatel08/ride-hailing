package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ridehailing/internal/handler"
	"ridehailing/internal/matching"
	"ridehailing/internal/pricing"
	"ridehailing/internal/repository"
	"ridehailing/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	users := repository.NewInMemoryUserRepo()
	drivers := repository.NewInMemoryDriverRepo()
	rides := repository.NewInMemoryRideRepo()
	couponRepo := repository.NewInMemoryCouponRepo()
	coupons := service.NewCouponService(couponRepo)
	fareStrategy := pricing.DefaultTieredPricing()
	radiusKm := float64(service.DefaultSearchRadiusKm)
	surge := pricing.NewDemandSupplyDecorator(fareStrategy, rides, drivers, radiusKm, nil, pricing.DefaultSurgeTiers())
	var driverStrategy matching.DriverMatchingStrategy
	switch os.Getenv("DRIVER_MATCHING_STRATEGY") {
	case "", "nearest":
		driverStrategy = matching.NearestDriverStrategy{}
	case "rating":
		driverStrategy = matching.HighestRatedStrategy{}
	default:
		return fmt.Errorf("unsupported DRIVER_MATCHING_STRATEGY %q (want nearest or rating)", os.Getenv("DRIVER_MATCHING_STRATEGY"))
	}
	rideService := service.NewRideService(
		users, drivers, rides, coupons,
		surge, driverStrategy,
		radiusKm, time.Now,
		service.WithSurgeStrategy(surge),
		service.WithCancellationFeePolicy(service.NewTimeBasedCancellationFeePolicy()),
	)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           handler.New(rideService, coupons).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-shutdownContext.Done():
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return err
	}
	err := <-serverErrors
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
