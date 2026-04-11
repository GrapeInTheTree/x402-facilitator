package facilitator

import (
	"context"
	"fmt"
	"strings"

	"github.com/gosuda/x402-facilitator/types"
)

// Facilitator is the per-network payment facilitator interface. Each
// implementation serves a single CAIP-2 network family (EVM, Solana, ...)
// and exposes exactly one method per public HTTP endpoint: Verify,
// Settle, and Supported.
type Facilitator interface {
	// Verify checks whether the payment payload is valid for the given
	// requirements without broadcasting anything on-chain.
	Verify(ctx context.Context, payment *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentVerifyResponse, error)
	// Settle broadcasts the payment on-chain and returns the transaction
	// hash once the node accepts it.
	Settle(ctx context.Context, payment *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentSettleResponse, error)
	// Supported returns the full /supported response for this facilitator
	// — its advertised kinds, protocol extensions, and fee-payer signer
	// addresses grouped by CAIP-2 family.
	Supported() *types.SupportedResponse
}

// NewFacilitator constructs the facilitator for a given scheme and CAIP-2
// network. This facilitator only implements the x402 v2 "exact" scheme;
// the network family is inferred from the CAIP-2 namespace prefix.
func NewFacilitator(scheme types.Scheme, network, rpcUrl string, privateKeyHex string) (Facilitator, error) {
	if scheme != types.Exact {
		return nil, fmt.Errorf("unsupported scheme %q (only %q is implemented)", scheme, types.Exact)
	}

	// Route by CAIP-2 namespace prefix. The network value itself (e.g.
	// "eip155:84532") is passed through to the per-family constructor so
	// it can validate the concrete chain.
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
		return nil, fmt.Errorf("unsupported network %q: expected a CAIP-2 identifier (eip155:*, solana:*, sui:*, tron:*)", network)
	}
}
