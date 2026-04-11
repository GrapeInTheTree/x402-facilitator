package facilitator

import (
	"context"

	"github.com/gosuda/x402-facilitator/types"
)

type SuiFacilitator struct {
}

func NewSuiFacilitator(network string, url string, privateKeyHex string) (*SuiFacilitator, error) {
	return &SuiFacilitator{}, nil
}

func (t *SuiFacilitator) Verify(ctx context.Context, payload *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentVerifyResponse, error) {
	return nil, nil
}

func (t *SuiFacilitator) Settle(ctx context.Context, payload *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentSettleResponse, error) {
	return nil, nil
}

// Supported advertises this facilitator on /supported. Sui Verify and
// Settle are still stubs; the SupportedResponse carried here is a
// placeholder that a follow-up commit gates to nil until a real
// implementation lands.
func (t *SuiFacilitator) Supported() *types.SupportedResponse {
	return &types.SupportedResponse{
		Kinds: []types.SupportedKind{{
			X402Version: int(types.X402VersionV2),
			Scheme:      string(types.Exact),
			Network:     "sui:*",
		}},
		Extensions: []string{},
		Signers:    map[string][]string{"sui:*": nil},
	}
}
