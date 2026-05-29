package facilitator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	suischeme "github.com/gosuda/x402-facilitator/scheme/sui"
	"github.com/gosuda/x402-facilitator/types"
	"github.com/gosuda/x402-facilitator/utils"
)

var _ Facilitator = (*SuiFacilitator)(nil)

type SuiFacilitator struct {
	scheme              types.Scheme
	network             string
	client              *suiRPCClient
	gaslessStablecoins  map[string]struct{}
	gaslessStableAssets []string
}

func NewSuiFacilitator(network string, url string, privateKeyHex string) (*SuiFacilitator, error) {
	if !strings.HasPrefix(network, "sui:") {
		return nil, fmt.Errorf("unsupported Sui network %q", network)
	}
	networkInfo := suischeme.GetNetworkInfo(network)
	if networkInfo == nil {
		return nil, fmt.Errorf("unsupported Sui network %q", network)
	}
	dialCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url, err := selectSuiEndpoint(dialCtx, url, networkInfo.DefaultURLs)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Sui endpoints: %w", err)
	}

	assets := suischeme.GetGaslessStablecoinTypes(network)
	allowlist := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		allowlist[suischeme.NormalizeType(asset)] = struct{}{}
	}

	return &SuiFacilitator{
		scheme:              types.Exact,
		network:             network,
		client:              newSuiRPCClientWithEndpoints(url, networkInfo.DefaultURLs),
		gaslessStablecoins:  allowlist,
		gaslessStableAssets: assets,
	}, nil
}

func selectSuiEndpoint(ctx context.Context, priorityURL string, defaultURLs []string) (string, error) {
	candidates := utils.EndpointCandidates(priorityURL, defaultURLs)
	return utils.SelectEndpoint(ctx, candidates, func(ctx context.Context, endpoint string) error {
		return newSuiRPCClient(endpoint).ping(ctx)
	})
}

func (t *SuiFacilitator) Verify(ctx context.Context, payload *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentVerifyResponse, error) {
	if payload == nil || req == nil {
		return &types.PaymentVerifyResponse{
			IsValid:       false,
			InvalidReason: types.ErrInvalidPayloadFormat.Error(),
		}, nil
	}

	suiPayload, payer, err := parseAndVerifySuiPayload(payload.Payload)
	if err != nil {
		return &types.PaymentVerifyResponse{
			IsValid:        false,
			InvalidReason:  invalidPayloadReason(err),
			InvalidMessage: err.Error(),
		}, nil
	}

	if invalid := t.validatePaymentEnvelope(payload, req, payer); invalid != nil {
		return invalid, nil
	}

	reqAmount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok || reqAmount.Sign() <= 0 {
		return &types.PaymentVerifyResponse{
			IsValid:       false,
			InvalidReason: types.ErrAmountMismatch.Error(),
			Payer:         payer,
		}, nil
	}

	dryRun, err := t.client.dryRunTransactionBlock(ctx, suiPayload.Transaction)
	if err != nil {
		return nil, fmt.Errorf("dry run transaction failed: %w", err)
	}
	if !dryRun.success() {
		return &types.PaymentVerifyResponse{
			IsValid:        false,
			InvalidReason:  types.ErrTransactionFailed.Error(),
			InvalidMessage: dryRun.statusError(),
			Payer:          payer,
		}, nil
	}
	if !dryRun.gasless() {
		return &types.PaymentVerifyResponse{
			IsValid:        false,
			InvalidReason:  types.ErrInvalidTransaction.Error(),
			InvalidMessage: "transaction is not gasless",
			Payer:          payer,
		}, nil
	}

	received := dryRun.balanceDelta(req.PayTo, req.Asset)
	if received.Cmp(reqAmount) != 0 {
		return &types.PaymentVerifyResponse{
			IsValid:        false,
			InvalidReason:  types.ErrAmountMismatch.Error(),
			InvalidMessage: fmt.Sprintf("expected payTo balance delta %s, got %s", reqAmount.String(), received.String()),
			Payer:          payer,
		}, nil
	}

	return &types.PaymentVerifyResponse{
		IsValid: true,
		Payer:   payer,
	}, nil
}

func (t *SuiFacilitator) Settle(ctx context.Context, payload *types.PaymentPayload, req *types.PaymentRequirements) (*types.PaymentSettleResponse, error) {
	network := types.Network("")
	if req != nil {
		network = types.Network(req.Network)
	}

	verified, err := t.Verify(ctx, payload, req)
	if err != nil {
		return nil, err
	}
	if !verified.IsValid {
		return &types.PaymentSettleResponse{
			Success:      false,
			ErrorReason:  verified.InvalidReason,
			ErrorMessage: verified.InvalidMessage,
			Payer:        verified.Payer,
			Network:      network,
		}, nil
	}

	suiPayload, err := suischeme.PayloadFromMap(payload.Payload)
	if err != nil {
		return &types.PaymentSettleResponse{
			Success:      false,
			ErrorReason:  types.ErrInvalidPayloadFormat.Error(),
			ErrorMessage: err.Error(),
			Payer:        verified.Payer,
			Network:      network,
		}, nil
	}

	executed, err := t.client.executeTransactionBlock(ctx, suiPayload.Transaction, []string{suiPayload.Signature})
	if err != nil {
		return &types.PaymentSettleResponse{
			Success:      false,
			ErrorReason:  types.ErrTransactionFailed.Error(),
			ErrorMessage: err.Error(),
			Payer:        verified.Payer,
			Network:      network,
		}, nil
	}
	if !executed.success() {
		return &types.PaymentSettleResponse{
			Success:      false,
			ErrorReason:  types.ErrTransactionFailed.Error(),
			ErrorMessage: executed.statusError(),
			Payer:        verified.Payer,
			Transaction:  executed.Digest,
			Network:      network,
		}, nil
	}

	return &types.PaymentSettleResponse{
		Success:     true,
		Payer:       verified.Payer,
		Transaction: executed.Digest,
		Network:     network,
	}, nil
}

func (t *SuiFacilitator) Supported() *types.SupportedResponse {
	return &types.SupportedResponse{
		Kinds: []types.SupportedKind{{
			X402Version: int(types.X402VersionV2),
			Scheme:      string(t.scheme),
			Network:     t.network,
			Extra: map[string]interface{}{
				"assetTransferMethod": "sui-gasless-stablecoin-address-balance",
				"assets":              t.gaslessStableAssets,
				"networkId":           suischeme.GetNetworkID(t.network),
				"networkName":         suischeme.GetNetworkName(t.network),
			},
		}},
		Extensions: []string{},
		Signers:    map[string][]string{},
	}
}

func (t *SuiFacilitator) validatePaymentEnvelope(payload *types.PaymentPayload, req *types.PaymentRequirements, payer string) *types.PaymentVerifyResponse {
	if payload.X402Version != int(types.X402VersionV2) {
		return &types.PaymentVerifyResponse{
			IsValid:       false,
			InvalidReason: types.ErrInvalidPayloadFormat.Error(),
			Payer:         payer,
		}
	}
	if payload.Accepted.Scheme != string(t.scheme) || req.Scheme != string(t.scheme) {
		return &types.PaymentVerifyResponse{
			IsValid:       false,
			InvalidReason: types.ErrIncompatibleScheme.Error(),
			Payer:         payer,
		}
	}
	if payload.Accepted.Network != t.network || req.Network != t.network {
		return &types.PaymentVerifyResponse{
			IsValid:       false,
			InvalidReason: types.ErrNetworkMismatch.Error(),
			Payer:         payer,
		}
	}
	if !strings.EqualFold(strings.TrimSpace(payload.Accepted.Asset), strings.TrimSpace(req.Asset)) {
		return &types.PaymentVerifyResponse{
			IsValid:       false,
			InvalidReason: types.ErrTokenMismatch.Error(),
			Payer:         payer,
		}
	}
	if payload.Accepted.Amount != req.Amount {
		return &types.PaymentVerifyResponse{
			IsValid:       false,
			InvalidReason: types.ErrAmountMismatch.Error(),
			Payer:         payer,
		}
	}
	if normalizeSuiAddress(payload.Accepted.PayTo) != normalizeSuiAddress(req.PayTo) {
		return &types.PaymentVerifyResponse{
			IsValid:       false,
			InvalidReason: types.ErrRecipientMismatch.Error(),
			Payer:         payer,
		}
	}
	if !t.isGaslessStablecoin(req.Asset) {
		return &types.PaymentVerifyResponse{
			IsValid:       false,
			InvalidReason: types.ErrInvalidToken.Error(),
			Payer:         payer,
		}
	}
	return nil
}

func (t *SuiFacilitator) isGaslessStablecoin(asset string) bool {
	_, ok := t.gaslessStablecoins[suischeme.NormalizeType(asset)]
	return ok
}

func parseAndVerifySuiPayload(payload map[string]interface{}) (*suischeme.Payload, string, error) {
	suiPayload, err := suischeme.PayloadFromMap(payload)
	if err != nil {
		return nil, "", err
	}

	txBytes, err := suiPayload.DecodeTransaction()
	if err != nil {
		return nil, "", err
	}
	payer, err := suischeme.VerifySignature(suiPayload.Signature, txBytes)
	if err != nil {
		return nil, "", err
	}

	return suiPayload, payer, nil
}

func invalidPayloadReason(err error) string {
	if errors.Is(err, suischeme.ErrInvalidSignature) ||
		errors.Is(err, suischeme.ErrUnsupportedSignature) ||
		errors.Is(err, suischeme.ErrEmptySignature) {
		return types.ErrInvalidSignature.Error()
	}
	return types.ErrInvalidPayloadFormat.Error()
}

type suiRPCClient struct {
	mu         sync.RWMutex
	url        string
	endpoints  []string
	httpClient *http.Client
}

func newSuiRPCClient(url string) *suiRPCClient {
	return newSuiRPCClientWithEndpoints(url, []string{url})
}

func newSuiRPCClientWithEndpoints(url string, endpoints []string) *suiRPCClient {
	candidates := utils.EndpointCandidates(url, endpoints)
	if len(candidates) == 0 {
		candidates = []string{strings.TrimSpace(url)}
	}
	return &suiRPCClient{
		url:       candidates[0],
		endpoints: candidates,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *suiRPCClient) dryRunTransactionBlock(ctx context.Context, txBytesBase64 string) (*dryRunTransactionBlockResponse, error) {
	var result dryRunTransactionBlockResponse
	if err := c.call(ctx, "sui_dryRunTransactionBlock", []interface{}{txBytesBase64}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *suiRPCClient) executeTransactionBlock(ctx context.Context, txBytesBase64 string, signatures []string) (*executeTransactionBlockResponse, error) {
	options := map[string]bool{
		"showEffects":        true,
		"showBalanceChanges": true,
	}
	params := []interface{}{txBytesBase64, signatures, options, "WaitForLocalExecution"}

	var result executeTransactionBlockResponse
	if err := c.call(ctx, "sui_executeTransactionBlock", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *suiRPCClient) ping(ctx context.Context) error {
	var result string
	return c.call(ctx, "sui_getLatestCheckpointSequenceNumber", []interface{}{}, &result)
}

func (c *suiRPCClient) call(ctx context.Context, method string, params []interface{}, result interface{}) error {
	candidates := c.endpointCandidates()
	selected, err := utils.DoWithEndpoint(ctx, candidates, func(ctx context.Context, endpoint string) error {
		return c.callEndpoint(ctx, endpoint, method, params, result)
	})
	if err != nil {
		return err
	}
	c.setActiveEndpoint(selected)
	return nil
}

func (c *suiRPCClient) endpointCandidates() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return utils.EndpointCandidates(c.url, c.endpoints)
}

func (c *suiRPCClient) setActiveEndpoint(endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.url = endpoint
	c.endpoints = utils.EndpointCandidates(endpoint, c.endpoints)
}

func (c *suiRPCClient) callEndpoint(ctx context.Context, endpoint string, method string, params []interface{}, result interface{}) error {
	reqBody := suiRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("sui rpc http status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var rpcResp suiRPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return err
	}
	if rpcResp.Error != nil {
		return rpcResp.Error
	}
	if len(rpcResp.Result) == 0 {
		return errors.New("sui rpc missing result")
	}
	return json.Unmarshal(rpcResp.Result, result)
}

type suiRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type suiRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *suiRPCError    `json:"error,omitempty"`
}

type suiRPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *suiRPCError) Error() string {
	if e == nil {
		return ""
	}
	if len(e.Data) == 0 {
		return fmt.Sprintf("sui rpc error %d: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("sui rpc error %d: %s: %s", e.Code, e.Message, string(e.Data))
}

type dryRunTransactionBlockResponse struct {
	Input          suiTransactionBlockData `json:"input"`
	Effects        suiEffects              `json:"effects"`
	BalanceChanges []suiBalanceChange      `json:"balanceChanges"`
}

func (r *dryRunTransactionBlockResponse) success() bool {
	return strings.EqualFold(r.Effects.Status.Status, "success")
}

func (r *dryRunTransactionBlockResponse) statusError() string {
	if r.Effects.Status.Error != "" {
		return r.Effects.Status.Error
	}
	if r.Effects.Status.Status != "" {
		return r.Effects.Status.Status
	}
	return "dry run failed"
}

func (r *dryRunTransactionBlockResponse) gasless() bool {
	return len(r.Input.GasData.Payment) == 0 &&
		r.Input.GasData.Price == "0" &&
		r.Input.GasData.Budget == "0"
}

func (r *dryRunTransactionBlockResponse) balanceDelta(owner string, coinType string) *big.Int {
	total := new(big.Int)
	normalizedOwner := normalizeSuiAddress(owner)
	normalizedCoinType := suischeme.NormalizeType(coinType)

	for _, change := range r.BalanceChanges {
		if ownerAddress(change.Owner) != normalizedOwner {
			continue
		}
		if suischeme.NormalizeType(change.CoinType) != normalizedCoinType {
			continue
		}

		amount, ok := new(big.Int).SetString(change.Amount, 10)
		if !ok {
			continue
		}
		total.Add(total, amount)
	}

	return total
}

type executeTransactionBlockResponse struct {
	Digest  string     `json:"digest"`
	Effects suiEffects `json:"effects"`
}

func (r *executeTransactionBlockResponse) success() bool {
	return strings.EqualFold(r.Effects.Status.Status, "success")
}

func (r *executeTransactionBlockResponse) statusError() string {
	if r.Effects.Status.Error != "" {
		return r.Effects.Status.Error
	}
	if r.Effects.Status.Status != "" {
		return r.Effects.Status.Status
	}
	return "transaction failed"
}

type suiEffects struct {
	Status suiExecutionStatus `json:"status"`
}

type suiTransactionBlockData struct {
	GasData suiGasData `json:"gasData"`
}

type suiGasData struct {
	Payment []json.RawMessage `json:"payment"`
	Price   string            `json:"price"`
	Budget  string            `json:"budget"`
}

type suiExecutionStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type suiBalanceChange struct {
	Owner    json.RawMessage `json:"owner"`
	CoinType string          `json:"coinType"`
	Amount   string          `json:"amount"`
}

func ownerAddress(raw json.RawMessage) string {
	rawText := strings.TrimSpace(string(raw))
	if rawText == "" || rawText == "null" {
		return ""
	}

	var tagged map[string]json.RawMessage
	if err := json.Unmarshal(raw, &tagged); err == nil {
		for _, key := range []string{"AddressOwner", "ObjectOwner"} {
			if value, ok := tagged[key]; ok {
				var address string
				if err := json.Unmarshal(value, &address); err == nil {
					return normalizeSuiAddress(address)
				}
			}
		}
	}

	var address string
	if err := json.Unmarshal(raw, &address); err == nil {
		return normalizeSuiAddress(address)
	}

	return ""
}

func normalizeSuiAddress(address string) string {
	address = strings.ToLower(strings.TrimSpace(address))
	if address == "" || address == "0x" {
		return ""
	}
	address = strings.TrimPrefix(address, "0x")
	if len(address) < 64 {
		address = strings.Repeat("0", 64-len(address)) + address
	}
	return "0x" + address
}
