package keeper

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ingenuity-build/quicksilver/x/interchainstaking/types"
)

func (k Keeper) getStoreKey(zone types.Zone, snapshot bool) []byte {
	if snapshot {
		return append(types.KeyPrefixSnapshotIntent, []byte(zone.ChainId)...)
	}
	return append(types.KeyPrefixIntent, []byte(zone.ChainId)...)
}

// GetIntent returns intent info by zone and delegator
func (k Keeper) GetIntent(ctx sdk.Context, zone types.Zone, delegator string, snapshot bool) (types.DelegatorIntent, bool) {
	intent := types.DelegatorIntent{}
	store := prefix.NewStore(ctx.KVStore(k.storeKey), k.getStoreKey(zone, snapshot))
	bz := store.Get([]byte(delegator))
	if len(bz) == 0 {
		// usually we'd return false here, but we always want to return an empty intent if one doesn't exist; keep standard Get* interface for consistency.
		return types.DelegatorIntent{Delegator: delegator, Intents: []*types.ValidatorIntent{}}, true
	}
	k.cdc.MustUnmarshal(bz, &intent)
	return intent, true
}

// SetIntent store the delegator intent
func (k Keeper) SetIntent(ctx sdk.Context, zone types.Zone, intent types.DelegatorIntent, snapshot bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), k.getStoreKey(zone, snapshot))
	bz := k.cdc.MustMarshal(&intent)
	store.Set([]byte(intent.Delegator), bz)
}

// DeleteIntent deletes delegator intent
func (k Keeper) DeleteIntent(ctx sdk.Context, zone types.Zone, delegator string, snapshot bool) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), k.getStoreKey(zone, snapshot))
	store.Delete([]byte(delegator))
}

// IterateIntents iterate through intents for a given zone
func (k Keeper) IterateIntents(ctx sdk.Context, zone types.Zone, snapshot bool, fn func(index int64, intent types.DelegatorIntent) (stop bool)) {
	store := prefix.NewStore(ctx.KVStore(k.storeKey), k.getStoreKey(zone, snapshot))

	iterator := sdk.KVStorePrefixIterator(store, nil)
	defer iterator.Close()

	i := int64(0)

	for ; iterator.Valid(); iterator.Next() {
		intent := types.DelegatorIntent{}
		k.cdc.MustUnmarshal(iterator.Value(), &intent)

		stop := fn(i, intent)

		if stop {
			break
		}
		i++
	}
}

// AllIntents returns every intent in the store for the specified zone
func (k Keeper) AllIntents(ctx sdk.Context, zone types.Zone, snapshot bool) []types.DelegatorIntent {
	intents := []types.DelegatorIntent{}
	k.IterateIntents(ctx, zone, snapshot, func(_ int64, intent types.DelegatorIntent) (stop bool) {
		intents = append(intents, intent)
		return false
	})
	return intents
}

// AllIntents returns every intent in the store for the specified zone
func (k Keeper) AllIntentsAsPointer(ctx sdk.Context, zone types.Zone, snapshot bool) []*types.DelegatorIntent {
	intents := []*types.DelegatorIntent{}
	k.IterateIntents(ctx, zone, snapshot, func(_ int64, intent types.DelegatorIntent) (stop bool) {
		intents = append(intents, &intent)
		return false
	})
	return intents
}

// AllOrdinalizedIntents returns every intent in the store for the specified zone.
func (k Keeper) AllOrdinalizedIntents(ctx sdk.Context, zone types.Zone, snapshot bool) ([]types.DelegatorIntent, error) {
	intents := []types.DelegatorIntent{}
	var err error
	k.IterateIntents(ctx, zone, snapshot, func(_ int64, intent types.DelegatorIntent) (stop bool) {
		addr, localErr := sdk.AccAddressFromBech32(intent.Delegator)
		if localErr != nil {
			err = localErr
			return true
		}
		balance := k.BankKeeper.GetBalance(ctx, addr, zone.LocalDenom)

		intents = append(intents, intent.Ordinalize(balance.Amount.ToDec()))
		return false
	})
	if err != nil {
		// check on nil here to ensure we don't return half a slice of intents
		return []types.DelegatorIntent{}, err
	}
	return intents, nil
}

func (k *Keeper) AggregateIntents(ctx sdk.Context, zone types.Zone) error {
	var err error
	snapshot := false
	intents := map[string]*types.ValidatorIntent{}
	ordinalizedIntentSum := sdk.ZeroDec()
	// reduce intents
	k.IterateIntents(ctx, zone, snapshot, func(_ int64, intent types.DelegatorIntent) (stop bool) {
		addr, localErr := sdk.AccAddressFromBech32(intent.Delegator)
		if localErr != nil {
			err = localErr
			return true
		}
		balance := k.BankKeeper.GetBalance(ctx, addr, zone.LocalDenom)

		for _, vIntent := range intent.Ordinalize(balance.Amount.ToDec()).Intents {
			thisIntent, ok := intents[vIntent.ValoperAddress]
			ordinalizedIntentSum = ordinalizedIntentSum.Add(vIntent.Weight)
			if !ok {
				intents[vIntent.ValoperAddress] = vIntent
			} else {
				thisIntent.Weight = thisIntent.Weight.Add(vIntent.Weight)
				intents[vIntent.ValoperAddress] = thisIntent
			}
		}

		return false
	})
	if err != nil {
		return err
	}

	if len(intents) > 0 && ordinalizedIntentSum.IsZero() {
		return fmt.Errorf("ordinalized intent sum is zero, this should never happen")
	}

	// normalise aggregated intents again.
	for key, val := range intents {
		val.Weight = val.Weight.Quo(ordinalizedIntentSum)
		intents[key] = val
	}

	zone.AggregateIntent = intents
	k.SetZone(ctx, &zone)
	return nil
}

func (k *Keeper) UpdateIntent(ctx sdk.Context, sender sdk.AccAddress, zone types.Zone, inAmount sdk.Coins, memo string) error {
	snapshot := false
	// this is here because we need access to the bankKeeper to ordinalize intent
	intent, _ := k.GetIntent(ctx, zone, sender.String(), snapshot)

	// ordinalize
	balance := k.BankKeeper.GetBalance(ctx, sender, zone.BaseDenom)

	// inAmount is ordinal with respect to the redemption rate, so we must scale
	baseBalance := zone.RedemptionRate.Mul(sdk.NewDecFromInt(balance.Amount))

	if inAmount.IsValid() {
		intent = zone.UpdateIntentWithCoins(intent, baseBalance, inAmount)
	}

	if len(memo) > 0 {
		var err error
		intent, err = zone.UpdateIntentWithMemo(intent, memo, baseBalance, inAmount)
		if err != nil {
			return err
		}
	}

	if len(intent.Intents) == 0 {
		return nil
	}

	k.SetIntent(ctx, zone, intent, snapshot)
	return nil
}
