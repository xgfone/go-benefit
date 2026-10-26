package benefit

import "errors"

// CalculateAmountDiscount calculates a fixed discount in integer minor units.
// An empty discount currency inherits original.Currency. A discount greater
// than the original amount is rejected, so drivers can fail before consuming
// a benefit. Applications that cap face value should cap it explicitly first.
func CalculateAmountDiscount(original, discount Money) (DiscountEffect, error) {
	if err := original.Validate(); err != nil {
		return DiscountEffect{}, err
	}
	if err := discount.Validate(); err != nil {
		return DiscountEffect{}, err
	}
	if discount.Currency != "" && !equalCurrencyCode(original.Currency, discount.Currency) {
		return DiscountEffect{}, errors.New("benefit: discount currency differs from original currency")
	}
	if original.Amount < 0 || discount.Amount < 0 || discount.Amount > original.Amount {
		// Check before subtraction to prevent overflow on invalid inputs.
		return DiscountEffect{}, errors.New("benefit: discount amount is outside the original amount")
	}

	return DiscountEffect{
		Type: DiscountEffectAmount,

		Currency: canonicalCurrencyCode(original.Currency),

		OriginalAmount: original.Amount,
		DiscountAmount: discount.Amount,
		PayableAmount:  original.Amount - discount.Amount,
	}, nil
}

// CalculatePercentageDiscount calculates the amount saved, rounding down to
// integer minor units. basisPoints is in [0, 10000]: 1000 saves 10 percent and
// 10000 is free. It avoids floating-point arithmetic and integer overflow even
// when original.Amount is math.MaxInt64.
func CalculatePercentageDiscount(original Money, basisPoints int64) (DiscountEffect, error) {
	if err := original.Validate(); err != nil {
		return DiscountEffect{}, err
	}
	if original.Amount < 0 || basisPoints < 0 || basisPoints > 10000 {
		return DiscountEffect{}, errors.New("benefit: invalid original amount or discount basis points")
	}

	effectType := DiscountEffectPercentage
	if basisPoints == 10000 {
		effectType = DiscountEffectFree
	}

	discount := original.Amount/10000*basisPoints + original.Amount%10000*basisPoints/10000

	return DiscountEffect{
		Type: effectType,

		Currency: canonicalCurrencyCode(original.Currency),

		OriginalAmount: original.Amount,
		DiscountAmount: discount,
		PayableAmount:  original.Amount - discount,
	}, nil
}
