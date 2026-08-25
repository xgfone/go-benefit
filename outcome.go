package benefit

import "errors"

// BenefitOutcome describes the value projected or delivered by applying one
// benefit. Additional normalized outcome categories can be added without
// changing driver operation signatures.
type BenefitOutcome struct {
	Discount *DiscountEffect `json:",omitempty"`
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
// confirmed reversal. Amount is a restored monetary balance or allowance; it
// is distinct from the transaction value described by BenefitOutcome.
type BenefitRestoration struct {
	Amount Money `json:",omitzero"`
}

// IsZero reports whether no normalized restoration is present.
func (r BenefitRestoration) IsZero() bool {
	return r.Amount.IsZero()
}

// Validate verifies every normalized restoration component that is present.
func (r BenefitRestoration) Validate() error {
	if !r.Amount.IsZero() {
		if err := r.Amount.Validate(); err != nil {
			return err
		}
		if r.Amount.Amount < 0 {
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
