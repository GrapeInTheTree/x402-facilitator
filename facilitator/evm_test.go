package facilitator

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gosuda/x402-facilitator/scheme/evm"
	"github.com/gosuda/x402-facilitator/types"
	"github.com/stretchr/testify/require"
)

const (
	PrivateKey = ""
	Network    = "eip155:84532"
	Token      = "USDC"
)

func TestEVMVerify(t *testing.T) {
	facilitator, err := NewEVMFacilitator(Network, "", PrivateKey)
	require.NoError(t, err)

	privKey, err := hex.DecodeString("")
	require.NoError(t, err)
	evmPayload, err := evm.NewEVMPayload(Network, Token,
		"", "", "10000", evm.NewRawPrivateSigner(privKey))
	require.NoError(t, err)

	evmPayloadJson, err := json.Marshal(evmPayload)
	require.NoError(t, err)
	var payloadMap map[string]interface{}
	require.NoError(t, json.Unmarshal(evmPayloadJson, &payloadMap))

	req := &types.PaymentRequirements{
		Scheme:  string(types.Exact),
		Network: Network,
		Asset:   Token,
		Amount:  "10000",
	}
	payload := &types.PaymentPayload{
		X402Version: int(types.X402VersionV2),
		Payload:     payloadMap,
		Accepted:    *req,
	}

	res, err := facilitator.Verify(t.Context(), payload, req)
	require.NoError(t, err)
	jsonRes, err := json.MarshalIndent(res, "", "\t")
	require.NoError(t, err)
	fmt.Println(string(jsonRes))
}

func TestEVMSettle(t *testing.T) {
	facilitator, err := NewEVMFacilitator(Network, "", PrivateKey)
	require.NoError(t, err)

	privKey, err := hex.DecodeString("")
	require.NoError(t, err)
	evmPayload, err := evm.NewEVMPayload(Network, Token,
		"", "", "10000", evm.NewRawPrivateSigner(privKey))
	require.NoError(t, err)
	evmPayloadJson, err := json.Marshal(evmPayload)
	require.NoError(t, err)
	var payloadMap map[string]interface{}
	require.NoError(t, json.Unmarshal(evmPayloadJson, &payloadMap))

	req := &types.PaymentRequirements{
		Scheme:  string(types.Exact),
		Network: Network,
		Asset:   Token,
		Amount:  "10000",
	}
	payload := &types.PaymentPayload{
		X402Version: int(types.X402VersionV2),
		Payload:     payloadMap,
		Accepted:    *req,
	}

	res, err := facilitator.Settle(t.Context(), payload, req)
	require.NoError(t, err)

	jsonRes, err := json.MarshalIndent(res, "", "\t")
	require.NoError(t, err)
	fmt.Println(string(jsonRes))
}

func TestPayloadDetection(t *testing.T) {
	t.Run("detects EIP3009 payload", func(t *testing.T) {
		eip3009Json := []byte(`{"signature":"0xabc","authorization":{"from":"0x1234"}}`)
		require.False(t, evm.IsPermit2PayloadJSON(eip3009Json))
	})

	t.Run("detects Permit2 payload", func(t *testing.T) {
		permit2Json := []byte(`{"signature":"0xabc","permit2Authorization":{"from":"0x1234"}}`)
		require.True(t, evm.IsPermit2PayloadJSON(permit2Json))
	})

	t.Run("returns false for invalid JSON", func(t *testing.T) {
		require.False(t, evm.IsPermit2PayloadJSON([]byte(`{invalid`)))
	})

	t.Run("returns false for empty payload", func(t *testing.T) {
		require.False(t, evm.IsPermit2PayloadJSON([]byte(`{}`)))
	})
}
