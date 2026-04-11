package facilitator

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/blocto/solana-go-sdk/client"
	solTypes "github.com/blocto/solana-go-sdk/types"

	"github.com/gosuda/x402-facilitator/types"
)

type SolanaFacilitator struct {
	scheme   types.Scheme
	client   *client.Client
	feePayer solTypes.Account
}

func NewSolanaFacilitator(network string, url string, privateKeyHex string) (*SolanaFacilitator, error) {
	client := client.NewClient(url)

	privKey, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex private key: %w", err)
	}

	feePayer, err := solTypes.AccountFromBytes(privKey)
	if err != nil {
		return nil, fmt.Errorf("invalid private key format: %w", err)
	}

	return &SolanaFacilitator{
		scheme:   types.Exact,
		client:   client,
		feePayer: feePayer,
	}, nil
}

func (t *SolanaFacilitator) Verify(ctx context.Context, payload *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentVerifyResponse, error) {
	return nil, nil
}

func (t *SolanaFacilitator) Settle(ctx context.Context, payload *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentSettleResponse, error) {
	return nil, nil
}

func (t *SolanaFacilitator) Supported() []*types.SupportedKind {
	// Verify/Settle are not yet v2-compliant; gated from discovery so
	// clients do not pick this facilitator for Solana payments.
	return nil
}

// CaipFamily returns the CAIP-2 family pattern for Solana clusters.
func (t *SolanaFacilitator) CaipFamily() string { return "solana:*" }

// GetSigners returns the facilitator's fee-payer public key (base58).
func (t *SolanaFacilitator) GetSigners() []string {
	return []string{t.feePayer.PublicKey.String()}
}
