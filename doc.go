// Package benefit defines a provider-neutral benefit redemption framework.
//
// Benefits include coupons, vouchers, electronic cards, virtual goods, and
// internal entitlements. BenefitInfo describes an inspected benefit snapshot,
// and BenefitOutcome describes the value projected or delivered by applying
// one benefit.
//
// Drivers adapt provider-specific APIs to the common model exposed here.
// The core package contains driver contracts, normalized data, and model
// validation. Optional subpackages provide driver registration and binding
// (driver), constraint execution (constraint), local evaluation policies
// (evaluation), and monetary calculations (discount).
//
// Model and request validation errors match ErrValidation with errors.Is.
// Configuration, schema, currency, and constraint parameter failures also
// match their more specific error categories. Wrapped encoding and decoding
// errors remain available through errors.Is and errors.As. Error messages
// provide diagnostic context and are not stable machine identifiers.
package benefit
