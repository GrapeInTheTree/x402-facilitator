package types

import (
	"github.com/gosuda/x402-facilitator/internal/sdk"
)

// Specification: https://github.com/coinbase/x402/blob/main/specs/x402-specification.md
//
// The wire types below are intentionally type aliases of the upstream x402
// Go SDK v2 definitions. Aliasing (rather than re-declaring) keeps this
// facilitator byte-compatible with every x402 v2 client at compile time —
// any upstream field rename or addition surfaces as a local build error.

type (
	// PaymentPayload is the v2 body a client sends in the X-PAYMENT header.
	// Scheme and network live inside Accepted; Payload is a scheme-specific
	// map (e.g. an EIP-3009 authorization for eip155, an SPL transfer for
	// solana).
	PaymentPayload = sdk.PaymentPayload

	// PaymentRequirements describes what a resource server accepts. In v2
	// the price field is called Amount; the v1 name MaxAmountRequired is
	// gone.
	PaymentRequirements = sdk.PaymentRequirements

	// SupportedKind is a single (scheme, network) pair advertised by the
	// facilitator on /supported.
	SupportedKind = sdk.SupportedKind

	// PaymentVerifyResponse is the /verify response body.
	PaymentVerifyResponse = sdk.VerifyResponse

	// PaymentSettleResponse is the /settle response body. In v2 the
	// on-chain hash is Transaction (not TxHash) and Network is a CAIP-2
	// identifier (not a numeric chain ID string).
	PaymentSettleResponse = sdk.SettleResponse

	// Network is a CAIP-2 network string (e.g. "eip155:84532") with a
	// .Match helper for wildcard comparisons.
	Network = sdk.Network
)

// HTTP wrapper types are kept local because swag generates the OpenAPI
// schema from annotations in api/server.go and needs Go structs it can
// resolve at generation time; the upstream SDK models /verify and /settle
// bodies as opaque bytes at the network boundary.

// PaymentVerifyRequest is the body POSTed to /verify.
type PaymentVerifyRequest struct {
	PaymentPayload      PaymentPayload      `json:"paymentPayload"`
	PaymentRequirements PaymentRequirements `json:"paymentRequirements"`
}

// PaymentSettleRequest is the body POSTed to /settle.
type PaymentSettleRequest struct {
	PaymentPayload      PaymentPayload      `json:"paymentPayload"`
	PaymentRequirements PaymentRequirements `json:"paymentRequirements"`
}

// SupportedResponse is the /supported response body. Signers groups
// fee-paying addresses by CAIP-2 family (e.g. "eip155:*") so clients can
// see every sender address this facilitator may broadcast from.
type SupportedResponse struct {
	Kinds      []SupportedKind     `json:"kinds"`
	Extensions []string            `json:"extensions"`
	Signers    map[string][]string `json:"signers"`
}
