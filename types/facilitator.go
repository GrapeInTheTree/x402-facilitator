// Package types declares the wire-level structures exchanged between
// facilitator clients, resource servers, and this facilitator over HTTP.
//
// The structs below mirror the x402 v2 specification field-for-field. They
// are intentionally redeclared here (rather than aliased from
// github.com/gosuda/x402-facilitator/internal/sdk) because swaggo/swag
// cannot resolve a Go type alias whose target has the same short name in a
// different package and aborts `make generate-api` with a false-positive
// recursion error.
//
// Drift against the upstream SDK is guarded by TestV2WireCompatibility in
// types/facilitator_test.go: it round-trips every struct declared here
// through the corresponding internal/sdk re-export and asserts JSON
// equality, so any upstream field rename surfaces as a local test failure.
//
// Specification: https://github.com/coinbase/x402/blob/main/specs/x402-specification.md
package types

// Network is a CAIP-2 network identifier (e.g. "eip155:84532" for Base
// Sepolia). It is a named string so handlers can distinguish a network
// from an arbitrary string in signatures.
type Network string

// PaymentRequirements describes what a resource server accepts for a
// given endpoint. It is the 402 Payment Required response body.
type PaymentRequirements struct {
	// Scheme of the payment protocol to use (e.g. "exact").
	Scheme string `json:"scheme"`
	// Network in CAIP-2 format (e.g. "eip155:84532").
	Network string `json:"network"`
	// Asset is the token contract address (EVM) or mint (SVM) being paid.
	Asset string `json:"asset"`
	// Amount required to pay for the resource in atomic units, as a
	// decimal string. This replaces the v1 field "maxAmountRequired".
	Amount string `json:"amount"`
	// PayTo is the address that receives the payment.
	PayTo string `json:"payTo"`
	// MaxTimeoutSeconds is the maximum time the resource server will
	// wait for the payment to be facilitated.
	MaxTimeoutSeconds int `json:"maxTimeoutSeconds"`
	// Extra carries scheme-specific metadata (e.g. the EIP-712 domain
	// name and version for EIP-3009).
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// PaymentPayload is the body a client sends in the X-PAYMENT header. In
// v2 the scheme and network fields live inside Accepted, not at the top
// level, and Payload is a decoded map rather than raw JSON.
type PaymentPayload struct {
	// X402Version is the protocol version (always 2 for v2 payloads).
	X402Version int `json:"x402Version"`
	// Payload is the scheme-specific payment material (e.g. an EIP-3009
	// authorization + signature). Its shape depends on Accepted.Scheme.
	Payload map[string]interface{} `json:"payload"`
	// Accepted echoes the PaymentRequirements the client is paying
	// against. Facilitators read scheme/network from here.
	Accepted PaymentRequirements `json:"accepted"`
	// Resource describes the resource being paid for. Optional.
	Resource *ResourceInfo `json:"resource,omitempty"`
	// Extensions carries protocol extension data. Optional.
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// ResourceInfo describes the resource being paid for. In v1 these fields
// lived on PaymentRequirements; v2 moved them up onto PaymentPayload.
type ResourceInfo struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// PaymentVerifyRequest is the request body sent to POST /verify.
type PaymentVerifyRequest struct {
	PaymentPayload      PaymentPayload      `json:"paymentPayload"`
	PaymentRequirements PaymentRequirements `json:"paymentRequirements"`
}

// PaymentVerifyResponse is the response returned from POST /verify.
type PaymentVerifyResponse struct {
	// IsValid reports whether the payment payload is valid.
	IsValid bool `json:"isValid"`
	// InvalidReason is a short machine-readable code (e.g.
	// "insufficient_balance") when IsValid is false.
	InvalidReason string `json:"invalidReason,omitempty"`
	// InvalidMessage is a human-readable explanation when IsValid is
	// false. Clients may surface this to end users.
	InvalidMessage string `json:"invalidMessage,omitempty"`
	// Payer is the address that signed the payment, when recoverable.
	Payer string `json:"payer,omitempty"`
}

// PaymentSettleRequest is the request body sent to POST /settle.
type PaymentSettleRequest struct {
	PaymentPayload      PaymentPayload      `json:"paymentPayload"`
	PaymentRequirements PaymentRequirements `json:"paymentRequirements"`
}

// PaymentSettleResponse is the response returned from POST /settle.
type PaymentSettleResponse struct {
	// Success reports whether settlement broadcast was accepted.
	Success bool `json:"success"`
	// ErrorReason is a short machine-readable code when Success is false.
	ErrorReason string `json:"errorReason,omitempty"`
	// ErrorMessage is a human-readable explanation when Success is false.
	ErrorMessage string `json:"errorMessage,omitempty"`
	// Payer is the address that signed the payment, when recoverable.
	Payer string `json:"payer,omitempty"`
	// Transaction is the on-chain transaction hash of the settled payment.
	// This replaces the v1 field "txHash".
	Transaction string `json:"transaction"`
	// Network is the CAIP-2 network the transaction was submitted to.
	// This replaces the v1 field "networkId".
	Network Network `json:"network"`
}

// SupportedKind is one (scheme, network) pair this facilitator accepts.
type SupportedKind struct {
	// X402Version is the protocol version supported for this pair.
	X402Version int `json:"x402Version"`
	// Scheme of the payment protocol (e.g. "exact").
	Scheme string `json:"scheme"`
	// Network in CAIP-2 format.
	Network string `json:"network"`
	// Extra carries scheme-specific metadata (e.g. the SVM fee payer).
	Extra map[string]interface{} `json:"extra,omitempty"`
}

// SupportedResponse is the body returned from GET /supported. It advertises
// every (scheme, network) pair the facilitator accepts alongside the
// protocol extensions it understands and the addresses it may broadcast
// settlement transactions from, grouped by CAIP-2 family pattern.
type SupportedResponse struct {
	// Kinds lists every accepted (scheme, network) pair.
	Kinds []SupportedKind `json:"kinds"`
	// Extensions lists the x402 protocol extensions this facilitator
	// honours. Empty when no extensions are supported.
	Extensions []string `json:"extensions"`
	// Signers maps a CAIP-2 family pattern (e.g. "eip155:*") to the
	// addresses this facilitator may use as the sender of settlement
	// transactions on that family. Clients use this to allowlist or
	// audit the facilitator's on-chain footprint.
	Signers map[string][]string `json:"signers"`
}
