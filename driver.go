package benefit

import (
	"context"
	"errors"
	"fmt"
)

// DriverType identifies a provider-specific benefit kind.
type DriverType string

// String returns the string representation of the driver type.
func (t DriverType) String() string {
	return string(t)
}

// Validate verifies the driver type format.
func (t DriverType) Validate() error {
	return validateNamespacedValue("driver type", t.String())
}

// DriverDescriptor is the global, configuration-independent driver metadata.
// Operations contains only optional capabilities beyond Inspect, Evaluate,
// and Redeem, which every Driver supports unconditionally.
type DriverDescriptor struct {
	Name       string            `json:"name"`
	Type       DriverType        `json:"type"`
	Kind       TypeDescriptor    `json:"kind"`
	Provider   TypeDescriptor    `json:"provider"`
	Operations OperationSupports `json:"operations,omitempty"`
}

// Descriptor returns the type descriptor of the driver.
func (d DriverDescriptor) Descriptor() TypeDescriptor {
	descriptor := TypeDescriptor{
		Name: d.Name,
		Type: d.Type.String(),
		Icon: d.Kind.Icon,
	}

	if descriptor.Icon == "" {
		descriptor.Icon = d.Provider.Icon
	}

	return descriptor
}

// Validate verifies descriptor identity and declared operations.
func (d DriverDescriptor) Validate() error {
	if err := d.Type.Validate(); err != nil {
		return err
	}
	if d.Name == "" {
		return errors.New("benefit: driver descriptor name is empty")
	}
	if d.Kind.Type == "" {
		return errors.New("benefit: driver kind type is empty")
	}
	if d.Provider.Type == "" {
		return errors.New("benefit: driver provider type is empty")
	}

	if value, expected := d.Type.String(), d.Provider.Type+"."+d.Kind.Type; value != expected {
		const msg = "benefit: driver type %q must equal provider and kind %q"
		return fmt.Errorf(msg, d.Type, expected)
	}

	if err := d.Operations.Validate(); err != nil {
		return err
	}
	return nil
}

// DriverDefinition describes, validates, and compiles one driver type.
type DriverDefinition interface {
	Descriptor() DriverDescriptor

	// ConfigSchema must return an equivalent schema on every call.
	ConfigSchema() ConfigSchema

	ValidateConfig(ctx context.Context, conf DriverConfig) (err error)

	// CompileConfig validates, parses, and compiles config independently
	// of ValidateConfig, which DriverRegistry.Bind does not call. It is
	// a deterministic, local operation: equal configurations must produce
	// equivalent factories or errors without network or time dependencies.
	//
	// Implementations must be safe for concurrent use.
	CompileConfig(conf DriverConfig) (factory DriverFactory, err error)
}

// Driver is a provider-bound benefit adapter.
//
// Every driver implements the core inspect, evaluate, and redeem operations.
// Evaluate is an optional, non-consuming quote or preflight call; callers are
// never required to call it before Redeem. Redeem is authoritative, must work
// without an EvaluationToken, and must check current eligibility even when an
// earlier Evaluate call returned an eligible result.
//
// Business rejection belongs in result values. Go errors are reserved for
// local invocation failures that do not describe a confirmed provider outcome.
// Implementations must be safe for concurrent operation calls.
type Driver interface {
	// Descriptor returns the global, configuration-independent metadata of
	// the driver type. The returned value must match the DriverDefinition
	// that constructed the driver.
	Descriptor() DriverDescriptor

	Inspect(ctx context.Context, req InspectRequest) (info BenefitInfo, err error)
	Evaluate(ctx context.Context, req EvaluateRequest) (result EvaluationResult, err error)
	Redeem(ctx context.Context, req RedeemRequest) (result RedeemResult, err error)
}

// Reverser is optionally implemented by drivers that support reversing a
// confirmed redemption.
type Reverser interface {
	Reverse(ctx context.Context, req ReverseRequest) (result ReverseResult, err error)
}

// InspectRequest identifies the benefit to inspect.
type InspectRequest struct {
	Reference BenefitReference `json:"-"`
	Context   OperationContext `json:"-"`
}

// EvaluateRequest evaluates a benefit against one operation context.
type EvaluateRequest struct {
	Reference BenefitReference `json:"-"`
	Context   OperationContext `json:"-"`
}

// RedeemRequest performs one uniquely identified redeem operation. Callers
// must reuse RedemptionID when retrying the same uncertain operation.
//
// Drivers should use RedemptionID as a provider idempotency key when supported
// and otherwise cooperate with the host to prevent duplicate consumption.
type RedeemRequest struct {
	RedemptionID string `json:"redemption_id"`

	// EvaluationToken optionally binds the redeem operation to an earlier
	// quote or evaluated snapshot. An empty token is always valid.
	EvaluationToken string `json:"evaluation_token,omitempty"`

	Reference BenefitReference `json:"-"`
	Context   OperationContext `json:"-"`
}

// ReverseRequest performs one uniquely identified reversal of a confirmed
// redemption. Callers must reuse ReversalID when retrying the same uncertain
// operation.
//
// Drivers should use ReversalID as a provider idempotency key when supported
// and otherwise cooperate with the host to prevent duplicate reversal.
type ReverseRequest struct {
	ReversalID   string `json:"reversal_id"`
	RedemptionID string `json:"redemption_id"`
	Reason       string `json:"reason,omitempty"`

	Context OperationContext `json:"-"`
}
