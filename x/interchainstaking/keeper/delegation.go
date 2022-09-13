package keeper

import (
	"math"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	distrTypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingTypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/ingenuity-build/quicksilver/utils"
	"github.com/ingenuity-build/quicksilver/x/interchainstaking/types"
)

// gets the key for delegator bond with validator
// VALUE: staking/Delegation
func GetDelegationKey(zone *types.Zone, delAddr sdk.AccAddress, valAddr sdk.ValAddress) []byte {
	return append(GetDelegationsKey(zone, delAddr), valAddr.Bytes()...)
}

// gets the prefix for a delegator for all validators
func GetDelegationsKey(zone *types.Zone, delAddr sdk.AccAddress) []byte {
	return append(append(types.KeyPrefixDelegation, []byte(zone.ChainId)...), delAddr.Bytes()...)
}

// GetDelegation returns a specific delegation.
func (k Keeper) GetDelegation(ctx sdk.Context, zone *types.Zone, delegatorAddress string, validatorAddress string) (delegation types.Delegation, found bool) {
	store := ctx.KVStore(k.storeKey)

	_, delAddr, _ := bech32.DecodeAndConvert(delegatorAddress)
	_, valAddr, _ := bech32.DecodeAndConvert(validatorAddress)

	key := GetDelegationKey(zone, delAddr, valAddr)

	value := store.Get(key)
	if value == nil {
		return delegation, false
	}

	delegation = types.MustUnmarshalDelegation(k.cdc, value)

	return delegation, true
}

// IterateAllDelegations iterates through all of the delegations.
func (k Keeper) IterateAllDelegations(ctx sdk.Context, zone *types.Zone, cb func(delegation types.Delegation) (stop bool)) {
	store := ctx.KVStore(k.storeKey)

	iterator := sdk.KVStorePrefixIterator(store, append(types.KeyPrefixDelegation, []byte(zone.ChainId)...))
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		delegation := types.MustUnmarshalDelegation(k.cdc, iterator.Value())
		if cb(delegation) {
			break
		}
	}
}

// GetAllDelegations returns all delegations used during genesis dump.
func (k Keeper) GetAllDelegations(ctx sdk.Context, zone *types.Zone) (delegations []types.Delegation) {
	k.IterateAllDelegations(ctx, zone, func(delegation types.Delegation) bool {
		delegations = append(delegations, delegation)
		return false
	})

	return delegations
}

// GetAllDelegations returns all delegations used during genesis dump.
func (k Keeper) GetAllDelegationsAsPointer(ctx sdk.Context, zone *types.Zone) (delegations []*types.Delegation) {
	k.IterateAllDelegations(ctx, zone, func(delegation types.Delegation) bool {
		delegations = append(delegations, &delegation)
		return false
	})

	return delegations
}

// GetValidatorDelegations returns all delegations to a specific validator.
// Useful for querier.
func (k Keeper) GetValidatorDelegations(ctx sdk.Context, zone *types.Zone, valAddr sdk.ValAddress) (delegations []types.Delegation) { //nolint:interfacer
	k.IterateAllDelegations(ctx, zone, func(delegation types.Delegation) bool {
		if delegation.GetValidatorAddr().Equals(valAddr) {
			delegations = append(delegations, delegation)
		}
		return false
	})

	return delegations
}

// GetDelegatorDelegations returns a given amount of all the delegations from a
// delegator.
func (k Keeper) GetDelegatorDelegations(ctx sdk.Context, zone *types.Zone, delegator sdk.AccAddress) (delegations []types.Delegation) {
	k.IterateDelegatorDelegations(ctx, zone, delegator, func(delegation types.Delegation) bool {
		delegations = append(delegations, delegation)
		return false
	})

	return delegations
}

// SetDelegation sets a delegation.
func (k Keeper) SetDelegation(ctx sdk.Context, zone *types.Zone, delegation types.Delegation) {
	delegatorAddress := delegation.GetDelegatorAddr()

	store := ctx.KVStore(k.storeKey)
	b := types.MustMarshalDelegation(k.cdc, delegation)
	store.Set(GetDelegationKey(zone, delegatorAddress, delegation.GetValidatorAddr()), b)
}

// RemoveDelegation removes a delegation
func (k Keeper) RemoveDelegation(ctx sdk.Context, zone *types.Zone, delegation types.Delegation) error {
	delegatorAddress := delegation.GetDelegatorAddr()

	store := ctx.KVStore(k.storeKey)
	store.Delete(GetDelegationKey(zone, delegatorAddress, delegation.GetValidatorAddr()))
	return nil
}

// IterateDelegatorDelegations iterates through one delegator's delegations.
func (k Keeper) IterateDelegatorDelegations(ctx sdk.Context, zone *types.Zone, delegator sdk.AccAddress, cb func(delegation types.Delegation) (stop bool)) {
	store := ctx.KVStore(k.storeKey)
	delegatorPrefixKey := GetDelegationsKey(zone, delegator)
	iterator := sdk.KVStorePrefixIterator(store, delegatorPrefixKey)
	defer iterator.Close()

	for ; iterator.Valid(); iterator.Next() {
		delegation := types.MustUnmarshalDelegation(k.cdc, iterator.Value())
		if cb(delegation) {
			break
		}
	}
}

func (k *Keeper) PrepareDelegationMessagesForCoins(ctx sdk.Context, zone *types.Zone, allocations map[string]sdk.Int) []sdk.Msg {
	var msgs []sdk.Msg
	for _, valoper := range utils.Keys(allocations) {
		if !allocations[valoper].IsZero() {
			msgs = append(msgs, &stakingTypes.MsgDelegate{DelegatorAddress: zone.DelegationAddress.Address, ValidatorAddress: valoper, Amount: sdk.NewCoin(zone.BaseDenom, allocations[valoper])})
		}
	}
	return msgs
}

func (k *Keeper) PrepareDelegationMessagesForShares(ctx sdk.Context, zone *types.Zone, coins sdk.Coins) []sdk.Msg {
	var msgs []sdk.Msg
	for _, coin := range coins.Sort() {
		if !coin.IsZero() {
			msgs = append(msgs, &stakingTypes.MsgRedeemTokensforShares{DelegatorAddress: zone.DelegationAddress.Address, Amount: coin})
		}
	}
	return msgs
}

func (k Keeper) DeterminePlanForDelegation(ctx sdk.Context, zone *types.Zone, amount sdk.Coins) map[string]sdk.Int {
	currentAllocations, currentSum := k.GetDelegationMap(ctx, zone)
	targetAllocations := zone.GetAggregateIntentOrDefault()
	allocations := DetermineAllocationsForDelegation(currentAllocations, currentSum, targetAllocations, amount)
	return allocations
}

// CalculateDeltas determines, for the current delegations, in delta between actual allocations and the target intent.
func calculateDeltas(currentAllocations map[string]sdk.Int, currentSum sdk.Int, targetAllocations map[string]*types.ValidatorIntent) []types.ValidatorIntent {
	deltas := make([]types.ValidatorIntent, 0)

	// for target allocations, raise the intent weight by the total delegated value to get target amount
	for _, valoper := range utils.Keys(targetAllocations) {
		current, ok := currentAllocations[valoper]
		if !ok {
			current = sdk.ZeroInt()
		}
		target := targetAllocations[valoper].Weight.MulInt(currentSum).TruncateInt()
		// diff between target and current allocations
		// positive == below target, negative == above target
		delta := target.Sub(current)
		deltas = append(deltas, types.ValidatorIntent{Weight: delta.ToDec(), ValoperAddress: valoper})
	}

	return deltas
}

// minDeltas returns the lowest value in a slice of Deltas.
func minDeltas(deltas []types.ValidatorIntent) sdk.Int {
	minValue := sdk.NewInt(math.MaxInt64)
	for _, intent := range deltas {
		if minValue.GT(intent.Weight.TruncateInt()) {
			minValue = intent.Weight.TruncateInt()
		}
	}

	return minValue
}

func DetermineAllocationsForDelegation(currentAllocations map[string]sdk.Int, currentSum sdk.Int, targetAllocations map[string]*types.ValidatorIntent, amount sdk.Coins) map[string]sdk.Int {
	input := amount[0].Amount
	deltas := calculateDeltas(currentAllocations, currentSum, targetAllocations)
	minValue := minDeltas(deltas)
	sum := sdk.ZeroInt()

	// sort keys by relative value of delta
	sort.SliceStable(deltas, func(i, j int) bool {
		return deltas[i].ValoperAddress > deltas[j].ValoperAddress
	})

	// sort keys by relative value of delta
	sort.SliceStable(deltas, func(i, j int) bool {
		return deltas[i].Weight.GT(deltas[j].Weight)
	})

	// raise all deltas such that the minimum value is zero.
	for idx := range deltas {
		deltas[idx].Weight = deltas[idx].Weight.Add(minValue.Abs().ToDec())
		sum = sum.Add(deltas[idx].Weight.TruncateInt())
	}

	// unequalSplit is the portion of input that should be distributed in attempt to make targets == 0
	unequalSplit := sdk.MinInt(sum, input)

	outSum := sdk.ZeroDec()
	if !unequalSplit.IsZero() {
		for idx := range deltas {
			deltas[idx].Weight = deltas[idx].Weight.QuoInt(sum).MulInt(unequalSplit)
			outSum = outSum.Add(deltas[idx].Weight)
		}
	}

	// equalSplit is the portion of input that should be distributed equally across all validators, once targets are zero.
	equalSplit := input.Sub(unequalSplit).ToDec()

	if !equalSplit.IsZero() {
		outSum = sdk.ZeroDec() // rezero outsum
		each := equalSplit.Quo(sdk.NewDec(int64(len(deltas))))
		for idx := range deltas {
			deltas[idx].Weight = deltas[idx].Weight.Add(each)
			outSum = outSum.Add(deltas[idx].Weight)
		}
	}

	// dust is the portion of the input that was truncated in previous calculations; add this to the first validator in the list,
	// once sorted alphabetically. This will always be a small amount, and will count toward the delta calculations on the next run.
	dust := input.Sub(outSum.TruncateInt())
	deltas[0].Weight = deltas[0].Weight.Add(dust.ToDec())

	outWeights := make(map[string]sdk.Int)
	for _, delta := range deltas {
		outWeights[delta.ValoperAddress] = delta.Weight.TruncateInt()
	}

	return outWeights
}

func (k *Keeper) WithdrawDelegationRewardsForResponse(ctx sdk.Context, zone *types.Zone, delegator string, response []byte) error {
	var msgs []sdk.Msg

	delegatorRewards := distrTypes.QueryDelegationTotalRewardsResponse{}
	err := k.cdc.Unmarshal(response, &delegatorRewards)
	if err != nil {
		return err
	}
	account := zone.DelegationAddress

	var delAddr sdk.AccAddress
	_, delAddr, _ = bech32.DecodeAndConvert(delegator)

	// send withdrawal msg for each delegation (delegator:validator pairs)
	k.IterateDelegatorDelegations(ctx, zone, delAddr, func(delegation types.Delegation) bool {
		amount := rewardsForDelegation(delegatorRewards, delegation.ValidatorAddress)
		k.Logger(ctx).Info("Withdraw rewards", "delegator", delegation.DelegationAddress, "validator", delegation.ValidatorAddress, "amount", amount)
		if !amount.IsZero() || !amount.Empty() {
			msgs = append(msgs, &distrTypes.MsgWithdrawDelegatorReward{DelegatorAddress: delegation.GetDelegationAddress(), ValidatorAddress: delegation.GetValidatorAddress()})
		}
		return false
	})

	if len(msgs) == 0 {
		// always setZone here because calling method update waitgroup.
		k.SetZone(ctx, zone)
		return nil
	}
	// increment withdrawal waitgroup for every withdrawal msg sent
	// this allows us to track individual msg responses and ensure all
	// responses have been received and handled...
	// HandleWithdrawRewards contains the opposing decrement.
	zone.WithdrawalWaitgroup += uint32(len(msgs))
	k.SetZone(ctx, zone)
	k.Logger(ctx).Info("Received WithdrawDelegationRewardsForResponse acknowledgement", "wg", zone.WithdrawalWaitgroup, "address", delegator)

	return k.SubmitTx(ctx, msgs, account, "")
}

func rewardsForDelegation(delegatorRewards distrTypes.QueryDelegationTotalRewardsResponse, validator string) sdk.DecCoins {
	for _, reward := range delegatorRewards.Rewards {
		if reward.ValidatorAddress == validator {
			return reward.Reward
		}
	}
	return sdk.NewDecCoins()
}

func (k *Keeper) GetDelegationMap(ctx sdk.Context, zone *types.Zone) (map[string]sdk.Int, sdk.Int) {
	out := make(map[string]sdk.Int)
	sum := sdk.ZeroInt()

	k.IterateAllDelegations(ctx, zone, func(delegation types.Delegation) bool {
		existing, found := out[delegation.ValidatorAddress]
		if !found {
			out[delegation.ValidatorAddress] = delegation.Amount.Amount
		} else {
			out[delegation.ValidatorAddress] = existing.Add(delegation.Amount.Amount)
		}
		sum = sum.Add(delegation.Amount.Amount)
		return false
	})

	return out, sum
}
