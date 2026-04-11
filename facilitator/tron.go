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

// Supported returns nil so this facilitator does not advertise itself on
// /supported. Verify and Settle are still stubs; a follow-up PR will fill
// them in and return a real SupportedResponse.
func (t *TronFacilitator) Supported() *types.SupportedResponse {
	return nil
}
