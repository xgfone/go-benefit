package discount

import (
	"errors"
	"strings"

	"github.com/xgfone/go-benefit"
)

// CalculateAmount calculates a fixed discount in integer minor units.
// An empty discount currency inherits original.Currency. A discount greater
// than the original amount is rejected, so drivers can fail before consuming
// a benefit. Applications that cap face value should cap it explicitly first.
func CalculateAmount(original, discount benefit.Money) (benefit.DiscountEffect, error) {
	if err := original.Validate(); err != nil {
		return benefit.DiscountEffect{}, err
	}
	if err := discount.Validate(); err != nil {
		return benefit.DiscountEffect{}, err
	}
	if discount.Currency != "" && !strings.EqualFold(original.Currency, discount.Currency) {
		return benefit.DiscountEffect{}, errors.New("benefit: discount currency differs from original currency")
	}
	if original.Amount < 0 || discount.Amount < 0 || discount.Amount > original.Amount {
		// Check before subtraction to prevent overflow on invalid inputs.
		return benefit.DiscountEffect{}, errors.New("benefit: discount amount is outside the original amount")
	}

	return benefit.DiscountEffect{
		Type: benefit.DiscountEffectAmount,

		Currency: strings.ToUpper(original.Currency),

		OriginalAmount: original.Amount,
		DiscountAmount: discount.Amount,
		PayableAmount:  original.Amount - discount.Amount,
	}, nil
}

// CalculatePercentage calculates the amount saved, rounding down to
// integer minor units. basisPoints is in [0, 10000]: 1000 saves 10 percent and
// 10000 is free. It avoids floating-point arithmetic and integer overflow even
// when original.Amount is math.MaxInt64.
func CalculatePercentage(original benefit.Money, basisPoints int64) (benefit.DiscountEffect, error) {
	if err := original.Validate(); err != nil {
		return benefit.DiscountEffect{}, err
	}
	if original.Amount < 0 || basisPoints < 0 || basisPoints > 10000 {
		return benefit.DiscountEffect{}, errors.New("benefit: invalid original amount or discount basis points")
	}

	effectType := benefit.DiscountEffectPercentage
	if basisPoints == 10000 {
		effectType = benefit.DiscountEffectFree
	}

	discount := original.Amount/10000*basisPoints + original.Amount%10000*basisPoints/10000

	return benefit.DiscountEffect{
		Type: effectType,

		Currency: strings.ToUpper(original.Currency),

		OriginalAmount: original.Amount,
		DiscountAmount: discount,
		PayableAmount:  original.Amount - discount,
	}, nil
}
