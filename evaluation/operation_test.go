package evaluation_test

import (
	"context"
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xgfone/go-benefit"
	"github.com/xgfone/go-benefit/constraint"
	"github.com/xgfone/go-benefit/evaluation"
)

func TestEvaluateOperationRejectsCoreOperation(t *testing.T) {
	if _, err := evaluation.EvaluateOperation(
		context.Background(),
		constraint.DefaultRegistry,
		nil,
		nil,
		benefit.OperationRedeem,
		"",
		constraint.Input{},
	); !errors.Is(err, benefit.ErrValidation) {
		t.Fatalf("expected validation category for core operation, got %v", err)
	}
}

func TestEvaluateOperationRejectsNilRegistry(t *testing.T) {
	_, err := evaluation.EvaluateOperation(
		context.Background(), nil, nil, nil, benefit.OperationReverse, "", constraint.Input{},
	)
	if !errors.Is(err, constraint.ErrNilRegistry) {
		t.Fatalf("unexpected nil registry error: %v", err)
	}
}

func TestLocalEligibilityUsesSharedConstraintRegistry(t *testing.T) {
	const typ benefit.ConstraintType = "test.default_registry"
	if err := constraint.Register(typ, constraint.EvaluatorFunc(func(
		context.Context, benefit.Constraint, constraint.Input,
	) (benefit.ConstraintDecision, error) {
		return benefit.ConstraintSatisfied(), nil
	})); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { constraint.DefaultRegistry.Unregister(typ) })
	input := constraint.Input{Benefit: benefit.BenefitInfo{
		Status:      benefit.StatusActive,
		Constraints: benefit.Constraints{{Type: typ}},
	}}
	report := constraint.Evaluate(context.Background(), input, input.Benefit.Constraints)
	if !report.IsSatisfied() {
		t.Fatalf("default registry did not evaluate the registered constraint: %#v", report)
	}
	result, err := evaluation.EvaluateLocalEligibility(context.Background(), nil, input)
	if err != nil || !result.Eligible || !result.Constraints.IsSatisfied() {
		t.Fatalf("local eligibility did not use the shared registry: %#v, %v", result, err)
	}
	result, err = evaluation.EvaluateLocalEligibility(context.Background(), constraint.NewRegistry(), input)
	if err != nil || result.Eligible || result.Constraints.Unrecognized != 1 {
		t.Fatalf("explicit registry was not honored: %#v, %v", result, err)
	}
}

func TestEvaluateOperationDecisionStatuses(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	input := constraint.Input{Now: now}

	unsupported, err := evaluation.EvaluateOperation(
		ctx,
		constraint.DefaultRegistry,
		nil,
		nil,
		benefit.OperationReverse,
		benefit.OperationModeReverseFull,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if unsupported.Status != benefit.OperationDecisionStatusUnsupported ||
		unsupported.IsSupported() || unsupported.IsEligible() ||
		unsupported.Constraints.Status != benefit.ConstraintReportStatusUnevaluated ||
		unsupported.Reason == "" {
		t.Fatalf("unexpected unsupported decision: %#v", unsupported)
	}
	if err := unsupported.Validate(); err != nil {
		t.Fatalf("unsupported decision failed validation: %v", err)
	}

	future, err := benefit.NewConstraint(
		constraint.TimeRange,
		"operator-only operation constraint",
		constraint.TimeRangeParams{StartsAt: now.Add(time.Hour)},
	)
	if err != nil {
		t.Fatal(err)
	}
	capabilities := benefit.OperationCapabilities{
		benefit.OperationCapability{
			Operation: benefit.OperationReverse,
			Modes: []benefit.OperationMode{
				benefit.OperationModeReverseFull,
				benefit.OperationModeReversePartial,
			},
		},
	}
	policies := benefit.OperationPolicies{{
		Operation:   benefit.OperationReverse,
		Constraints: benefit.Constraints{future},
	}}
	ineligible, err := evaluation.EvaluateOperation(
		ctx,
		constraint.DefaultRegistry,
		capabilities,
		policies,
		benefit.OperationReverse,
		benefit.OperationModeReverseFull,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ineligible.Status != benefit.OperationDecisionStatusIneligible ||
		!ineligible.IsSupported() || ineligible.IsEligible() ||
		ineligible.Constraints.Status != benefit.ConstraintReportStatusUnsatisfied ||
		len(ineligible.Constraints.Violations) != 1 || ineligible.Reason == "" {
		t.Fatalf("unexpected ineligible decision: %#v", ineligible)
	}
	if err := ineligible.Validate(); err != nil {
		t.Fatalf("ineligible decision failed validation: %v", err)
	}
	data, err := json.Marshal(ineligible)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if !strings.Contains(encoded, `"Status":"ineligible"`) ||
		strings.Contains(encoded, "operator-only") || strings.Contains(encoded, `"Params"`) ||
		strings.Contains(encoded, `"Remark"`) || strings.Contains(encoded, `"Supported"`) ||
		strings.Contains(encoded, `"Eligible"`) {
		t.Fatalf("operation definition leaked into decision JSON: %s", encoded)
	}

	eligibleInput := input
	eligibleInput.Now = now.Add(2 * time.Hour)
	eligible, err := evaluation.EvaluateOperation(
		ctx,
		constraint.DefaultRegistry,
		capabilities,
		policies,
		benefit.OperationReverse,
		benefit.OperationModeReverseFull,
		eligibleInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	if eligible.Status != benefit.OperationDecisionStatusEligible ||
		!eligible.IsSupported() || !eligible.IsEligible() ||
		eligible.Constraints.Status != benefit.ConstraintReportStatusSatisfied ||
		eligible.Reason != "" || eligible.Details != nil {
		t.Fatalf("unexpected eligible decision: %#v", eligible)
	}
	if err := eligible.Validate(); err != nil {
		t.Fatalf("eligible decision failed validation: %v", err)
	}
}

func TestEvaluateOperationMatchesPoliciesByMode(t *testing.T) {
	capabilities := benefit.OperationCapabilities{{
		Operation: benefit.OperationReverse,
		Modes: []benefit.OperationMode{
			benefit.OperationModeReverseFull,
			benefit.OperationModeReversePartial,
		},
	}}
	policies := benefit.OperationPolicies{{
		Operation:  benefit.OperationReverse,
		MatchModes: []benefit.OperationMode{benefit.OperationModeReversePartial},
		Disabled:   true,
		Remark:     "operator-only partial reversal policy",
	}}

	partial, err := evaluation.EvaluateOperation(
		context.Background(),
		constraint.DefaultRegistry,
		capabilities,
		policies,
		benefit.OperationReverse,
		benefit.OperationModeReversePartial,
		constraint.Input{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if partial.Status != benefit.OperationDecisionStatusUnsupported ||
		partial.Mode != benefit.OperationModeReversePartial ||
		strings.Contains(partial.Reason, "operator-only") {
		t.Fatalf("unexpected partial reverse decision: %#v", partial)
	}

	full, err := evaluation.EvaluateOperation(
		context.Background(),
		constraint.DefaultRegistry,
		capabilities,
		policies,
		benefit.OperationReverse,
		benefit.OperationModeReverseFull,
		constraint.Input{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if full.Status != benefit.OperationDecisionStatusEligible || !full.IsEligible() {
		t.Fatalf("unexpected full reverse decision: %#v", full)
	}

	operation, err := evaluation.EvaluateOperation(
		context.Background(),
		constraint.DefaultRegistry,
		capabilities,
		policies,
		benefit.OperationReverse,
		"",
		constraint.Input{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if operation.Status != benefit.OperationDecisionStatusEligible {
		t.Fatalf("mode-specific policy unexpectedly matched operation-level evaluation: %#v", operation)
	}
}

func TestEvaluateOperationRejectsPoliciesOutsideCapability(t *testing.T) {
	capabilities := benefit.OperationCapabilities{{
		Operation: benefit.OperationReverse,
		Modes:     []benefit.OperationMode{benefit.OperationModeReverseFull},
	}}
	tests := map[string]benefit.OperationPolicies{
		"unsupported operation": {{Operation: "Archive", Disabled: true}},
		"unsupported mode": {{
			Operation:  benefit.OperationReverse,
			MatchModes: []benefit.OperationMode{benefit.OperationModeReversePartial},
			Disabled:   true,
		}},
	}

	for name, policies := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := evaluation.EvaluateOperation(
				context.Background(),
				constraint.DefaultRegistry,
				capabilities,
				policies,
				benefit.OperationReverse,
				benefit.OperationModeReverseFull,
				constraint.Input{},
			); !errors.Is(err, benefit.ErrValidation) {
				t.Fatalf("expected validation category for policy outside driver capability, got %v", err)
			}
		})
	}
}

func TestEvaluateLocalEligibilityIncludesUnknownConstraints(t *testing.T) {
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	known := mustConstraint(t, constraint.TimeRange, constraint.TimeRangeParams{
		StartsAt:  now.Add(-time.Hour),
		ExpiresAt: now.Add(time.Hour),
	})
	unknown := mustConstraint(t, "provider.special_rule", map[string]any{"enabled": true})
	input := constraint.Input{
		Benefit: benefit.BenefitInfo{
			Status:      benefit.StatusActive,
			Constraints: benefit.Constraints{known, unknown},
		},
		Now: now,
	}

	result, err := evaluation.EvaluateLocalEligibility(context.Background(), nil, input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Eligible || result.Failure == nil ||
		result.Failure.Code != benefit.EvaluationFailureConstraintUnsatisfied {
		t.Fatalf("unexpected eligibility result: %#v", result)
	}
	if result.Constraints.Status != benefit.ConstraintReportStatusUnsatisfied {
		t.Fatalf("unexpected constraint report status: %#v", result.Constraints)
	}
	if result.Constraints.Unrecognized != 1 {
		t.Fatalf("unexpected unrecognized count: %#v", result.Constraints)
	}
}

func TestEvaluateLocalEligibilityDoesNotEvaluateInactiveBenefitConstraints(t *testing.T) {
	result, err := evaluation.EvaluateLocalEligibility(
		context.Background(),
		nil,
		constraint.Input{
			Benefit: benefit.BenefitInfo{Status: benefit.StatusExpired},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Eligible || result.Constraints.IsEvaluated() ||
		result.Constraints.Status != benefit.ConstraintReportStatusUnevaluated {
		t.Fatalf("unexpected inactive benefit result: %#v", result)
	}
	if result.Failure == nil || result.Failure.Code != benefit.EvaluationFailureBenefitInactive {
		t.Fatalf("unexpected inactive benefit failure: %#v", result.Failure)
	}
}

func mustConstraint(t *testing.T, typ benefit.ConstraintType, params any) benefit.Constraint {
	t.Helper()
	rule, err := benefit.NewConstraint(typ, "", params)
	if err != nil {
		t.Fatal(err)
	}
	return rule
}
