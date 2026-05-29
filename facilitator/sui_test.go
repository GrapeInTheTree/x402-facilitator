package facilitator

import (
	"encoding/json"
	"math/big"
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
	suiTestDigest  = "11111111111111111111111111111111"
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
		case "sui_dryRunTransactionBlock":
			require.Equal(t, []any{suiPayload.Transaction}, rpcReq.Params)
			writeSuiRPCResult(t, w, map[string]any{
				"input": suiGaslessInput(),
				"effects": map[string]any{
					"status": map[string]any{"status": "success"},
				},
				"objectChanges": []interface{}{},
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
				"digest": suiTestDigest,
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
	require.Equal(t, suiTestDigest, settled.Transaction)
	require.Equal(t, types.Network(suiTestNetwork), settled.Network)

	require.Equal(t, []string{
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

		writeSuiRPCResult(t, w, map[string]any{
			"input": suiGaslessInput(),
			"effects": map[string]any{
				"status": map[string]any{"status": "success"},
			},
			"objectChanges": []interface{}{},
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

		writeSuiRPCResult(t, w, map[string]any{
			"input": suiInputWithGasData(map[string]any{
				"payment": []map[string]any{{"objectId": "0x1"}},
				"price":   "1000",
				"budget":  "1000000",
			}),
			"effects": map[string]any{
				"status": map[string]any{"status": "success"},
			},
			"objectChanges": []interface{}{},
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
			"input": suiGaslessInput(),
			"effects": map[string]any{
				"status": map[string]any{"status": "success"},
			},
			"objectChanges": []interface{}{},
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

func TestSuiVerifyReturnsErrorOnDryRunRPCFailure(t *testing.T) {
	signer := newSuiTestSigner(t)
	suiPayload := newSuiTestPayload(t, signer)
	req := newSuiPaymentRequirements()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary endpoint failure", http.StatusBadGateway)
	}))
	defer server.Close()

	facilitator := &SuiFacilitator{
		scheme:  types.Exact,
		network: suiTestNetwork,
		client:  newSuiRPCClientWithEndpoints(server.URL, []string{server.URL}),
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
	require.Nil(t, verified)
	require.ErrorContains(t, err, "dry run transaction failed")
}

func TestSuiVerifyRejectsSignaturePayerSenderMismatch(t *testing.T) {
	sender := newSuiTestSigner(t)
	payer := newAltSuiTestSigner(t)
	req := newSuiPaymentRequirements()

	txBytes, err := suischeme.BuildGaslessStablecoinTransferTransaction(t.Context(), suischeme.GaslessStablecoinTransfer{
		Sender:    sender.Address(),
		Recipient: req.PayTo,
		Network:   req.Network,
		Asset:     req.Asset,
		Amount:    req.Amount,
	})
	require.NoError(t, err)
	suiPayload, err := suischeme.NewSignedPayload(txBytes, payer)
	require.NoError(t, err)

	facilitator := &SuiFacilitator{
		scheme:  types.Exact,
		network: suiTestNetwork,
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
	require.False(t, verified.IsValid)
	require.Equal(t, types.ErrInvalidSignature.Error(), verified.InvalidReason)
	require.Equal(t, payer.Address(), verified.Payer)
	require.Contains(t, verified.InvalidMessage, "does not match transaction sender")
}

func TestSuiVerifyRejectsAmountBelowMinimum(t *testing.T) {
	signer := newSuiTestSigner(t)
	req := newSuiPaymentRequirements()
	req.Amount = "9999"
	suiPayload := newSuiTestPayloadWithAmount(t, signer, req.Amount)

	facilitator := &SuiFacilitator{
		scheme:  types.Exact,
		network: suiTestNetwork,
		gaslessStablecoins: map[string]struct{}{
			suischeme.NormalizeType(suischeme.USDCType): {},
		},
		gaslessStableAssets: []string{suischeme.USDCType},
		minTransferAmounts: map[string]*big.Int{
			suischeme.NormalizeType(suischeme.USDCType): big.NewInt(10000),
		},
	}
	payment := &types.PaymentPayload{
		X402Version: int(types.X402VersionV2),
		Payload:     suiPayloadMap(t, suiPayload),
		Accepted:    *req,
	}

	verified, err := facilitator.Verify(t.Context(), payment, req)
	require.NoError(t, err)
	require.False(t, verified.IsValid)
	require.Equal(t, types.ErrAmountMismatch.Error(), verified.InvalidReason)
	require.Contains(t, verified.InvalidMessage, "below minimum")
}

func TestSuiSettleMapsAlreadyExecutedTransactionToSuccess(t *testing.T) {
	signer := newSuiTestSigner(t)
	suiPayload := newSuiTestPayload(t, signer)
	req := newSuiPaymentRequirements()
	txBytes, err := suiPayload.DecodeTransaction()
	require.NoError(t, err)
	txDigest, err := suischeme.TransactionDigest(txBytes)
	require.NoError(t, err)

	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpcReq suiRPCRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&rpcReq))
		methods = append(methods, rpcReq.Method)

		switch rpcReq.Method {
		case "sui_dryRunTransactionBlock":
			writeSuiDryRunSuccess(t, w, suiTestAmount)
		case "sui_executeTransactionBlock":
			writeSuiRPCError(t, w, -32000, "Transaction already executed")
		case "sui_getTransactionBlock":
			require.Equal(t, txDigest, rpcReq.Params[0])
			writeSuiRPCResult(t, w, map[string]any{
				"digest": txDigest,
				"effects": map[string]any{
					"status": map[string]any{"status": "success"},
				},
				"balanceChanges": []map[string]any{{
					"owner":    map[string]any{"AddressOwner": suiTestPayTo},
					"coinType": suischeme.USDCType,
					"amount":   suiTestAmount,
				}},
			})
		default:
			t.Fatalf("unexpected rpc method %s", rpcReq.Method)
		}
	}))
	defer server.Close()

	facilitator := &SuiFacilitator{
		scheme:  types.Exact,
		network: suiTestNetwork,
		client:  newSuiRPCClientWithEndpoints(server.URL, []string{server.URL}),
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

	settled, err := facilitator.Settle(t.Context(), payment, req)
	require.NoError(t, err)
	require.True(t, settled.Success)
	require.Equal(t, txDigest, settled.Transaction)
	require.Equal(t, []string{
		"sui_dryRunTransactionBlock",
		"sui_executeTransactionBlock",
		"sui_getTransactionBlock",
	}, methods)
}

func TestNewSuiFacilitatorWithOptionsUsesExternalAllowlistAndMinOverrides(t *testing.T) {
	customAsset := "0x2::custom_usdc::CUSTOM_USDC"

	facilitator, err := NewSuiFacilitatorWithOptions(suiTestNetwork, "http://127.0.0.1:1", "", SuiFacilitatorOptions{
		GaslessStablecoinTypes: []string{customAsset},
		MinTransferAmounts: map[string]string{
			customAsset: "123",
		},
	})
	require.NoError(t, err)
	require.True(t, facilitator.isGaslessStablecoin(customAsset))
	require.False(t, facilitator.isGaslessStablecoin(suischeme.USDCType))
	minAmount, ok := facilitator.minTransferAmount(customAsset)
	require.True(t, ok)
	require.Equal(t, "123", minAmount.String())
}

func TestSuiAddressHelpersRejectEmptyInputs(t *testing.T) {
	require.Empty(t, suischeme.NormalizeAddress(""))
	require.Empty(t, suischeme.NormalizeAddress("0x"))
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

func newAltSuiTestSigner(t *testing.T) *suischeme.Ed25519Signer {
	t.Helper()

	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	signer, err := suischeme.NewEd25519SignerFromPrivateKey(seed)
	require.NoError(t, err)
	return signer
}

func newSuiTestPayload(t *testing.T, signer suischeme.Signer) *suischeme.Payload {
	return newSuiTestPayloadWithAmount(t, signer, suiTestAmount)
}

func newSuiTestPayloadWithAmount(t *testing.T, signer suischeme.Signer, amount string) *suischeme.Payload {
	t.Helper()

	txBytes, err := suischeme.BuildGaslessStablecoinTransferTransaction(t.Context(), suischeme.GaslessStablecoinTransfer{
		Sender:    signer.Address(),
		Recipient: suiTestPayTo,
		Network:   suiTestNetwork,
		Asset:     suischeme.USDCType,
		Amount:    amount,
	})
	require.NoError(t, err)

	payload, err := suischeme.NewSignedPayload(txBytes, signer)
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

func suiGaslessInput() map[string]any {
	return suiInputWithGasData(map[string]any{
		"payment": []interface{}{},
		"price":   "0",
		"budget":  "0",
	})
}

func suiInputWithGasData(gasData map[string]any) map[string]any {
	return map[string]any{
		"transaction": map[string]any{
			"kind": "ProgrammableTransaction",
			"transactions": []map[string]any{
				suiMoveCallCommand("0x2", "balance", "send_funds", []string{suischeme.USDCType}),
			},
		},
		"gasData": gasData,
	}
}

func suiMoveCallCommand(pkg string, module string, function string, typeArguments []string) map[string]any {
	return map[string]any{
		"MoveCall": map[string]any{
			"package":        pkg,
			"module":         module,
			"function":       function,
			"type_arguments": typeArguments,
		},
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

func writeSuiDryRunSuccess(t *testing.T, w http.ResponseWriter, amount string) {
	t.Helper()

	writeSuiRPCResult(t, w, map[string]any{
		"input": suiGaslessInput(),
		"effects": map[string]any{
			"status": map[string]any{"status": "success"},
		},
		"objectChanges": []interface{}{},
		"balanceChanges": []map[string]any{{
			"owner":    map[string]any{"AddressOwner": suiTestPayTo},
			"coinType": suischeme.USDCType,
			"amount":   amount,
		}},
	})
}

func writeSuiRPCError(t *testing.T, w http.ResponseWriter, code int, message string) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}))
}
