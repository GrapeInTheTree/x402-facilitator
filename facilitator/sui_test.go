package facilitator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	suischeme "github.com/gosuda/x402-facilitator/scheme/sui"
	"github.com/gosuda/x402-facilitator/types"
	"github.com/stretchr/testify/require"
)

const (
	suiTestNetwork = "sui:mainnet"
	suiTestAmount  = "1000000"
	suiTestPayTo   = "0xabc"
)

func TestSuiVerifyAndSettle(t *testing.T) {
	signer := newSuiTestSigner(t)
	suiPayload := newSuiTestPayload(t, signer)
	payloadMap := suiPayloadMap(t, suiPayload)
	req := newSuiPaymentRequirements()

	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpcReq suiRPCRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&rpcReq))
		methods = append(methods, rpcReq.Method)

		switch rpcReq.Method {
		case "sui_getLatestCheckpointSequenceNumber":
			writeSuiRPCResult(t, w, "1")
		case "sui_dryRunTransactionBlock":
			require.Equal(t, []any{suiPayload.Transaction}, rpcReq.Params)
			writeSuiRPCResult(t, w, map[string]any{
				"input": map[string]any{
					"gasData": map[string]any{
						"payment": []interface{}{},
						"price":   "0",
						"budget":  "0",
					},
				},
				"effects": map[string]any{
					"status": map[string]any{"status": "success"},
				},
				"balanceChanges": []map[string]any{{
					"owner":    map[string]any{"AddressOwner": "0x0000000000000000000000000000000000000000000000000000000000000abc"},
					"coinType": suischeme.USDCType,
					"amount":   suiTestAmount,
				}},
			})
		case "sui_executeTransactionBlock":
			require.Len(t, rpcReq.Params, 4)
			require.Equal(t, suiPayload.Transaction, rpcReq.Params[0])
			require.Equal(t, []interface{}{suiPayload.Signature}, rpcReq.Params[1])
			require.Equal(t, "WaitForLocalExecution", rpcReq.Params[3])
			writeSuiRPCResult(t, w, map[string]any{
				"digest": "6Yw1QvL4BAGFakeDigestForTestOnly",
				"effects": map[string]any{
					"status": map[string]any{"status": "success"},
				},
			})
		default:
			t.Fatalf("unexpected rpc method %s", rpcReq.Method)
		}
	}))
	defer server.Close()

	facilitator, err := NewSuiFacilitator(suiTestNetwork, server.URL, "")
	require.NoError(t, err)

	payment := &types.PaymentPayload{
		X402Version: int(types.X402VersionV2),
		Payload:     payloadMap,
		Accepted:    *req,
	}

	verified, err := facilitator.Verify(t.Context(), payment, req)
	require.NoError(t, err)
	require.True(t, verified.IsValid)
	require.Equal(t, signer.Address(), verified.Payer)

	settled, err := facilitator.Settle(t.Context(), payment, req)
	require.NoError(t, err)
	require.True(t, settled.Success)
	require.Equal(t, signer.Address(), settled.Payer)
	require.Equal(t, "6Yw1QvL4BAGFakeDigestForTestOnly", settled.Transaction)
	require.Equal(t, types.Network(suiTestNetwork), settled.Network)

	require.Equal(t, []string{
		"sui_getLatestCheckpointSequenceNumber",
		"sui_dryRunTransactionBlock",
		"sui_dryRunTransactionBlock",
		"sui_executeTransactionBlock",
	}, methods)
}

func TestSuiVerifyRejectsAmountMismatch(t *testing.T) {
	signer := newSuiTestSigner(t)
	suiPayload := newSuiTestPayload(t, signer)
	req := newSuiPaymentRequirements()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpcReq suiRPCRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&rpcReq))
		if rpcReq.Method == "sui_getLatestCheckpointSequenceNumber" {
			writeSuiRPCResult(t, w, "1")
			return
		}

		writeSuiRPCResult(t, w, map[string]any{
			"input": map[string]any{
				"gasData": map[string]any{
					"payment": []interface{}{},
					"price":   "0",
					"budget":  "0",
				},
			},
			"effects": map[string]any{
				"status": map[string]any{"status": "success"},
			},
			"balanceChanges": []map[string]any{{
				"owner":    map[string]any{"AddressOwner": suiTestPayTo},
				"coinType": suischeme.USDCType,
				"amount":   "999999",
			}},
		})
	}))
	defer server.Close()

	facilitator, err := NewSuiFacilitator(suiTestNetwork, server.URL, "")
	require.NoError(t, err)

	payment := &types.PaymentPayload{
		X402Version: int(types.X402VersionV2),
		Payload:     suiPayloadMap(t, suiPayload),
		Accepted:    *req,
	}

	verified, err := facilitator.Verify(t.Context(), payment, req)
	require.NoError(t, err)
	require.False(t, verified.IsValid)
	require.Equal(t, types.ErrAmountMismatch.Error(), verified.InvalidReason)
	require.Equal(t, signer.Address(), verified.Payer)
}

func TestSuiVerifyRejectsGasPaidTransaction(t *testing.T) {
	signer := newSuiTestSigner(t)
	suiPayload := newSuiTestPayload(t, signer)
	req := newSuiPaymentRequirements()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpcReq suiRPCRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&rpcReq))
		if rpcReq.Method == "sui_getLatestCheckpointSequenceNumber" {
			writeSuiRPCResult(t, w, "1")
			return
		}

		writeSuiRPCResult(t, w, map[string]any{
			"input": map[string]any{
				"gasData": map[string]any{
					"payment": []map[string]any{{"objectId": "0x1"}},
					"price":   "1000",
					"budget":  "1000000",
				},
			},
			"effects": map[string]any{
				"status": map[string]any{"status": "success"},
			},
			"balanceChanges": []map[string]any{{
				"owner":    map[string]any{"AddressOwner": suiTestPayTo},
				"coinType": suischeme.USDCType,
				"amount":   suiTestAmount,
			}},
		})
	}))
	defer server.Close()

	facilitator, err := NewSuiFacilitator(suiTestNetwork, server.URL, "")
	require.NoError(t, err)

	payment := &types.PaymentPayload{
		X402Version: int(types.X402VersionV2),
		Payload:     suiPayloadMap(t, suiPayload),
		Accepted:    *req,
	}

	verified, err := facilitator.Verify(t.Context(), payment, req)
	require.NoError(t, err)
	require.False(t, verified.IsValid)
	require.Equal(t, types.ErrInvalidTransaction.Error(), verified.InvalidReason)
	require.Equal(t, signer.Address(), verified.Payer)
}

func TestSelectSuiEndpointFallsBackFromUserEndpoint(t *testing.T) {
	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusBadGateway)
	}))
	defer badServer.Close()

	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpcReq suiRPCRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&rpcReq))
		require.Equal(t, "sui_getLatestCheckpointSequenceNumber", rpcReq.Method)
		writeSuiRPCResult(t, w, "1")
	}))
	defer goodServer.Close()

	selected, err := selectSuiEndpoint(t.Context(), badServer.URL, []string{goodServer.URL})
	require.NoError(t, err)
	require.Equal(t, goodServer.URL, selected)
}

func TestSuiVerifyFallsBackPerRequest(t *testing.T) {
	signer := newSuiTestSigner(t)
	suiPayload := newSuiTestPayload(t, signer)
	req := newSuiPaymentRequirements()

	badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary endpoint failure", http.StatusBadGateway)
	}))
	defer badServer.Close()

	goodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpcReq suiRPCRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&rpcReq))
		require.Equal(t, "sui_dryRunTransactionBlock", rpcReq.Method)
		writeSuiRPCResult(t, w, map[string]any{
			"input": map[string]any{
				"gasData": map[string]any{
					"payment": []interface{}{},
					"price":   "0",
					"budget":  "0",
				},
			},
			"effects": map[string]any{
				"status": map[string]any{"status": "success"},
			},
			"balanceChanges": []map[string]any{{
				"owner":    map[string]any{"AddressOwner": suiTestPayTo},
				"coinType": suischeme.USDCType,
				"amount":   suiTestAmount,
			}},
		})
	}))
	defer goodServer.Close()

	facilitator := &SuiFacilitator{
		scheme:  types.Exact,
		network: suiTestNetwork,
		client:  newSuiRPCClientWithEndpoints(badServer.URL, []string{badServer.URL, goodServer.URL}),
		gaslessStablecoins: map[string]struct{}{
			suischeme.NormalizeType(suischeme.USDCType): {},
		},
		gaslessStableAssets: []string{suischeme.USDCType},
	}
	payment := &types.PaymentPayload{
		X402Version: int(types.X402VersionV2),
		Payload:     suiPayloadMap(t, suiPayload),
		Accepted:    *req,
	}

	verified, err := facilitator.Verify(t.Context(), payment, req)
	require.NoError(t, err)
	require.True(t, verified.IsValid)
	require.Equal(t, goodServer.URL, facilitator.client.url)
}

func newSuiTestSigner(t *testing.T) *suischeme.Ed25519Signer {
	t.Helper()

	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(255 - i)
	}
	signer, err := suischeme.NewEd25519SignerFromPrivateKey(seed)
	require.NoError(t, err)
	return signer
}

func newSuiTestPayload(t *testing.T, signer suischeme.Signer) *suischeme.Payload {
	t.Helper()

	payload, err := suischeme.NewSignedPayload([]byte("sui transaction data"), signer)
	require.NoError(t, err)
	return payload
}

func newSuiPaymentRequirements() *types.PaymentRequirements {
	return &types.PaymentRequirements{
		Scheme:  string(types.Exact),
		Network: suiTestNetwork,
		Asset:   suischeme.USDCType,
		Amount:  suiTestAmount,
		PayTo:   suiTestPayTo,
	}
}

func suiPayloadMap(t *testing.T, payload *suischeme.Payload) map[string]any {
	t.Helper()

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	var payloadMap map[string]any
	require.NoError(t, json.Unmarshal(raw, &payloadMap))
	return payloadMap
}

func writeSuiRPCResult(t *testing.T, w http.ResponseWriter, result interface{}) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"result":  result,
	}))
}
