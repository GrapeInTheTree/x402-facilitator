package types_test

import (
	"encoding/json"
	"testing"

	"github.com/gosuda/x402-facilitator/internal/sdk"
	"github.com/gosuda/x402-facilitator/types"
	"github.com/stretchr/testify/require"
)

// TestV2WireCompatibility round-trips every locally declared wire struct in
// types/facilitator.go through the corresponding x402 v2 SDK type (via
// internal/sdk) and asserts JSON equality. If the upstream SDK ever renames
// a field or changes a JSON tag and go.mod is bumped, this test fails and
// forces an explicit update of types/facilitator.go rather than letting the
// public HTTP contract of /verify, /settle, and /supported drift silently.
//
// The local structs live in types/facilitator.go (rather than as aliases of
// the SDK) because swaggo/swag cannot resolve an alias chain whose target
// has the same short name in a different package; this test is the contract
// that keeps those parallel declarations honest.
func TestV2WireCompatibility(t *testing.T) {
	t.Run("PaymentRequirements", func(t *testing.T) {
		local := types.PaymentRequirements{
			Scheme:            "exact",
			Network:           "eip155:84532",
			Asset:             "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
			Amount:            "1000",
			PayTo:             "0x1234567890123456789012345678901234567890",
			MaxTimeoutSeconds: 300,
			Extra: map[string]interface{}{
				"name":    "USDC",
				"version": "2",
			},
		}
		assertWireCompat[sdk.PaymentRequirements](t, local)
	})

	t.Run("PaymentPayload", func(t *testing.T) {
		local := types.PaymentPayload{
			X402Version: int(types.X402VersionV2),
			Payload: map[string]interface{}{
				"signature": "0xabc",
				"authorization": map[string]interface{}{
					"from":        "0x1234567890123456789012345678901234567890",
					"to":          "0x2345678901234567890123456789012345678901",
					"value":       "1000",
					"validAfter":  "0",
					"validBefore": "9999999999",
					"nonce":       "0x0000000000000000000000000000000000000000000000000000000000000001",
				},
			},
			Accepted: types.PaymentRequirements{
				Scheme:            "exact",
				Network:           "eip155:84532",
				Asset:             "0x036CbD53842c5426634e7929541eC2318f3dCF7e",
				Amount:            "1000",
				PayTo:             "0x2345678901234567890123456789012345678901",
				MaxTimeoutSeconds: 300,
			},
			Resource: &types.ResourceInfo{
				URL:         "https://example.com/api/data",
				Description: "example resource",
				MimeType:    "application/json",
			},
			Extensions: map[string]interface{}{
				"eip2612GasSponsoring": map[string]interface{}{
					"sponsor": "0x3456789012345678901234567890123456789012",
				},
			},
		}
		assertWireCompat[sdk.PaymentPayload](t, local)
	})

	t.Run("SupportedKind", func(t *testing.T) {
		local := types.SupportedKind{
			X402Version: int(types.X402VersionV2),
			Scheme:      "exact",
			Network:     "eip155:84532",
			Extra: map[string]interface{}{
				"feePayer": "0x1234567890123456789012345678901234567890",
			},
		}
		assertWireCompat[sdk.SupportedKind](t, local)
	})

	t.Run("SupportedResponse", func(t *testing.T) {
		local := types.SupportedResponse{
			Kinds: []types.SupportedKind{{
				X402Version: int(types.X402VersionV2),
				Scheme:      "exact",
				Network:     "eip155:84532",
			}},
			Extensions: []string{"eip2612GasSponsoring"},
			Signers: map[string][]string{
				"eip155:*": {"0x1234567890123456789012345678901234567890"},
			},
		}
		assertWireCompat[sdk.SupportedResponse](t, local)
	})

	t.Run("PaymentVerifyResponse", func(t *testing.T) {
		local := types.PaymentVerifyResponse{
			IsValid:        false,
			InvalidReason:  "insufficient_balance",
			InvalidMessage: "payer balance is below the requested amount",
			Payer:          "0x1234567890123456789012345678901234567890",
		}
		assertWireCompat[sdk.VerifyResponse](t, local)
	})

	t.Run("PaymentSettleResponse", func(t *testing.T) {
		local := types.PaymentSettleResponse{
			Success:     true,
			Payer:       "0x1234567890123456789012345678901234567890",
			Transaction: "0xabcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
			Network:     types.Network("eip155:84532"),
		}
		assertWireCompat[sdk.SettleResponse](t, local)
	})
}

// assertWireCompat marshals local to JSON, unmarshals the bytes into the
// upstream SDK type SDK, re-marshals, and asserts that the two JSON blobs
// are equal. Any field rename, tag change, or added required field between
// the local struct and the upstream SDK type surfaces as a mismatch here.
func assertWireCompat[SDK any](t *testing.T, local any) {
	t.Helper()

	localJSON, err := json.Marshal(local)
	require.NoError(t, err, "marshal local value")

	var remote SDK
	require.NoError(t, json.Unmarshal(localJSON, &remote), "unmarshal local JSON into SDK type")

	remoteJSON, err := json.Marshal(remote)
	require.NoError(t, err, "marshal SDK value")

	require.JSONEq(t, string(localJSON), string(remoteJSON),
		"wire drift between local %T and its x402 SDK counterpart", local)
}
