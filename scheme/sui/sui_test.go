package sui_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/gosuda/x402-facilitator/scheme/sui"
	"github.com/stretchr/testify/require"
)

func TestEd25519SignerSignsAndVerifiesTransaction(t *testing.T) {
	signer := newTestSigner(t)
	txBytes := []byte("sui transaction data")

	payload, err := sui.NewSignedPayload(txBytes, signer)
	require.NoError(t, err)
	require.NotEmpty(t, payload.Signature)
	require.NotEmpty(t, payload.Transaction)

	decodedTx, err := payload.DecodeTransaction()
	require.NoError(t, err)
	require.Equal(t, txBytes, decodedTx)

	payer, err := sui.VerifySignature(payload.Signature, decodedTx)
	require.NoError(t, err)
	require.Equal(t, signer.Address(), payer)
}

func TestPayloadJSONRoundTrip(t *testing.T) {
	signer := newTestSigner(t)
	payload, err := sui.NewSignedPayload([]byte{0x01, 0x02, 0x03}, signer)
	require.NoError(t, err)

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	parsed, err := sui.ParsePayload(raw)
	require.NoError(t, err)
	require.Equal(t, payload.Signature, parsed.Signature)
	require.Equal(t, payload.Transaction, parsed.Transaction)
}

func TestVerifySignatureRejectsUnsupportedScheme(t *testing.T) {
	signer := newTestSigner(t)
	txBytes := []byte("sui transaction data")
	payload, err := sui.NewSignedPayload(txBytes, signer)
	require.NoError(t, err)

	serialized, err := base64.StdEncoding.DecodeString(payload.Signature)
	require.NoError(t, err)
	serialized[0] = 0x01

	_, err = sui.VerifySignature(base64.StdEncoding.EncodeToString(serialized), txBytes)
	require.ErrorIs(t, err, sui.ErrUnsupportedSignature)
}

func newTestSigner(t *testing.T) *sui.Ed25519Signer {
	t.Helper()

	seed := make([]byte, 32)
	for i := range seed {
		seed[i] = byte(i + 1)
	}

	signer, err := sui.NewEd25519SignerFromPrivateKey(seed)
	require.NoError(t, err)
	return signer
}
