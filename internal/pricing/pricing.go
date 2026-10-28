package pricing

import (
	"math"

	"ridehailing/internal/model"
)

type PricingStrategy interface {
	Calculate(cabType model.CabType, distanceKm float64) int64
}

type Tier struct {
	UpToKm    float64
	RatePaise int64
}

type CabPricingConfig struct {
	Tiers        []Tier
	MinFarePaise int64
}

type TieredPricing struct {
	Configs map[model.CabType]CabPricingConfig
}

func NewTieredPricing(configs map[model.CabType]CabPricingConfig) *TieredPricing {
	copyOfConfigs := make(map[model.CabType]CabPricingConfig, len(configs))
	for cabType, config := range configs {
		config.Tiers = append([]Tier(nil), config.Tiers...)
		copyOfConfigs[cabType] = config
	}
	return &TieredPricing{Configs: copyOfConfigs}
}

// DefaultTieredPricing returns the assignment's default hatchback and sedan rates.
func DefaultTieredPricing() *TieredPricing {
	return NewTieredPricing(map[model.CabType]CabPricingConfig{
		model.CabTypeHatchback: {
			Tiers:        []Tier{{UpToKm: 2, RatePaise: 1000}, {UpToKm: 5, RatePaise: 800}, {UpToKm: math.Inf(1), RatePaise: 500}},
			MinFarePaise: 5000,
		},
		model.CabTypeSedan: {
			Tiers:        []Tier{{UpToKm: 2, RatePaise: 1400}, {UpToKm: 5, RatePaise: 1100}, {UpToKm: math.Inf(1), RatePaise: 700}},
			MinFarePaise: 7000,
		},
	})
}

// Calculate prices using the requested cab type, including when a larger cab is assigned.
func (p *TieredPricing) Calculate(cabType model.CabType, distanceKm float64) int64 {
	config, ok := p.Configs[cabType]
	if !ok || distanceKm <= 0 {
		if ok && config.MinFarePaise > 0 {
			return config.MinFarePaise
		}
		return 0
	}

	remaining := distanceKm
	previousLimit := 0.0
	fare := 0.0
	for _, tier := range config.Tiers {
		width := tier.UpToKm - previousLimit
		if width < 0 {
			return 0
		}
		km := math.Min(remaining, width)
		fare += km * float64(tier.RatePaise)
		remaining -= km
		previousLimit = tier.UpToKm
		if remaining <= 0 {
			break
		}
	}
	if remaining > 0 {
		return 0
	}

	farePaise := int64(math.Round(fare))
	if farePaise < config.MinFarePaise {
		return config.MinFarePaise
	}
	return farePaise
}
