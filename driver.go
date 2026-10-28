package benefit

import (
	"context"
	"errors"
	"fmt"
)

// ErrBenefitNotFound reports that an Inspect reference did not resolve to a
// benefit. Evaluate and Redeem implementations must translate the same
// confirmed provider outcome into their operation-specific failure results.
var ErrBenefitNotFound = errors.New("benefit: benefit not found")

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
// Operations contains the driver type's maximum optional capabilities beyond
// Inspect, Evaluate, and Redeem, which every Driver supports unconditionally.
type DriverDescriptor struct {
	Name string
	Type DriverType

	Kind     TypeDescriptor
	Provider TypeDescriptor

	Operations   OperationCapabilities `json:",omitempty"`
	ConfigSchema ConfigSchema
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

// Validate verifies descriptor identity, config schema and declared operations.
func (d DriverDescriptor) Validate() error {
	if err := d.Type.Validate(); err != nil {
		return err
	}
	if d.Name == "" {
		return fmt.Errorf("%w: driver descriptor name is empty", ErrValidation)
	}
	if d.Kind.Type == "" {
		return fmt.Errorf("%w: driver kind type is empty", ErrValidation)
	}
	if d.Provider.Type == "" {
		return fmt.Errorf("%w: driver provider type is empty", ErrValidation)
	}

	if value, expected := d.Type.String(), d.Provider.Type+"."+d.Kind.Type; value != expected {
		const msg = "%w: driver type %q must equal provider and kind %q"
		return fmt.Errorf(msg, ErrValidation, d.Type, expected)
	}

	if err := d.Operations.Validate(); err != nil {
		return err
	}
	if err := d.ConfigSchema.Validate(); err != nil {
		return fmt.Errorf("benefit: invalid driver %q config schema: %w", d.Type, err)
	}
	return nil
}

// DriverDefinition describes, validates, and compiles one driver type.
type DriverDefinition interface {
	// Descriptor must return equivalent metadata on every call.
	//
	// Callers must treat the metadata and all values reachable from it
	// as immutable.
	Descriptor() DriverDescriptor

	// ValidateConfig validates config without compiling it. It must accept a
	// configuration if and only if CompileConfig accepts it, must not retain
	// config or populate compilation caches, and must not depend on network
	// or time. Implementations should share parsing and validation logic
	// with CompileConfig.
	ValidateConfig(conf DriverConfig) (err error)

	// CompileConfig validates, parses, and compiles config independently
	// of ValidateConfig, which registry binding does not call. It is a
	// deterministic, local operation: equal configurations must produce
	// equivalent factories or errors without network or time dependencies.
	// Implementations and proxies may cache and reuse compiled factories
	// or drivers for equal configurations.
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
// Confirmed business outcomes from Evaluate, Redeem, and Reverse belong in
// result values. Inspect may return documented lookup errors such as
// ErrBenefitNotFound. Other Go errors are reserved for local invocation or
// integration failures that do not describe a confirmed provider outcome.
// Implementations must be safe for concurrent operation calls.
type Driver interface {
	// Descriptor returns the global, configuration-independent metadata of
	// the driver type. The returned value must match the DriverDefinition
	// that constructed the driver.
	//
	// Callers must treat the metadata and all values reachable from it
	// as immutable.
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

// DriverInput contains optional application-defined input supplied for one
// Driver call. The caller and Driver implementation agree on its dynamic type.
// Callers, drivers, and constraint evaluators must treat the contained value
// and all values reachable from it as immutable and must not retain them beyond
// the Driver call.
type DriverInput any

// DriverRequest contains the fields common to every bound Driver call.
type DriverRequest struct {
	Reference BenefitReference `json:"-"`
	Input     DriverInput      `json:"-"`
}

// InspectRequest identifies the benefit to inspect.
type InspectRequest struct {
	DriverRequest
}

// EvaluateRequest evaluates a benefit against one driver input.
type EvaluateRequest struct {
	DriverRequest
}

// RedeemRequest performs one redeem operation for the referenced benefit.
// RedemptionId is an optional caller-assigned correlation and idempotency ID.
//
// When RedemptionId is supplied, callers must reuse it when retrying the same
// uncertain operation. Drivers should use it as a provider idempotency key
// when supported. Without it, callers and drivers can only rely on
// provider-side idempotency.
type RedeemRequest struct {
	DriverRequest

	RedemptionId string `json:",omitempty"`

	// EvaluationToken optionally binds the redeem operation to an earlier
	// quote or evaluated snapshot. An empty token is always valid.
	EvaluationToken string `json:",omitempty"`
}

// ReverseRequest reverses a confirmed redemption for the referenced benefit.
// ReversalId and RedemptionId are optional caller-assigned correlation and
// idempotency IDs. ProviderRedemptionId is an optional provider-assigned
// redemption ID returned by Redeem. A driver may require it in addition to the
// benefit reference when the provider reverses individual redemptions.
// Mode selects a full or partial reversal; an empty Mode means full.
//
// When ReversalId is supplied, callers must reuse it when retrying the same
// uncertain operation. Drivers should use it as a provider idempotency key
// when supported. Without it, callers and drivers can only rely on
// provider-side idempotency.
type ReverseRequest struct {
	DriverRequest

	ReversalId   string `json:",omitempty"`
	RedemptionId string `json:",omitempty"`

	ProviderRedemptionId string `json:",omitempty"`

	Mode   OperationMode `json:",omitempty"`
	Reason string        `json:",omitempty"`
}
