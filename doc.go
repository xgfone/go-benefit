// Package benefit defines a provider-neutral benefit redemption framework.
//
// Benefits include coupons, vouchers, electronic cards, virtual goods, and
// internal entitlements. BenefitInfo describes an inspected benefit snapshot,
// and BenefitOutcome describes the value projected or delivered by applying
// one benefit.
//
// Drivers adapt provider-specific APIs to the common model exposed here.
package benefit
