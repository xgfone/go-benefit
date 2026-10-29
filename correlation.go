package benefit

import "fmt"

// Correlate validates the request and result, checks caller IDs, and returns
// a result whose missing caller RedemptionId is filled from req. A non-empty
// returned caller ID must equal the request ID, including when the latter is
// empty. Provider-assigned IDs are never filled from caller IDs.
//
// The original result is not modified. On error it returns a zero result;
// mismatched IDs wrap ErrIdMismatch. A validation error after invocation does
// not prove that the provider did not consume the benefit and is not permission
// to retry. Correlation alone provides no idempotency or persistence.
func (r RedeemResult) Correlate(req RedeemRequest) (RedeemResult, error) {
	if err := req.Validate(); err != nil {
		return RedeemResult{}, err
	}
	if err := r.Validate(); err != nil {
		return RedeemResult{}, err
	}

	if r.Redemption != nil {
		err := checkCallerId("RedemptionId", req.RedemptionId, r.Redemption.RedemptionId)
		if err != nil {
			return RedeemResult{}, err
		}

		redemption := *r.Redemption
		redemption.RedemptionId = req.RedemptionId
		r.Redemption = &redemption
	}
	return r, nil
}

// Correlate validates the request and result, checks the caller RedemptionId
// and ReversalId, and fills missing caller IDs in an independent reversal record.
// Non-empty returned caller IDs must equal the request IDs. ProviderRedemptionId
// must match when both request and result supply it. Missing provider IDs remain
// missing; they are never inferred from caller IDs. Other immutable payloads may
// be shared with the original result.
//
// On error it returns a zero result; mismatched IDs wrap ErrIdMismatch. As with
// RedeemResult.Correlate, errors do not prove the absence of a remote effect,
// and correlation does not implement idempotency.
func (r ReverseResult) Correlate(req ReverseRequest) (ReverseResult, error) {
	if err := req.Validate(); err != nil {
		return ReverseResult{}, err
	}
	if err := r.Validate(); err != nil {
		return ReverseResult{}, err
	}

	if r.Reversal != nil {
		err := checkCallerId("RedemptionId", req.RedemptionId, r.Reversal.RedemptionId)
		if err != nil {
			return ReverseResult{}, err
		}

		err = checkCallerId("ReversalId", req.ReversalId, r.Reversal.ReversalId)
		if err != nil {
			return ReverseResult{}, err
		}

		if req.ProviderRedemptionId != "" &&
			r.Reversal.ProviderRedemptionId != "" &&
			req.ProviderRedemptionId != r.Reversal.ProviderRedemptionId {
			return ReverseResult{}, fmt.Errorf("%w: ProviderRedemptionId", ErrIdMismatch)
		}

		reversal := *r.Reversal
		reversal.RedemptionId = req.RedemptionId
		reversal.ReversalId = req.ReversalId
		r.Reversal = &reversal
	}

	return r, nil
}

func checkCallerId(field, requested, returned string) error {
	if returned != "" && returned != requested {
		return fmt.Errorf("%w: %s", ErrIdMismatch, field)
	}
	return nil
}
