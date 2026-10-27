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
package benefit
