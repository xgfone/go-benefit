package benefit_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	benefit "github.com/xgfone/go-benefit"
)

const (
	exampleDriverType         benefit.DriverType     = "example.coupon"
	exampleMerchantConstraint benefit.ConstraintType = "example.merchant"
)

type exampleConfig struct {
	Token          string `json:"token"`
	MerchantID     string `json:"merchant_id"`
	MinimumAmount  int64  `json:"minimum_amount"`
	DiscountAmount int64  `json:"discount_amount"`
}

type exampleUseContext struct {
	MerchantID string
	Amount     benefit.Money
}

type exampleDriverDefinition struct{}

func (exampleDriverDefinition) Descriptor() benefit.DriverDescriptor {
	return benefit.DriverDescriptor{
		Name: "Example coupon",
		Type: exampleDriverType,
		Provider: benefit.TypeDescriptor{
			Type: "example",
			Name: "Example provider",
		},
		Kind: benefit.TypeDescriptor{
			Type: "coupon",
			Name: "Coupon",
		},
		Operations: benefit.OperationSupports{
			benefit.OperationSupport{
				Operation: benefit.OperationReverse,
				Supported: true,
			},
		},
	}
}

func (exampleDriverDefinition) ConfigSchema() benefit.ConfigSchema {
	return benefit.ConfigSchema{
		Revision: "v1",
		Schema: json.RawMessage(`{
			"$schema":"https://json-schema.org/draft/2020-12/schema",
			"type":"object",
			"properties":{
				"token":{"type":"string","x-secret":true},
				"merchant_id":{"type":"string"},
				"minimum_amount":{"type":"integer","minimum":0},
				"discount_amount":{"type":"integer","minimum":0}
			},
			"required":["token","merchant_id","minimum_amount","discount_amount"],
			"additionalProperties":false
		}`),
	}
}

func (exampleDriverDefinition) ValidateConfig(ctx context.Context, raw benefit.DriverConfig) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := parseExampleConfig(raw)
	return err
}

func (exampleDriverDefinition) CompileConfig(raw benefit.DriverConfig) (benefit.DriverFactory, error) {
	config, err := parseExampleConfig(raw)
	if err != nil {
		return nil, err
	}

	minimum, err := benefit.NewConstraint(
		benefit.ConstraintMinimumAmount,
		"A minimum purchase amount is required.",
		benefit.AmountConstraintParams{
			Amount:   config.MinimumAmount,
			Currency: "CNY",
		},
	)
	if err != nil {
		return nil, err
	}
	merchant, err := benefit.NewConstraint(
		exampleMerchantConstraint,
		"The coupon is restricted to one merchant.",
		benefit.ScopeConstraintParams{
			Values: []string{config.MerchantID},
			Match:  benefit.ScopeMatchAny,
		},
	)
	if err != nil {
		return nil, err
	}

	constraints := benefit.Constraints{minimum, merchant}
	constraintRegistry := benefit.NewConstraintRegistry()
	if err := constraintRegistry.Register(
		benefit.ConstraintMinimumAmount,
		benefit.NewMinimumAmountConstraintEvaluator(extractExampleAmount),
	); err != nil {
		return nil, err
	}
	if err := constraintRegistry.Register(
		exampleMerchantConstraint,
		benefit.NewScopeConstraintEvaluator(extractExampleMerchant),
	); err != nil {
		return nil, err
	}

	return benefit.DriverFactoryFunc(func() (benefit.Driver, error) {
		return &exampleCouponDriver{
			config:             config,
			constraints:        constraints,
			constraintRegistry: constraintRegistry,
		}, nil
	}), nil
}

func parseExampleConfig(raw benefit.DriverConfig) (exampleConfig, error) {
	var config exampleConfig
	if err := raw.Validate(); err != nil {
		return config, err
	}
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return config, err
	}
	if config.Token == "" || config.MerchantID == "" {
		return config, errors.New("token and merchant_id are required")
	}
	if config.MinimumAmount < 0 || config.DiscountAmount < 0 {
		return config, errors.New("amounts must not be negative")
	}
	return config, nil
}

func extractExampleAmount(input benefit.EvaluationInput) (benefit.Money, bool, error) {
	facts, ok := input.Context.(exampleUseContext)
	if !ok {
		return benefit.Money{}, false, fmt.Errorf("unexpected operation context type %T", input.Context)
	}
	return facts.Amount, true, nil
}

func extractExampleMerchant(input benefit.EvaluationInput) ([]string, error) {
	facts, ok := input.Context.(exampleUseContext)
	if !ok {
		return nil, fmt.Errorf("unexpected operation context type %T", input.Context)
	}
	return []string{facts.MerchantID}, nil
}

type exampleCouponDriver struct {
	config             exampleConfig
	constraints        benefit.Constraints
	constraintRegistry *benefit.ConstraintRegistry
}

func (*exampleCouponDriver) Descriptor() benefit.DriverDescriptor {
	return (exampleDriverDefinition{}).Descriptor()
}

func (d *exampleCouponDriver) Inspect(
	ctx context.Context,
	request benefit.InspectRequest,
) (benefit.BenefitInfo, error) {
	if err := ctx.Err(); err != nil {
		return benefit.BenefitInfo{}, err
	}

	status := benefit.StatusActive
	providerBenefitID := "provider-coupon-1"
	if request.Reference.Value != "COUPON-001" {
		status = benefit.StatusUnknown
		providerBenefitID = ""
	}

	return benefit.BenefitInfo{
		ProviderBenefitID: providerBenefitID,
		DriverType:        exampleDriverType,
		Name:              "CNY 20 coupon",
		Status:            status,
		Usage:             benefit.Usage{RemainingCount: 1},
		Constraints:       append(benefit.Constraints(nil), d.constraints...),
		Operations:        d.Descriptor().Operations,
	}, nil
}

func (d *exampleCouponDriver) Evaluate(
	ctx context.Context,
	request benefit.EvaluateRequest,
) (benefit.EvaluationResult, error) {
	info, err := d.Inspect(ctx, benefit.InspectRequest(request))
	if err != nil {
		return benefit.EvaluationResult{}, err
	}

	result, err := benefit.EvaluateLocalEligibility(ctx, d.constraintRegistry, benefit.EvaluationInput{
		Benefit: info,
		Context: request.Context,
	})
	if err != nil || !result.Eligible {
		return result, err
	}

	facts, ok := request.Context.(exampleUseContext)
	if !ok {
		return benefit.EvaluationResult{}, fmt.Errorf("unexpected operation context type %T", request.Context)
	}
	discountAmount := min(d.config.DiscountAmount, facts.Amount.Amount)
	result.Outcome = benefit.BenefitOutcome{Discount: &benefit.DiscountEffect{
		Type:           benefit.DiscountEffectAmount,
		Currency:       facts.Amount.Currency,
		OriginalAmount: facts.Amount.Amount,
		DiscountAmount: discountAmount,
		PayableAmount:  facts.Amount.Amount - discountAmount,
	}}
	result.EvaluationToken = "quote-COUPON-001"
	return result, nil
}

func (d *exampleCouponDriver) Redeem(
	ctx context.Context,
	request benefit.RedeemRequest,
) (benefit.RedeemResult, error) {
	if err := request.Validate(); err != nil {
		return benefit.RedeemResult{}, err
	}
	if request.Reference.Value != "COUPON-001" {
		return benefit.RedeemResult{
			Status: benefit.ResultStatusFailure,
			Failure: &benefit.RedeemFailure{
				Type: benefit.RedeemFailureBenefitNotFound,
			},
		}, nil
	}

	evaluation, err := d.Evaluate(ctx, benefit.EvaluateRequest{
		Reference: request.Reference,
		Context:   request.Context,
	})
	if err != nil {
		return benefit.RedeemResult{}, err
	}
	if !evaluation.Eligible {
		return benefit.RedeemResult{
			Status: benefit.ResultStatusFailure,
			Failure: &benefit.RedeemFailure{
				Type:       benefit.RedeemFailureConstraintUnsatisfied,
				Violations: evaluation.Constraints.Violations(),
			},
		}, nil
	}

	return benefit.RedeemResult{
		Status: benefit.ResultStatusSuccess,
		Redemption: &benefit.Redemption{
			RedemptionID:         request.RedemptionID,
			ProviderRedemptionID: "provider-" + request.RedemptionID,
			Outcome:              evaluation.Outcome,
			RedeemedAt:           time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC),
		},
	}, nil
}

func (d *exampleCouponDriver) Reverse(
	ctx context.Context,
	request benefit.ReverseRequest,
) (benefit.ReverseResult, error) {
	if err := ctx.Err(); err != nil {
		return benefit.ReverseResult{}, err
	}
	if err := request.Validate(); err != nil {
		return benefit.ReverseResult{}, err
	}
	return benefit.ReverseResult{
		Status: benefit.ResultStatusSuccess,
		Reversal: &benefit.Reversal{
			RedemptionID:         request.RedemptionID,
			ProviderRedemptionID: "provider-" + request.RedemptionID,
			ReversalID:           request.ReversalID,
			ProviderReversalID:   "provider-" + request.ReversalID,
			RestoredAmount: benefit.Money{
				Amount:   d.config.DiscountAmount,
				Currency: "CNY",
			},
			ReversedAt: time.Date(2026, 8, 20, 12, 5, 0, 0, time.UTC),
		},
	}, nil
}

func Example() {
	ctx := context.Background()
	registry := benefit.NewDriverRegistry()
	if err := registry.Register(exampleDriverDefinition{}); err != nil {
		panic(err)
	}

	if _, ok := registry.ConfigSchema(exampleDriverType); !ok {
		panic("config schema is unavailable")
	}
	config := benefit.DriverConfig(`{
		"token":"secret-token",
		"merchant_id":"merchant-1",
		"minimum_amount":10000,
		"discount_amount":2000
	}`)
	if err := registry.ValidateConfig(ctx, exampleDriverType, config); err != nil {
		panic(err)
	}

	driver, err := registry.Bind(exampleDriverType, config)
	if err != nil {
		panic(err)
	}
	reference := benefit.BenefitReference{Value: "COUPON-001"}
	operationContext := exampleUseContext{
		MerchantID: "merchant-1",
		Amount:     benefit.Money{Amount: 10000, Currency: "CNY"},
	}

	info, err := driver.Inspect(ctx, benefit.InspectRequest{
		Reference: reference,
		Context:   operationContext,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("inspect: %s (%s)\n", info.Name, info.Status)

	evaluation, err := driver.Evaluate(ctx, benefit.EvaluateRequest{
		Reference: reference,
		Context:   operationContext,
	})
	if err != nil {
		panic(err)
	}
	payable := benefit.Money{
		Amount:   evaluation.Outcome.Discount.PayableAmount,
		Currency: evaluation.Outcome.Discount.Currency,
	}
	major, err := payable.Major()
	if err != nil {
		panic(err)
	}
	fmt.Printf("evaluate: eligible=%t payable=%s %s\n", evaluation.Eligible, major, payable.Currency)

	redeemResult, err := driver.Redeem(ctx, benefit.RedeemRequest{
		RedemptionID:    "redeem-1",
		EvaluationToken: evaluation.EvaluationToken,
		Reference:       reference,
		Context:         operationContext,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf(
		"redeem: %s provider_id=%s\n",
		redeemResult.Status,
		redeemResult.Redemption.ProviderRedemptionID,
	)

	reverser, ok := driver.(benefit.Reverser)
	if !ok {
		panic("driver does not implement its declared reverse operation")
	}
	reverseResult, err := reverser.Reverse(ctx, benefit.ReverseRequest{
		ReversalID:   "reverse-1",
		RedemptionID: redeemResult.Redemption.RedemptionID,
		Reason:       "order cancelled",
		Context:      operationContext,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf(
		"reverse: %s provider_id=%s\n",
		reverseResult.Status,
		reverseResult.Reversal.ProviderReversalID,
	)

	// Output:
	// inspect: CNY 20 coupon (active)
	// evaluate: eligible=true payable=80.00 CNY
	// redeem: success provider_id=provider-redeem-1
	// reverse: success provider_id=provider-reverse-1
}
