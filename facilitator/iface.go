package facilitator

import (
	"context"
	"fmt"
	"strings"

	"github.com/gosuda/x402-facilitator/types"
)

type Facilitator interface {
	Verify(ctx context.Context, payment *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentVerifyResponse, error)
	Settle(ctx context.Context, payment *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentSettleResponse, error)
	Supported() []*types.SupportedKind

	// CaipFamily returns the CAIP-2 network-family pattern this facilitator
	// serves, e.g. "eip155:*" for EVM chains or "solana:*" for Solana
	// clusters. It is used as the key under SupportedResponse.Signers so
	// clients can group fee-paying addresses by blockchain family.
	CaipFamily() string

	// GetSigners returns the fee-paying addresses this facilitator will use
	// to broadcast settlement transactions. These are surfaced in the
	// /supported response so clients know which addresses may appear as
	// the sender of the on-chain settlement.
	GetSigners() []string
}

func NewFacilitator(scheme types.Scheme, network, rpcUrl string, privateKeyHex string) (Facilitator, error) {
	if scheme != types.Exact {
		return nil, fmt.Errorf("unsupported scheme: %s", scheme)
	}

	// Route by CAIP-2 network prefix
	switch {
	case strings.HasPrefix(network, "eip155:"):
		return NewEVMFacilitator(network, rpcUrl, privateKeyHex)
	case strings.HasPrefix(network, "solana:"):
		return NewSolanaFacilitator(network, rpcUrl, privateKeyHex)
	case strings.HasPrefix(network, "sui:"):
		return NewSuiFacilitator(network, rpcUrl, privateKeyHex)
	case strings.HasPrefix(network, "tron:"):
		return NewTronFacilitator(network, rpcUrl, privateKeyHex)
	default:
		return nil, fmt.Errorf("unsupported network: %s", network)
	}
}
