package types

import abcitypes "github.com/cometbft/cometbft/abci/types"

func AttributesToMap(attrs []abcitypes.EventAttribute) map[string]string {
	out := make(map[string]string)
	for _, attr := range attrs {
		out[string(attr.Key)] = string(attr.Value)
	}
	return out
}
