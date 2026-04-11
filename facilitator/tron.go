package facilitator

import (
	"context"

	"github.com/gosuda/x402-facilitator/types"
)

type TronFacilitator struct {
}

func NewTronFacilitator(network string, url string, privateKeyHex string) (*TronFacilitator, error) {
	return &TronFacilitator{}, nil
}

func (t *TronFacilitator) Verify(ctx context.Context, payload *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentVerifyResponse, error) {
	return nil, nil
}

func (t *TronFacilitator) Settle(ctx context.Context, payload *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentSettleResponse, error) {
	return nil, nil
}

// Supported advertises this facilitator on /supported. Tron Verify and
// Settle are still stubs; the SupportedResponse carried here is a
// placeholder that a follow-up commit gates to nil until a real
// implementation lands.
func (t *TronFacilitator) Supported() *types.SupportedResponse {
	return &types.SupportedResponse{
		Kinds: []types.SupportedKind{{
			X402Version: int(types.X402VersionV2),
			Scheme:      string(types.Exact),
			Network:     "tron:*",
		}},
		Extensions: []string{},
		Signers:    map[string][]string{"tron:*": nil},
	}
}
