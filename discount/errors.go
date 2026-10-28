package discount

import (
	"fmt"

	"github.com/xgfone/go-benefit"
)

var (
	// ErrCurrencyMismatch reports a discount currency different from the
	// original currency.
	ErrCurrencyMismatch = fmt.Errorf("%w: discount currency differs from original currency", benefit.ErrValidation)

	// ErrInvalidAmount reports a negative original or discount amount.
	ErrInvalidAmount = fmt.Errorf("%w: amount must not be negative", benefit.ErrValidation)

	// ErrDiscountExceedsOriginal reports a non-negative discount greater than
	// the original amount.
	ErrDiscountExceedsOriginal = fmt.Errorf("%w: discount amount exceeds original amount", benefit.ErrValidation)

	// ErrInvalidBasisPoints reports discount basis points outside [0, 10000].
	ErrInvalidBasisPoints = fmt.Errorf("%w: discount basis points must be in [0, 10000]", benefit.ErrValidation)
)
