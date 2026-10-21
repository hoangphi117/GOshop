package service

import (
	"context"
	"fmt"
	"goshop/internal/utils"
	"goshop/pkg/shipping"
	"math"
	"strings"
)

type orderShippingValidator struct {
	factory *shipping.ShippingFactory
}

// NewOrderShippingValidator creates an OrderShippingValidatorPort.
func NewOrderShippingValidator(factory *shipping.ShippingFactory) OrderShippingValidatorPort {
	return &orderShippingValidator{factory: factory}
}

func (v *orderShippingValidator) ValidateOrCalculateShippingFee(
	ctx context.Context,
	provider string,
	districtID int,
	wardCode string,
	totalWeight int,
	insuranceValue float64,
	declaredFee float64,
) (float64, error) {
	if v.factory == nil {
		if declaredFee < 0 {
			return 0, utils.NewError(utils.ErrCodeBadRequest, "Shipping fee cannot be negative")
		}
		return declaredFee, nil
	}

	cleanProvider := strings.ToLower(strings.TrimSpace(provider))
	if cleanProvider == "" {
		cleanProvider = "ghn"
	}

	carrier, err := v.factory.GetProvider(cleanProvider)
	if err != nil {
		return 0, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Unsupported shipping provider: %s", provider))
	}

	if totalWeight <= 0 {
		totalWeight = 500 // default 500g
	}

	req := shipping.CreateOrderRequest{
		ToDistrictID:   districtID,
		ToWardCode:     wardCode,
		Weight:         totalWeight,
		Length:         20,
		Width:          15,
		Height:         10,
		InsuranceValue: insuranceValue,
	}

	calculatedFee, err := carrier.CalculateFee(ctx, req)
	if err != nil {
		if declaredFee > 0 {
			return declaredFee, nil
		}
		return 30000.0, nil
	}

	// Tolerance check: allow minor round-off differences (within 1,000 VND)
	if declaredFee > 0 && math.Abs(declaredFee-calculatedFee) <= 1000 {
		return declaredFee, nil
	}

	// Authoritative carrier fee wins (prevents client tamper: e.g. client sent 0)
	return calculatedFee, nil
}
