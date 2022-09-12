package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ingenuity-build/quicksilver/utils"
)

func (v Validator) SharesToTokens(shares sdk.Dec) sdk.Int {
	if v.DelegatorShares.IsZero() {
		return sdk.ZeroInt()
	}

	return v.VotingPower.ToDec().Quo(v.DelegatorShares).TruncateInt()
}

func (di DelegatorIntent) AddOrdinal(multiplier sdk.Dec, intents ValidatorIntents) DelegatorIntent {
	if len(intents) == 0 {
		return di
	}

	if len(di.Intents) == 0 {
		di.Intents = make(map[string]*ValidatorIntent, 0)
	}

	di = di.Ordinalize(multiplier)

OUTER:
	for _, idx := range utils.Keys(intents) {
		if i, ok := intents[idx]; ok {
			for _, j := range utils.Keys(di.Intents) {
				if i.ValoperAddress == di.Intents[j].ValoperAddress {
					di.Intents[j].Weight = di.Intents[j].Weight.Add(i.Weight)
					continue OUTER
				}
			}
			di.Intents[i.ValoperAddress] = i
		}
	}

	return di.Normalize()
}

func (di DelegatorIntent) Normalize() DelegatorIntent {
	summedWeight := sdk.ZeroDec()
	for _, i := range utils.Keys(di.Intents) {
		summedWeight = summedWeight.Add(di.Intents[i].Weight)
	}

	// zero summed weight, we should panic here, something is very wrong...
	if summedWeight.IsZero() {
		return di
	}

	for _, i := range utils.Keys(di.Intents) {
		di.Intents[i].Weight = di.Intents[i].Weight.QuoTruncate(summedWeight)
	}
	return di
}

func (di DelegatorIntent) Ordinalize(multiple sdk.Dec) DelegatorIntent {
	for _, i := range utils.Keys(di.Intents) {
		di.Intents[i].Weight = di.Intents[i].Weight.Mul(multiple)
	}

	return di
}
