package drivertest_test

import (
	"context"
	"encoding/json/jsontext"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/discount"
	"github.com/xgfone/go-benefit/drivertest"
)

type testDriver struct {
	consumed    bool
	evaluations int
	redemptions int
	reversals   int
	noRestore   bool
	noConsume   bool
}

func (*testDriver) Descriptor() benefit.DriverDescriptor {
	return benefit.DriverDescriptor{
		Type: "test.coupon",
		Name: "Test",
		Kind: benefit.TypeDescriptor{Type: "coupon"},

		Provider: benefit.TypeDescriptor{Type: "test"},
		ConfigSchema: benefit.ConfigSchema{
			Revision: "v1",
			Optional: true,
			Schema:   jsontext.Value(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`),
		},

		Operations: benefit.OperationCapabilities{
			{
				Operation: benefit.OperationReverse,
				Modes: []benefit.OperationMode{
					benefit.OperationModeReverseFull,
				},
			},
		},
	}
}
func (d *testDriver) Inspect(context.Context, benefit.InspectRequest) (benefit.BenefitInfo, error) {
	return benefit.BenefitInfo{
		DriverType: "test.coupon",
		Status:     benefit.StatusActive,
	}, nil
}

func (d *testDriver) Evaluate(_ context.Context, req benefit.EvaluateRequest) (benefit.EvaluationResult, error) {
	d.evaluations++
	effect, err := discount.CalculateAmount(*req.Input.(*benefit.Money), benefit.Money{Amount: 20})
	return benefit.EvaluationResult{
		Eligible: true,
		Outcome:  benefit.BenefitOutcome{Discount: &effect},
	}, err
}

func (d *testDriver) Redeem(_ context.Context, req benefit.RedeemRequest) (benefit.RedeemResult, error) {
	if d.consumed {
		panic("fixture was reused")
	}
	if !d.noConsume {
		d.consumed = true
	}

	d.redemptions++
	effect, err := discount.CalculateAmount(*req.Input.(*benefit.Money), benefit.Money{Amount: 20})
	return benefit.RedeemResult{
		Status: benefit.ResultStatusSuccess,
		Redemption: &benefit.Redemption{
			RedemptionId:         req.RedemptionId,
			ProviderRedemptionId: "provider-redeem",
			Outcome:              benefit.BenefitOutcome{Discount: &effect},
		},
	}, err
}

func (d *testDriver) Reverse(_ context.Context, req benefit.ReverseRequest) (benefit.ReverseResult, error) {
	if !d.consumed || req.ProviderRedemptionId != "provider-redeem" {
		panic("reverse did not use original redemption")
	}
	if !d.noRestore {
		d.consumed = false
	}

	d.reversals++
	return benefit.ReverseResult{
		Status: benefit.ResultStatusSuccess,
		Reversal: &benefit.Reversal{
			RedemptionId:         req.RedemptionId,
			ReversalId:           req.ReversalId,
			ProviderRedemptionId: req.ProviderRedemptionId,
			ProviderReversalId:   "provider-reverse",
			Outcome: benefit.ReversalOutcome{
				Restored: benefit.BenefitRestoration{
					Money: &benefit.Money{Amount: 50},
				},
			},
		},
	}, nil
}

func stateFixture(d *testDriver) drivertest.Fixture {
	input := &benefit.Money{Amount: 100, Currency: "CNY"}
	req := benefit.DriverRequest{Reference: benefit.BenefitReference{Value: "coupon"}, Input: input}
	return drivertest.Fixture{
		Driver:   d,
		Inspect:  benefit.InspectRequest{Reference: req.Reference, Input: req.Input},
		Evaluate: benefit.EvaluateRequest{Reference: req.Reference, Input: req.Input},
		Redeem:   benefit.RedeemRequest{Reference: req.Reference, Input: req.Input, RedemptionId: "caller-redeem"},
		Reverse:  benefit.ReverseRequest{Reference: req.Reference, Input: req.Input, ReversalId: "caller-reverse"},

		Original:            input,
		InputSnapshot:       func() any { return *input },
		ConsumptionSnapshot: func() any { return d.consumed },

		AssertRedeemed: func(t *testing.T, _ benefit.RedeemRequest, _ benefit.RedeemResult) {
			t.Helper()
			if !d.consumed || d.redemptions != 1 {
				t.Fatal("Redeem did not actually consume benefit")
			}
		},

		AssertReversed: func(t *testing.T, _ benefit.ReverseRequest, _ benefit.ReverseResult) {
			t.Helper()
			if d.consumed || d.reversals != 1 {
				t.Fatal("Reverse did not actually restore benefit")
			}
		},
	}
}

func TestRun(t *testing.T) {
	var drivers []*testDriver
	drivertest.Run(t, func(t *testing.T) drivertest.Fixture {
		d := new(testDriver)
		drivers = append(drivers, d)
		return stateFixture(d)
	})

	if len(drivers) != 3 || drivers[0].evaluations != 2 ||
		drivers[0].redemptions != 1 || drivers[1].evaluations != 0 ||
		drivers[1].redemptions != 1 || drivers[2].evaluations != 0 ||
		drivers[2].reversals != 1 || drivers[2].consumed {
		t.Fatalf("wrong fixture lifecycle: %+v", drivers)
	}
}

// Run uses *testing.T subtests. Exercise expected contract failures in a child
// test process so a broken fixture cannot mark the parent regression as failed.
func TestRunRejectsUnverifiedMutations(t *testing.T) {
	const variable = "GO_BENEFIT_DRIVERTEST_MUTATION_CASE"
	if mode := os.Getenv(variable); mode != "" {
		drivertest.Run(t, func(t *testing.T) drivertest.Fixture {
			d := &testDriver{
				noRestore: mode == "no_restore",
				noConsume: mode == "no_consume",
			}

			f := stateFixture(d)
			if mode == "missing_redeem_assertion" {
				f.AssertRedeemed = nil
			}
			if mode == "missing_reverse_assertion" {
				f.AssertReversed = nil
			}
			return f
		})
		return
	}

	for mode, message := range map[string]string{
		"no_restore":                "Reverse did not actually restore benefit",
		"no_consume":                "Redeem did not actually consume benefit",
		"missing_redeem_assertion":  "Reverse fixture requires AssertRedeemed and AssertReversed",
		"missing_reverse_assertion": "Reverse fixture requires AssertRedeemed and AssertReversed",
	} {
		t.Run(mode, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRunRejectsUnverifiedMutations$", "-test.v")
			cmd.Env = append(os.Environ(), variable+"="+mode)
			output, err := cmd.CombinedOutput()

			_, ok := err.(*exec.ExitError)
			if !ok || !strings.Contains(string(output), message) {
				t.Fatalf("broken mutation was not detected: %v\n%s", err, output)
			}
		})
	}
}

func TestChecksAcceptEveryMutationStatus(t *testing.T) {
	for _, status := range []benefit.ResultStatus{
		benefit.ResultStatusSuccess,
		benefit.ResultStatusFailure,
		benefit.ResultStatusPending,
		benefit.ResultStatusUnknown,
	} {
		t.Run(string(status), func(t *testing.T) {
			req := benefit.DriverRequest{Reference: benefit.BenefitReference{Value: "coupon"}}
			rd := benefit.RedeemResult{Status: status}
			rv := benefit.ReverseResult{Status: status}

			if status == benefit.ResultStatusSuccess {
				rd.Redemption = &benefit.Redemption{RedemptionId: "caller"}
				rv.Reversal = &benefit.Reversal{RedemptionId: "caller", ReversalId: "reverse"}
			}
			if status == benefit.ResultStatusFailure {
				rd.Failure = &benefit.RedeemFailure{Code: benefit.RedeemFailureProviderRejected}
				rv.Failure = &benefit.ReversalFailure{Code: benefit.ReversalFailureProviderRejected}
			}

			drivertest.CheckRedeem(t, benefit.RedeemRequest{
				Reference: req.Reference,
				Input:     req.Input,

				RedemptionId: "caller",
			}, rd, nil)

			drivertest.CheckReverse(t, benefit.ReverseRequest{
				Reference: req.Reference,
				Input:     req.Input,

				RedemptionId:         "caller",
				ReversalId:           "reverse",
				ProviderRedemptionId: "provider",
			}, rv)
		})
	}
}

// Embed TB to satisfy its private methods; the checks only need these methods.
type failingTB struct{ testing.TB }
type contractFailure struct{}

func (failingTB) Helper()               {}
func (failingTB) Fatal(...any)          { panic(contractFailure{}) }
func (failingTB) Fatalf(string, ...any) { panic(contractFailure{}) }

func TestChecksRejectBrokenDrivers(t *testing.T) {
	req := benefit.DriverRequest{
		Reference: benefit.BenefitReference{Value: "coupon"},
	}
	rdReq := benefit.RedeemRequest{
		Reference: req.Reference,
		Input:     req.Input,

		RedemptionId: "caller",
	}
	rvReq := benefit.ReverseRequest{
		Reference: req.Reference,
		Input:     req.Input,

		RedemptionId:         "caller",
		ReversalId:           "reverse",
		ProviderRedemptionId: "provider",
	}

	for name, check := range map[string]func(){
		"evaluation payload": func() {
			drivertest.CheckEvaluation(failingTB{}, benefit.EvaluationResult{
				Eligible: true,
				Failure:  &benefit.EvaluationFailure{Code: "bad"},
			}, nil)
		},
		"missing monetary effect": func() {
			drivertest.CheckEvaluation(failingTB{}, benefit.EvaluationResult{
				Eligible: true,
			}, &benefit.Money{})
		},
		"wrong monetary context": func() {
			drivertest.CheckEvaluation(failingTB{}, benefit.EvaluationResult{
				Eligible: true,
				Outcome: benefit.BenefitOutcome{
					Discount: &benefit.DiscountEffect{Type: "amount"},
				},
			}, &benefit.Money{Amount: 100})
		},
		"missing caller ID": func() {
			drivertest.CheckRedeem(failingTB{}, rdReq, benefit.RedeemResult{
				Status:     benefit.ResultStatusSuccess,
				Redemption: &benefit.Redemption{},
			}, nil)
		},
		"unknown with confirmed payload": func() {
			drivertest.CheckRedeem(failingTB{}, rdReq, benefit.RedeemResult{
				Status:     benefit.ResultStatusUnknown,
				Redemption: &benefit.Redemption{},
			}, nil)
		},
		"wrong reversal caller": func() {
			drivertest.CheckReverse(failingTB{}, rvReq, benefit.ReverseResult{
				Status:   benefit.ResultStatusSuccess,
				Reversal: &benefit.Reversal{RedemptionId: "wrong"},
			})
		},
		"wrong reversal provider": func() {
			drivertest.CheckReverse(failingTB{}, rvReq, benefit.ReverseResult{
				Status: benefit.ResultStatusSuccess,
				Reversal: &benefit.Reversal{
					RedemptionId:         "caller",
					ReversalId:           "reverse",
					ProviderRedemptionId: "wrong",
				},
			})
		},
		"pending with failure": func() {
			drivertest.CheckReverse(failingTB{}, rvReq, benefit.ReverseResult{
				Status:  benefit.ResultStatusPending,
				Failure: &benefit.ReversalFailure{},
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if _, ok := recover().(contractFailure); !ok {
					t.Error("broken driver was not rejected")
				}
			}()
			check()
		})
	}
}

// Embedding the core Driver interface intentionally hides the concrete Reverser.
type coreOnlyDriver struct{ benefit.Driver }

func (d coreOnlyDriver) Descriptor() benefit.DriverDescriptor {
	descriptor := d.Driver.Descriptor()
	descriptor.Operations = nil
	return descriptor
}

func TestRunWithoutOptionalReverseOrSnapshots(t *testing.T) {
	drivertest.Run(t, func(t *testing.T) drivertest.Fixture {
		input := &benefit.Money{Amount: 100, Currency: "CNY"}
		req := benefit.DriverRequest{
			Reference: benefit.BenefitReference{Value: "coupon"},
			Input:     input,
		}

		return drivertest.Fixture{
			Driver:   coreOnlyDriver{Driver: new(testDriver)},
			Inspect:  benefit.InspectRequest{Reference: req.Reference, Input: req.Input},
			Evaluate: benefit.EvaluateRequest{Reference: req.Reference, Input: req.Input},
			Redeem: benefit.RedeemRequest{
				Reference: req.Reference,
				Input:     req.Input,

				RedemptionId: "caller",
			}}
	})
}
