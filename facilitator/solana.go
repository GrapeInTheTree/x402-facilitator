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

// Supported advertises this facilitator on /supported. Solana Verify and
// Settle are still stubs; the SupportedResponse carried here is a
// placeholder that a follow-up commit gates to nil until a real
// implementation lands.
func (t *SolanaFacilitator) Supported() *types.SupportedResponse {
	return &types.SupportedResponse{
		Kinds: []types.SupportedKind{{
			X402Version: int(types.X402VersionV2),
			Scheme:      string(t.scheme),
			Network:     "solana:*",
		}},
		Extensions: []string{},
		Signers: map[string][]string{
			"solana:*": {t.feePayer.PublicKey.String()},
		},
	}
}
