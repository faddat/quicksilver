package keeper

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ingenuity-build/quicksilver/utils"
	"github.com/ingenuity-build/quicksilver/x/interchainstaking/types"
	"github.com/stretchr/testify/require"
)

func TestDetermineAllocations(t *testing.T) {
	// we auto generate the validator addresses in these tests. any dust gets allocated to the first validator in the list
	// once sorted alphabetically on valoper. So we track the `dust` as a separate value in each test case and allocate to
	// the first validator.

	val1 := utils.GenerateValAddressForTest()
	val2 := utils.GenerateValAddressForTest()
	val3 := utils.GenerateValAddressForTest()
	val4 := utils.GenerateValAddressForTest()

	tc := []struct {
		current  map[string]sdk.Int
		sum      sdk.Int
		target   map[string]*types.ValidatorIntent
		inAmount sdk.Coins
		expected map[string]sdk.Int
		dust     sdk.Int
	}{
		{
			current: map[string]sdk.Int{
				val1.String(): sdk.NewInt(350000),
				val2.String(): sdk.NewInt(650000),
				val3.String(): sdk.NewInt(75000),
			},
			sum: sdk.NewInt(1075000),
			target: map[string]*types.ValidatorIntent{
				val1.String(): {ValoperAddress: val1.String(), Weight: sdk.NewDecWithPrec(30, 2)},
				val2.String(): {ValoperAddress: val2.String(), Weight: sdk.NewDecWithPrec(63, 2)},
				val3.String(): {ValoperAddress: val3.String(), Weight: sdk.NewDecWithPrec(7, 2)},
			},
			inAmount: sdk.NewCoins(sdk.NewCoin("uqck", sdk.NewInt(50000))),
			expected: map[string]sdk.Int{
				val1.String(): sdk.ZeroInt(),
				val2.String(): sdk.NewInt(33181),
				val3.String(): sdk.NewInt(16818),
			},
			dust: sdk.OneInt(),
		},
		{
			current: map[string]sdk.Int{
				val1.String(): sdk.NewInt(52),
				val2.String(): sdk.NewInt(24),
				val3.String(): sdk.NewInt(20),
				val4.String(): sdk.NewInt(4),
			},
			sum: sdk.NewInt(100),
			target: map[string]*types.ValidatorIntent{
				val1.String(): {ValoperAddress: val1.String(), Weight: sdk.NewDecWithPrec(50, 2)},
				val2.String(): {ValoperAddress: val2.String(), Weight: sdk.NewDecWithPrec(25, 2)},
				val3.String(): {ValoperAddress: val3.String(), Weight: sdk.NewDecWithPrec(15, 2)},
				val4.String(): {ValoperAddress: val3.String(), Weight: sdk.NewDecWithPrec(10, 2)},
			},
			inAmount: sdk.NewCoins(sdk.NewCoin("uqck", sdk.NewInt(20))),
			expected: map[string]sdk.Int{
				val4.String(): sdk.NewInt(11),
				val3.String(): sdk.ZeroInt(),
				val2.String(): sdk.NewInt(6),
				val1.String(): sdk.NewInt(3),
			},
			dust: sdk.ZeroInt(),
		},
		{
			current: map[string]sdk.Int{
				val1.String(): sdk.NewInt(52),
				val2.String(): sdk.NewInt(24),
				val3.String(): sdk.NewInt(20),
				val4.String(): sdk.NewInt(4),
			},
			sum: sdk.NewInt(100),
			target: map[string]*types.ValidatorIntent{
				val1.String(): {ValoperAddress: val1.String(), Weight: sdk.NewDecWithPrec(50, 2)},
				val2.String(): {ValoperAddress: val2.String(), Weight: sdk.NewDecWithPrec(25, 2)},
				val3.String(): {ValoperAddress: val3.String(), Weight: sdk.NewDecWithPrec(15, 2)},
				val4.String(): {ValoperAddress: val3.String(), Weight: sdk.NewDecWithPrec(10, 2)},
			},
			inAmount: sdk.NewCoins(sdk.NewCoin("uqck", sdk.NewInt(50))),
			expected: map[string]sdk.Int{
				val4.String(): sdk.NewInt(18),
				val2.String(): sdk.NewInt(13),
				val1.String(): sdk.NewInt(10),
				val3.String(): sdk.NewInt(7),
			},
			dust: sdk.NewInt(2),
		},

		// test to check for div-by-zero when no existing allocations exist.
		{
			current: map[string]sdk.Int{},
			sum:     sdk.NewInt(0),
			target: map[string]*types.ValidatorIntent{
				val1.String(): {ValoperAddress: val1.String(), Weight: sdk.NewDecWithPrec(25, 2)},
				val2.String(): {ValoperAddress: val2.String(), Weight: sdk.NewDecWithPrec(25, 2)},
				val3.String(): {ValoperAddress: val3.String(), Weight: sdk.NewDecWithPrec(25, 2)},
				val4.String(): {ValoperAddress: val3.String(), Weight: sdk.NewDecWithPrec(25, 2)},
			},
			inAmount: sdk.NewCoins(sdk.NewCoin("uqck", sdk.NewInt(50))),
			expected: map[string]sdk.Int{
				val4.String(): sdk.NewInt(12),
				val2.String(): sdk.NewInt(12),
				val1.String(): sdk.NewInt(12),
				val3.String(): sdk.NewInt(12),
			},
			dust: sdk.NewInt(2),
		},
	}

	for caseNumber, val := range tc {
		allocations := determineAllocationsForDelegation(val.current, val.sum, val.target, val.inAmount)
		// as per the comment above, allocate the dust to the expected output of the first validator alphabetically.
		// no dust? short-circuit!
		dustVal := utils.Keys(val.target)[0]
		val.expected[dustVal] = val.expected[dustVal].Add(val.dust)

		require.Equal(t, len(val.expected), len(allocations))
		for valoper := range val.expected {
			ex, ok := val.expected[valoper]
			require.True(t, ok)
			ac, ok := allocations[valoper]
			require.True(t, ok)
			require.True(t, ex.Equal(ac), fmt.Sprintf("Test Case #%d failed; allocations did not equal expected output - expected %s, got %s.", caseNumber, val.expected[valoper], allocations[valoper]))
		}
	}
}
