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

func (t *TronFacilitator) Supported() []*types.SupportedKind {
	// Verify/Settle are not yet v2-compliant; gated from discovery.
	return nil
}

// CaipFamily returns the CAIP-2 family pattern for Tron networks.
func (t *TronFacilitator) CaipFamily() string { return "tron:*" }

// GetSigners returns no addresses — the Tron facilitator is a stub.
func (t *TronFacilitator) GetSigners() []string { return nil }
