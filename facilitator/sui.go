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

func (t *SuiFacilitator) Supported() []*types.SupportedKind {
	// Verify/Settle are not yet v2-compliant; gated from discovery.
	return nil
}

// CaipFamily returns the CAIP-2 family pattern for Sui networks.
func (t *SuiFacilitator) CaipFamily() string { return "sui:*" }

// GetSigners returns no addresses — the Sui facilitator is a stub.
func (t *SuiFacilitator) GetSigners() []string { return nil }
