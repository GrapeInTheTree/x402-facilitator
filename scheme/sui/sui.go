package sui

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"golang.org/x/crypto/blake2b"
)

const (
	// Ed25519 signature scheme flag used by Sui's serialized signature format.
	SignatureSchemeEd25519 byte = 0x00

	intentScopeTransactionData byte = 0x00
	intentVersionV0            byte = 0x00
	intentAppIDSui             byte = 0x00

	// USDCType is the canonical native USDC type currently supported by Sui's
	// gasless stablecoin transfer flow.
	USDCType    = "0xdba34672e30cb065b1f93e3ab55318768fd6fef66c15942c9f7cb846e2f900e7::usdc::USDC"
	USDSUIType  = "0x44f838219cf67b058f3b37907b655f226153c18e33dfcd0da559a844fea9b1c1::usdsui::USDSUI"
	SUIUSDEType = "0x41d587e5336f1c86cad50d38a7136db99333bb9bda91cea4ba69115defeb1402::sui_usde::SUI_USDE"
	USDYType    = "0x960b531667636f39e85867775f52f6b1f220a058c4de786905bdf761e06a56bb::usdy::USDY"
	FDUSDType   = "0xf16e6b723f242ec745dfd7634ad072c42d5c1d9ac9d62a39c381303eaa57693a::fdusd::FDUSD"
	AUSDType    = "0x2053d08c1e2bd02791056171aab0fd12bd7cd7efad2ab8f6b9c8902f14df2ff2::ausd::AUSD"
	USDBType    = "0xe14726c336e81b32328e92afc37345d159f5b550b09fa92bd43640cfdd0a0cfd::usdb::USDB"
)

var (
	ErrEmptyTransaction = errors.New("empty_transaction")

	ErrEmptySignature       = errors.New("empty_signature")
	ErrInvalidSignature     = errors.New("invalid_signature")
	ErrUnsupportedSignature = errors.New("unsupported_signature_scheme")

	// DefaultGaslessStablecoinTypeList mirrors the Sui gasless stablecoin
	// allowlist from the public docs. The protocol configuration can change, so
	// facilitator deployments should treat this as a default policy snapshot.
	DefaultGaslessStablecoinTypeList = []string{
		USDCType,
		USDSUIType,
		SUIUSDEType,
		USDYType,
		FDUSDType,
		AUSDType,
		USDBType,
	}
	DefaultGaslessStablecoinTypes = defaultGaslessStablecoinTypes()
)

// Payload is the x402 Sui "exact" scheme payload. Transaction is the base64
// encoded Sui TransactionData bytes. Signature is the base64 encoded Sui
// serialized signature, containing the scheme flag, signature, and public key.
type Payload struct {
	Signature   string `json:"signature"`
	Transaction string `json:"transaction"`
}

// Signer can sign Sui TransactionData bytes and expose the corresponding Sui
// address.
type Signer interface {
	SignTransaction(txBytes []byte) (string, error)
	Address() string
}

type Ed25519Signer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

func NewPayload(signature string, txBytes []byte) (*Payload, error) {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return nil, ErrEmptySignature
	}
	if len(txBytes) == 0 {
		return nil, ErrEmptyTransaction
	}

	return &Payload{
		Signature:   signature,
		Transaction: base64.StdEncoding.EncodeToString(txBytes),
	}, nil
}

func NewSignedPayload(txBytes []byte, signer Signer) (*Payload, error) {
	if signer == nil {
		return nil, errors.New("nil_signer")
	}
	signature, err := signer.SignTransaction(txBytes)
	if err != nil {
		return nil, err
	}
	return NewPayload(signature, txBytes)
}

func ParsePayload(raw []byte) (*Payload, error) {
	var payload Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("invalid_payload: %w", err)
	}
	if strings.TrimSpace(payload.Signature) == "" {
		return nil, ErrEmptySignature
	}
	if strings.TrimSpace(payload.Transaction) == "" {
		return nil, ErrEmptyTransaction
	}
	return &payload, nil
}

func PayloadFromMap(value map[string]interface{}) (*Payload, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return ParsePayload(raw)
}

func (p *Payload) DecodeTransaction() ([]byte, error) {
	if p == nil || strings.TrimSpace(p.Transaction) == "" {
		return nil, ErrEmptyTransaction
	}
	txBytes, err := base64.StdEncoding.DecodeString(p.Transaction)
	if err != nil {
		return nil, fmt.Errorf("invalid_transaction_encoding: %w", err)
	}
	if len(txBytes) == 0 {
		return nil, ErrEmptyTransaction
	}
	return txBytes, nil
}

func NewEd25519SignerFromPrivateKey(privateKey []byte) (*Ed25519Signer, error) {
	var key ed25519.PrivateKey
	switch len(privateKey) {
	case ed25519.SeedSize:
		key = ed25519.NewKeyFromSeed(privateKey)
	case ed25519.PrivateKeySize:
		key = append(ed25519.PrivateKey(nil), privateKey...)
	default:
		return nil, fmt.Errorf("invalid_ed25519_private_key_length: %d", len(privateKey))
	}

	publicKey, ok := key.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("invalid_ed25519_public_key")
	}

	return &Ed25519Signer{
		privateKey: key,
		publicKey:  append(ed25519.PublicKey(nil), publicKey...),
	}, nil
}

func NewEd25519SignerFromHex(privateKeyHex string) (*Ed25519Signer, error) {
	privateKeyHex = strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x")
	privateKey, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, err
	}
	return NewEd25519SignerFromPrivateKey(privateKey)
}

func (s *Ed25519Signer) SignTransaction(txBytes []byte) (string, error) {
	if s == nil {
		return "", errors.New("nil_signer")
	}
	if len(txBytes) == 0 {
		return "", ErrEmptyTransaction
	}

	signature := ed25519.Sign(s.privateKey, TransactionIntentDigest(txBytes))
	serialized := make([]byte, 1+ed25519.SignatureSize+ed25519.PublicKeySize)
	serialized[0] = SignatureSchemeEd25519
	copy(serialized[1:], signature)
	copy(serialized[1+ed25519.SignatureSize:], s.publicKey)

	return base64.StdEncoding.EncodeToString(serialized), nil
}

func (s *Ed25519Signer) Address() string {
	if s == nil {
		return ""
	}
	return AddressFromPublicKey(SignatureSchemeEd25519, s.publicKey)
}

func VerifySignature(signature string, txBytes []byte) (string, error) {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return "", ErrEmptySignature
	}
	if len(txBytes) == 0 {
		return "", ErrEmptyTransaction
	}

	serialized, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidSignature, err)
	}
	if len(serialized) != 1+ed25519.SignatureSize+ed25519.PublicKeySize {
		return "", fmt.Errorf("%w: invalid_length", ErrInvalidSignature)
	}
	if serialized[0] != SignatureSchemeEd25519 {
		return "", ErrUnsupportedSignature
	}

	rawSig := serialized[1 : 1+ed25519.SignatureSize]
	publicKey := serialized[1+ed25519.SignatureSize:]
	if !ed25519.Verify(ed25519.PublicKey(publicKey), TransactionIntentDigest(txBytes), rawSig) {
		return "", ErrInvalidSignature
	}

	return AddressFromPublicKey(serialized[0], publicKey), nil
}

func TransactionIntentDigest(txBytes []byte) []byte {
	if len(txBytes) > math.MaxInt-3 {
		panic("transaction bytes too large")
	}
	intentMessage := make([]byte, 0, 3+len(txBytes))
	intentMessage = append(intentMessage, intentScopeTransactionData, intentVersionV0, intentAppIDSui)
	intentMessage = append(intentMessage, txBytes...)

	digest := blake2b.Sum256(intentMessage)
	return digest[:]
}

func AddressFromPublicKey(signatureScheme byte, publicKey []byte) string {
	if len(publicKey) > math.MaxInt-1 {
		panic("public key too large")
	}
	material := make([]byte, 0, 1+len(publicKey))
	material = append(material, signatureScheme)
	material = append(material, publicKey...)

	digest := blake2b.Sum256(material)
	return "0x" + hex.EncodeToString(digest[:])
}

func NormalizeType(asset string) string {
	return strings.ToLower(strings.TrimSpace(asset))
}

func IsDefaultGaslessStablecoinType(asset string) bool {
	_, ok := DefaultGaslessStablecoinTypes[NormalizeType(asset)]
	return ok
}

func defaultGaslessStablecoinTypes() map[string]struct{} {
	types := make(map[string]struct{}, len(DefaultGaslessStablecoinTypeList))
	for _, asset := range DefaultGaslessStablecoinTypeList {
		types[NormalizeType(asset)] = struct{}{}
	}
	return types
}
