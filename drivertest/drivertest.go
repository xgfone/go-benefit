// Package drivertest provides reusable checks and success-flow tests for driver
// implementations. Tests use application-supplied, isolated provider fixtures;
// they never contact a real provider unless the fixture explicitly does so.
// Provider wire formats, retry guarantees, and persistence are outside its scope.
package drivertest

import (
	"reflect"
	"testing"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/discount"
)

// CheckEvaluation checks the evaluation payload. A non-nil original additionally
// requires a monetary effect for eligible results and checks its amount/currency.
func CheckEvaluation(t testing.TB, result benefit.EvaluationResult, original *benefit.Money) {
	t.Helper()
	if err := result.Validate(); err != nil {
		t.Fatalf("invalid evaluation result: %v", err)
	}
	if result.Eligible {
		checkOutcome(t, result.Outcome, original)
	}
}

// CheckRedeem checks a returned result, including caller-ID propagation. It does
// not repair missing IDs. A non-nil original requires a monetary success outcome.
// The caller must check invocation errors and the expected status separately.
func CheckRedeem(t testing.TB, req benefit.RedeemRequest, result benefit.RedeemResult, original *benefit.Money) {
	t.Helper()
	if err := req.Validate(); err != nil {
		t.Fatalf("invalid redeem test request: %v", err)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("invalid redeem result: %v", err)
	}
	if result.Redemption != nil {
		if result.Redemption.RedemptionId != req.RedemptionId {
			t.Fatal("redeem result did not preserve caller RedemptionId")
		}
		checkOutcome(t, result.Redemption.Outcome, original)
	}
}

// CheckReverse checks the result payload and caller/provider ID correlation.
// It does not require missing provider IDs or equate restored resources with
// reversed discounts or cash refunds. Expected statuses are checked by callers.
func CheckReverse(t testing.TB, req benefit.ReverseRequest, result benefit.ReverseResult) {
	t.Helper()
	if err := req.Validate(); err != nil {
		t.Fatalf("invalid reverse test request: %v", err)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("invalid reverse result: %v", err)
	}
	if result.Reversal != nil {
		r := result.Reversal
		if r.RedemptionId != req.RedemptionId || r.ReversalId != req.ReversalId {
			t.Fatal("reverse result did not preserve caller operation IDs")
		}
		if req.ProviderRedemptionId != "" && r.ProviderRedemptionId != "" &&
			r.ProviderRedemptionId != req.ProviderRedemptionId {
			t.Fatal("reverse result references a different provider redemption")
		}
	}
}

func checkOutcome(t testing.TB, outcome benefit.BenefitOutcome, original *benefit.Money) {
	t.Helper()
	if original != nil {
		if _, err := discount.Resolve(*original, outcome); err != nil {
			t.Fatalf("monetary outcome does not match transaction: %v", err)
		}
	}
}

// Fixture is an isolated, eligible benefit that can be successfully redeemed
// without an evaluation token, and fully reversed if the driver declares that
// capability. Each invocation of the factory passed to Run must create a new
// provider state. Use t.Cleanup to restore any process-global test transport.
//
// Original optionally enables monetary assertions. Snapshot callbacks must
// return detached values suitable for reflect.DeepEqual. InputSnapshot covers
// every mutable request input. ConsumptionSnapshot covers provider consumption
// state. Their absence disables the respective immutability/side-effect assertions;
// arbitrary DriverInput values cannot be generically cloned by this package.
// Input snapshots are checked around all operations; consumption snapshots are
// checked only around Inspect and Evaluate, which must not consume benefits.
// AssertRedeemed and AssertReversed check provider state after confirmed success,
// rather than trusting result payloads. Both are required for fixtures declaring
// Reverse. They must verify the fixture's actual consumption/restoration, using
// authoritative test-provider state or queries; their meaning is resource-specific
// and need not equate restored resources with transaction discounts. For a driver
// without Reverse, AssertRedeemed is optional. No assertion is made about provider
// idempotency or retaining input after return.
type Fixture struct {
	Driver benefit.Driver

	Inspect  benefit.InspectRequest
	Evaluate benefit.EvaluateRequest
	Redeem   benefit.RedeemRequest
	Reverse  benefit.ReverseRequest

	Original *benefit.Money

	InputSnapshot       func() any
	ConsumptionSnapshot func() any

	AssertRedeemed func(*testing.T, benefit.RedeemRequest, benefit.RedeemResult)
	AssertReversed func(*testing.T, benefit.ReverseRequest, benefit.ReverseResult)
}

// Run checks descriptor/capability consistency, read-only Inspect/Evaluate,
// Evaluate followed by Redeem, Redeem without Evaluate, and optional full
// Reverse. It runs sequentially and creates a fresh fixture for each subtest.
// Use the Check functions in additional provider-specific failure/pending/unknown
// cases.
func Run(t *testing.T, newFixture func(*testing.T) Fixture) {
	t.Helper()
	fresh := func(t *testing.T) Fixture {
		t.Helper()
		f := newFixture(t)
		if f.Driver == nil {
			t.Fatal("fixture has no driver")
		}

		descriptor := f.Driver.Descriptor()
		if err := descriptor.Validate(); err != nil {
			t.Fatalf("invalid driver descriptor: %v", err)
		}

		_, declared := descriptor.Operations.Get(benefit.OperationReverse)
		if declared {
			if _, implemented := f.Driver.(benefit.Reverser); !implemented {
				t.Fatal("driver declares Reverse but does not implement Reverser")
			}
			if f.AssertRedeemed == nil || f.AssertReversed == nil {
				t.Fatal("Reverse fixture requires AssertRedeemed and AssertReversed to verify provider state")
			}
		}
		return f
	}

	redeem := func(t *testing.T, f Fixture) benefit.RedeemResult {
		t.Helper()
		checkInput := unchanged(t, f.InputSnapshot, "Redeem mutated request input")
		req := f.Redeem
		req.EvaluationToken = ""
		result, err := f.Driver.Redeem(t.Context(), req)

		checkInput()
		if err != nil {
			t.Fatalf("Redeem failed: %v", err)
		}

		CheckRedeem(t, req, result, f.Original)
		if result.Status != benefit.ResultStatusSuccess {
			t.Fatalf("eligible fixture did not redeem: %s", result.Status)
		}
		if f.AssertRedeemed != nil {
			f.AssertRedeemed(t, req, result)
		}
		return result
	}

	t.Run("inspect_evaluate_redeem", func(t *testing.T) {
		f := fresh(t)
		checkInput := unchanged(t, f.InputSnapshot, "Inspect/Evaluate mutated request input")
		checkConsumption := unchanged(t, f.ConsumptionSnapshot, "Inspect/Evaluate consumed benefit")
		info, err := f.Driver.Inspect(t.Context(), f.Inspect)

		checkInput()
		checkConsumption()
		if err != nil {
			t.Fatalf("Inspect failed: %v", err)
		}
		if err := info.Validate(); err != nil {
			t.Fatalf("invalid Inspect result: %v", err)
		}
		if info.DriverType != f.Driver.Descriptor().Type {
			t.Fatal("Inspect returned a different driver type")
		}

		for range 2 {
			result, err := f.Driver.Evaluate(t.Context(), f.Evaluate)
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}

			CheckEvaluation(t, result, f.Original)
			if !result.Eligible {
				t.Fatal("fixture is not eligible")
			}

			checkInput()
			checkConsumption()
		}

		redeem(t, f)
	})

	t.Run("redeem_without_evaluate", func(t *testing.T) {
		redeem(t, fresh(t))
	})

	t.Run("redeem_reverse", func(t *testing.T) {
		f := fresh(t)
		_, declared := f.Driver.Descriptor().Operations.Get(benefit.OperationReverse)
		if !declared {
			t.Skip("driver does not declare Reverse")
		}

		result := redeem(t, f)
		req := f.Reverse
		// Use the IDs of this fixture's actual consumption, never the coupon code.
		req.RedemptionId = result.Redemption.RedemptionId
		req.ProviderRedemptionId = result.Redemption.ProviderRedemptionId
		req.Mode = benefit.OperationModeReverseFull
		checkInput := unchanged(t, f.InputSnapshot, "Reverse mutated request input")
		reversed, err := f.Driver.(benefit.Reverser).Reverse(t.Context(), req)

		checkInput()
		if err != nil {
			t.Fatalf("Reverse failed: %v", err)
		}

		CheckReverse(t, req, reversed)
		if reversed.Status != benefit.ResultStatusSuccess {
			t.Fatalf("eligible fixture did not reverse: %s", reversed.Status)
		}

		f.AssertReversed(t, req, reversed)
	})
}

func unchanged(t *testing.T, snapshot func() any, message string) func() {
	t.Helper()
	if snapshot == nil {
		return func() {}
	}

	before := snapshot()
	return func() {
		t.Helper()
		if !reflect.DeepEqual(before, snapshot()) {
			t.Fatal(message)
		}
	}
}
