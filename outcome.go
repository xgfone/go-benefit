package benefit

import "errors"

// BenefitOutcome describes the value projected or delivered by applying one
// benefit. Additional normalized outcome categories can be added without
// changing driver operation signatures.
type BenefitOutcome struct {
	Discount *DiscountEffect `json:",omitzero"`
}

// IsZero reports whether no normalized outcome is present.
func (o BenefitOutcome) IsZero() bool {
	return o.Discount == nil
}

// Validate verifies every normalized outcome component that is present.
func (o BenefitOutcome) Validate() error {
	if o.Discount != nil {
		return o.Discount.Validate()
	}
	return nil
}

// BenefitRestoration describes benefit resources made available again by a
// confirmed reversal.
type BenefitRestoration struct {
	// Money is a restored monetary balance or allowance; it is distinct from
	// the transaction value described by BenefitOutcome.
	//
	// A nil Money means none was supplied; a non-nil pointer distinguishes
	// an explicit zero amount from a non-zero amount. An amount with an empty
	// Currency first inherits Reversed.Discount.Currency when present, then
	// the transaction currency.
	//
	// Use omitempty: omitzero would call Money.IsZero and erase an explicitly
	// supplied zero monetary restoration.
	Money *Money `json:",omitempty"`
}

// IsZero reports whether no normalized restoration is present.
func (r BenefitRestoration) IsZero() bool {
	return r.Money == nil
}

// Validate verifies every normalized restoration component that is present.
func (r BenefitRestoration) Validate() error {
	if r.Money != nil {
		if err := r.Money.Validate(); err != nil {
			return err
		}
		if r.Money.Amount < 0 {
			return errors.New("benefit: restored amount must not be negative")
		}
	}
	return nil
}

// ReversalOutcome describes both the delivered benefit outcome undone by a
// reversal and the consumed benefit resources restored by it.
type ReversalOutcome struct {
	Reversed BenefitOutcome     `json:",omitzero"`
	Restored BenefitRestoration `json:",omitzero"`
}

// IsZero reports whether no normalized reversal outcome is present.
func (o ReversalOutcome) IsZero() bool {
	return o.Reversed.IsZero() && o.Restored.IsZero()
}

// Validate verifies every normalized reversal outcome component present.
func (o ReversalOutcome) Validate() error {
	if err := o.Reversed.Validate(); err != nil {
		return err
	}
	return o.Restored.Validate()
}
