package utils

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
)

func AccAddressFromBech32(address string, checkHRP string) (addr sdk.AccAddress, err error) {
	if len(strings.TrimSpace(address)) == 0 {
		return sdk.AccAddress{}, errors.New("empty address string is not allowed")
	}

	hrp, bz, err := bech32.DecodeAndConvert(address)
	if err != nil {
		return nil, err
	}

	if checkHRP != "" {
		if checkHRP != hrp {
			return sdk.AccAddress{}, fmt.Errorf("unexpected hrp - got %s expected %s", hrp, checkHRP)
		}
	}

	err = sdk.VerifyAddressFormat(bz)
	if err != nil {
		return nil, err
	}

	return sdk.AccAddress(bz), nil
}

func ValAddressFromBech32(address string, checkHRP string) (addr sdk.ValAddress, err error) {
	if len(strings.TrimSpace(address)) == 0 {
		return sdk.ValAddress{}, errors.New("empty address string is not allowed")
	}

	hrp, bz, err := bech32.DecodeAndConvert(address)
	if err != nil {
		return nil, err
	}

	if checkHRP != "" {
		if checkHRP != hrp {
			return sdk.ValAddress{}, fmt.Errorf("unexpected hrp - got %s expected %s", hrp, checkHRP)
		}
	}

	err = sdk.VerifyAddressFormat(bz)
	if err != nil {
		return nil, err
	}

	return sdk.ValAddress(bz), nil
}

func GenerateAccAddressForTest() sdk.AccAddress {
	size := 32 // change the length of the generated random string here

	rb := make([]byte, size)
	_, err := rand.Read(rb)
	if err != nil {
		panic(err)
	}

	return sdk.AccAddress(rb)
}

func GenerateValAddressForTest() sdk.ValAddress {
	size := 32 // change the length of the generated random string here

	rb := make([]byte, size)
	_, err := rand.Read(rb)
	if err != nil {
		panic(err)
	}

	return sdk.ValAddress(rb)
}
