package benefit

// BenefitOutcome describes the value projected or delivered by applying one
// benefit. Additional normalized outcome categories can be added without
// changing driver operation signatures.
type BenefitOutcome struct {
	Discount *DiscountEffect `json:"discount,omitempty"`
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
