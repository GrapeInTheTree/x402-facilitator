package sui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	bcs "github.com/iotaledger/bcs-go"
	suigraphql "github.com/open-move/sui-go-sdk/graphql"
	suitx "github.com/open-move/sui-go-sdk/transaction"
	suitypes "github.com/open-move/sui-go-sdk/types"
	suitypetag "github.com/open-move/sui-go-sdk/typetag"
	suiutils "github.com/open-move/sui-go-sdk/utils"
	"golang.org/x/crypto/blake2b"
)

const (
	TransactionKindProgrammable = "ProgrammableTransaction"

	transactionDataDigestTag         = "TransactionData::"
	gaslessStablecoinSendFundsTarget = "0x2::balance::send_funds"

	CommandKindMoveCall        = "MoveCall"
	CommandKindTransferObjects = "TransferObjects"
	CommandKindSplitCoins      = "SplitCoins"
	CommandKindMergeCoins      = "MergeCoins"
	CommandKindPublish         = "Publish"
	CommandKindMakeMoveVec     = "MakeMoveVec"
	CommandKindUpgrade         = "Upgrade"
)

type TransactionBlockData = suigraphql.TransactionDataContent
type TransactionEffects = suigraphql.TransactionEffectsResult
type TransactionExecutionStatus = suigraphql.StatusResult
type BalanceChange = suigraphql.BalanceChangeResult
type ObjectChange = suigraphql.ObjectChangeResult
type ObjectOwnerResult = suigraphql.ObjectOwnerResult
type ObjectRefResult = suigraphql.ObjectRefResult
type GasData = suigraphql.GasData
type TransactionKind = suigraphql.TransactionKindContent
type MoveCallCommand = suitx.MoveCall
type ExecuteTransactionBlock = suigraphql.TransactionResult

type GaslessStablecoinTransfer struct {
	Sender    string
	Recipient string
	Network   string
	Asset     string
	Amount    string
}

func ParseTransactionBlockData(data []byte) (*TransactionBlockData, error) {
	var transaction TransactionBlockData
	if err := json.Unmarshal(data, &transaction); err != nil {
		return nil, err
	}
	return &transaction, nil
}

type DryRunTransactionBlock struct {
	Input          TransactionBlockData `json:"input"`
	Effects        TransactionEffects   `json:"effects"`
	ObjectChanges  []ObjectChange       `json:"objectChanges,omitempty"`
	BalanceChanges []BalanceChange      `json:"balanceChanges,omitempty"`
}

func ParseDryRunTransactionBlock(data []byte) (*DryRunTransactionBlock, error) {
	var dryRun DryRunTransactionBlock
	if err := json.Unmarshal(data, &dryRun); err != nil {
		return nil, err
	}
	return &dryRun, nil
}

func (r DryRunTransactionBlock) Success() bool {
	return transactionEffectsSuccess(&r.Effects)
}

func (r DryRunTransactionBlock) StatusError() string {
	return transactionEffectsStatusError(&r.Effects, "dry run failed")
}

func (r DryRunTransactionBlock) Gasless() bool {
	gasData := r.Input.GasData
	return gasData != nil &&
		len(gasData.Payment) == 0 &&
		gasData.Price == "0" &&
		gasData.Budget == "0"
}

func (r DryRunTransactionBlock) ValidateGaslessStablecoinPayment(asset string) error {
	if !r.Gasless() {
		return errors.New("transaction is not gasless")
	}
	if transactionEffectsHasObjectWrites(&r.Effects) || len(r.ObjectChanges) > 0 {
		return errors.New("transaction writes objects")
	}

	commands := TransactionCommands(r.Input.Transaction)
	if len(commands) == 0 {
		return errors.New("transaction has no programmable commands")
	}

	moveCallCount := 0
	for _, command := range commands {
		if command.Kind != CommandKindMoveCall || command.MoveCall == nil {
			return fmt.Errorf("unsupported programmable command %q", command.Kind)
		}
		moveCallCount++
		if err := ValidateGaslessStablecoinMoveCall(*command.MoveCall, asset); err != nil {
			return err
		}
	}
	if moveCallCount == 0 {
		return errors.New("transaction has no move calls")
	}

	return nil
}

func (r DryRunTransactionBlock) BalanceDelta(owner string, coinType string) *big.Int {
	total := new(big.Int)
	normalizedOwner := NormalizeAddress(owner)
	normalizedCoinType := NormalizeType(coinType)

	for _, change := range r.BalanceChanges {
		if OwnerAddress(change.Owner) != normalizedOwner {
			continue
		}
		if NormalizeType(change.CoinType) != normalizedCoinType {
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

func TransactionSender(txBytes []byte) (string, error) {
	if len(txBytes) == 0 {
		return "", ErrEmptyTransaction
	}
	txData, err := bcs.Unmarshal[gaslessStablecoinTransactionData](txBytes)
	if err != nil {
		return "", fmt.Errorf("invalid transaction data: %w", err)
	}
	if txData.V1 == nil {
		return "", errors.New("transaction missing V1 data")
	}
	return NormalizeAddress(txData.V1.Sender.String()), nil
}

func TransactionDigest(txBytes []byte) (string, error) {
	if len(txBytes) == 0 {
		return "", ErrEmptyTransaction
	}
	if len(txBytes) > int(^uint(0)>>1)-len(transactionDataDigestTag) {
		return "", errors.New("transaction data too large")
	}
	tagged := make([]byte, 0, len(transactionDataDigestTag)+len(txBytes))
	tagged = append(tagged, transactionDataDigestTag...)
	tagged = append(tagged, txBytes...)
	digest := blake2b.Sum256(tagged)
	return suitypes.Digest(digest[:]).String(), nil
}

func ParseExecuteTransactionBlock(data []byte) (*ExecuteTransactionBlock, error) {
	var executed ExecuteTransactionBlock
	if err := json.Unmarshal(data, &executed); err != nil {
		return nil, err
	}
	return &executed, nil
}

func TransactionResultStatusError(result *ExecuteTransactionBlock, fallback string) string {
	if result == nil {
		return fallback
	}
	if err := result.GetError(); err != nil && *err != "" {
		return *err
	}
	return transactionEffectsStatusError(result.Effects, fallback)
}

func TransactionResultBalanceDelta(result *ExecuteTransactionBlock, owner string, coinType string) *big.Int {
	total := new(big.Int)
	if result == nil {
		return total
	}

	normalizedOwner := NormalizeAddress(owner)
	normalizedCoinType := NormalizeType(coinType)
	for _, change := range result.BalanceChanges {
		if OwnerAddress(change.Owner) != normalizedOwner {
			continue
		}
		if NormalizeType(change.CoinType) != normalizedCoinType {
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

func MinimumGaslessStablecoinAmount(decimals uint8) *big.Int {
	if decimals <= 2 {
		return big.NewInt(1)
	}
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)-2), nil)
}

func transactionEffectsSuccess(e *TransactionEffects) bool {
	return e != nil && e.Status != nil && strings.EqualFold(e.Status.Status, "success")
}

func transactionEffectsStatusError(e *TransactionEffects, fallback string) string {
	if e != nil && e.Status != nil {
		if e.Status.Error != nil && *e.Status.Error != "" {
			return *e.Status.Error
		}
		if e.Status.Status != "" {
			return e.Status.Status
		}
	}
	return fallback
}

func transactionEffectsHasObjectWrites(e *TransactionEffects) bool {
	if e == nil {
		return false
	}
	return len(e.Created) > 0 ||
		len(e.Mutated) > 0 ||
		len(e.Deleted) > 0 ||
		len(e.Wrapped) > 0 ||
		len(e.Unwrapped) > 0
}

func OwnerAddress(owner interface{}) string {
	switch value := owner.(type) {
	case nil:
		return ""
	case json.RawMessage:
		rawText := strings.TrimSpace(string(value))
		if rawText == "" || rawText == "null" {
			return ""
		}

		var tagged map[string]json.RawMessage
		if err := json.Unmarshal(value, &tagged); err == nil {
			for _, key := range []string{"AddressOwner", "ObjectOwner"} {
				if taggedValue, ok := tagged[key]; ok {
					var address string
					if err := json.Unmarshal(taggedValue, &address); err == nil {
						return NormalizeAddress(address)
					}
				}
			}
		}

		var address string
		if err := json.Unmarshal(value, &address); err == nil {
			return NormalizeAddress(address)
		}
	case string:
		return NormalizeAddress(value)
	case map[string]interface{}:
		for _, key := range []string{"AddressOwner", "ObjectOwner"} {
			if taggedValue, ok := value[key]; ok {
				if address, ok := taggedValue.(string); ok {
					return NormalizeAddress(address)
				}
			}
		}
	case map[string]json.RawMessage:
		for _, key := range []string{"AddressOwner", "ObjectOwner"} {
			if taggedValue, ok := value[key]; ok {
				var address string
				if err := json.Unmarshal(taggedValue, &address); err == nil {
					return NormalizeAddress(address)
				}
			}
		}
	}

	raw, err := json.Marshal(owner)
	if err != nil {
		return ""
	}
	return OwnerAddress(json.RawMessage(raw))
}

func BuildGaslessStablecoinTransferTransaction(ctx context.Context, transfer GaslessStablecoinTransfer) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sender := NormalizeAddress(transfer.Sender)
	if sender == "" {
		return nil, errors.New("empty sender")
	}
	recipient := NormalizeAddress(transfer.Recipient)
	if recipient == "" {
		return nil, errors.New("empty recipient")
	}
	coinType, err := resolveGaslessStablecoinAsset(transfer.Network, transfer.Asset)
	if err != nil {
		return nil, err
	}

	amount, err := strconv.ParseUint(strings.TrimSpace(transfer.Amount), 10, 64)
	if err != nil || amount == 0 {
		return nil, fmt.Errorf("invalid amount: %s", transfer.Amount)
	}

	senderAddress, err := suiutils.ParseAddress(sender)
	if err != nil {
		return nil, err
	}
	recipientAddress, err := suiutils.ParseAddress(recipient)
	if err != nil {
		return nil, err
	}
	packageAddress, err := suiutils.ParseAddress("0x2")
	if err != nil {
		return nil, err
	}
	coinTypeTag, err := suiutils.ParseTypeTag(coinType)
	if err != nil {
		return nil, err
	}
	recipientBytes, err := bcs.Marshal(&recipientAddress)
	if err != nil {
		return nil, err
	}

	balanceInput := uint16(0)
	recipientInput := uint16(1)
	txData := gaslessStablecoinTransactionData{
		V1: &gaslessStablecoinTransactionDataV1{
			Kind: gaslessStablecoinTransactionKind{
				ProgrammableTransaction: &gaslessStablecoinProgrammableTransaction{
					Inputs: []gaslessStablecoinCallArg{
						{
							FundsWithdrawal: &gaslessStablecoinFundsWithdrawal{
								Reservation: gaslessStablecoinReservation{
									MaxAmountU64: &amount,
								},
								TypeArg: gaslessStablecoinWithdrawalType{
									Balance: &coinTypeTag,
								},
								WithdrawFrom: gaslessStablecoinWithdrawFrom{
									Sender: &struct{}{},
								},
							},
						},
						{
							Pure: &suitx.Pure{
								Bytes: recipientBytes,
							},
						},
					},
					Commands: []suitx.Command{
						{
							MoveCall: &suitx.ProgrammableMoveCall{
								Package:       packageAddress,
								Module:        "balance",
								Function:      "send_funds",
								TypeArguments: []suitypetag.TypeTag{coinTypeTag},
								Arguments: []suitx.Argument{
									{Input: &balanceInput},
									{Input: &recipientInput},
								},
							},
						},
					},
				},
			},
			Sender: senderAddress,
			GasData: suitx.GasData{
				Payment: []suitypes.ObjectRef{},
				Owner:   senderAddress,
				Price:   0,
				Budget:  0,
			},
			Expiration: suitx.ExpirationNone(),
		},
	}

	txBytes, err := bcs.Marshal(&txData)
	if err != nil {
		return nil, err
	}
	return txBytes, nil
}

type gaslessStablecoinTransactionData struct {
	V1 *gaslessStablecoinTransactionDataV1
}

func (gaslessStablecoinTransactionData) IsBcsEnum() {}

type gaslessStablecoinTransactionDataV1 struct {
	Kind       gaslessStablecoinTransactionKind
	Sender     suitypes.Address
	GasData    suitx.GasData
	Expiration suitx.TransactionExpiration
}

type gaslessStablecoinTransactionKind struct {
	ProgrammableTransaction *gaslessStablecoinProgrammableTransaction
	ChangeEpoch             *struct{}
	Genesis                 *struct{}
	ConsensusCommitPrologue *struct{}
}

func (gaslessStablecoinTransactionKind) IsBcsEnum() {}

type gaslessStablecoinProgrammableTransaction struct {
	Inputs   []gaslessStablecoinCallArg
	Commands []suitx.Command
}

type gaslessStablecoinCallArg struct {
	Pure            *suitx.Pure
	Object          *suitx.ObjectArg
	FundsWithdrawal *gaslessStablecoinFundsWithdrawal
}

func (gaslessStablecoinCallArg) IsBcsEnum() {}

type gaslessStablecoinFundsWithdrawal struct {
	Reservation  gaslessStablecoinReservation
	TypeArg      gaslessStablecoinWithdrawalType
	WithdrawFrom gaslessStablecoinWithdrawFrom
}

type gaslessStablecoinReservation struct {
	MaxAmountU64 *uint64
}

func (gaslessStablecoinReservation) IsBcsEnum() {}

type gaslessStablecoinWithdrawalType struct {
	Balance *suitypetag.TypeTag
}

func (gaslessStablecoinWithdrawalType) IsBcsEnum() {}

type gaslessStablecoinWithdrawFrom struct {
	Sender  *struct{}
	Sponsor *struct{}
}

func (gaslessStablecoinWithdrawFrom) IsBcsEnum() {}

func NewGaslessStablecoinPaymentPayload(ctx context.Context, recipient string, coinType string, amount string, signer Signer) (*Payload, error) {
	return NewGaslessStablecoinPaymentPayloadForNetwork(ctx, "", coinType, recipient, amount, signer)
}

func NewGaslessStablecoinPaymentPayloadForNetwork(ctx context.Context, network string, asset string, recipient string, amount string, signer Signer) (*Payload, error) {
	if signer == nil {
		return nil, errors.New("nil signer")
	}

	txBytes, err := BuildGaslessStablecoinTransferTransaction(ctx, GaslessStablecoinTransfer{
		Sender:    signer.Address(),
		Recipient: recipient,
		Network:   network,
		Asset:     asset,
		Amount:    amount,
	})
	if err != nil {
		return nil, err
	}
	return NewSignedPayload(txBytes, signer)
}

func NewGaslessStablecoinPaymentPayloadFromPrivateKeyHex(ctx context.Context, network string, asset string, recipient string, amount string, privateKeyHex string) (*Payload, error) {
	signer, err := NewEd25519SignerFromHex(privateKeyHex)
	if err != nil {
		return nil, err
	}
	return NewGaslessStablecoinPaymentPayloadForNetwork(ctx, network, asset, recipient, amount, signer)
}

func ValidateGaslessStablecoinMoveCall(moveCall MoveCallCommand, asset string) error {
	if NormalizeAddress(moveCall.Package) != NormalizeAddress("0x2") {
		return fmt.Errorf("unsupported Move package %q", moveCall.Package)
	}
	if !IsAllowedGaslessStablecoinMoveCall(moveCall.Module, moveCall.Function) {
		return fmt.Errorf("unsupported Move call %s::%s", moveCall.Module, moveCall.Function)
	}
	if len(moveCall.TypeArguments) == 0 {
		return fmt.Errorf("Move call %s::%s is missing stablecoin type argument", moveCall.Module, moveCall.Function)
	}

	expectedAsset := NormalizeType(asset)
	for _, typeArgument := range moveCall.TypeArguments {
		if NormalizeType(typeArgument) != expectedAsset {
			return fmt.Errorf("Move call type argument %q does not match required asset", typeArgument)
		}
	}

	return nil
}

func IsAllowedGaslessStablecoinMoveCall(module string, function string) bool {
	switch module {
	case "balance":
		switch function {
		case "send_funds", "redeem_funds", "withdrawal_split":
			return true
		}
	case "coin":
		switch function {
		case "send_funds", "into_balance":
			return true
		}
	}
	return false
}

func resolveGaslessStablecoinAsset(network string, asset string) (string, error) {
	asset = strings.TrimSpace(asset)
	if asset == "" {
		return "", errors.New("empty asset")
	}
	if network != "" {
		coinType, ok := GetGaslessStablecoinType(network, asset)
		if !ok {
			return "", fmt.Errorf("asset is not gasless stablecoin allowlisted on %s: %s", network, asset)
		}
		return coinType, nil
	}

	normalizedAsset := NormalizeType(asset)
	for symbol, coinType := range defaultStablecoinTypesBySymbol() {
		if NormalizeType(symbol) == normalizedAsset || NormalizeType(coinType) == normalizedAsset {
			return coinType, nil
		}
	}
	return "", fmt.Errorf("asset is not gasless stablecoin allowlisted: %s", asset)
}

func ParseTransactionKind(data []byte) (*TransactionKind, error) {
	var transaction TransactionKind
	if err := json.Unmarshal(data, &transaction); err != nil {
		return nil, err
	}
	return &transaction, nil
}

func TransactionCommands(transaction *TransactionKind) []TransactionCommand {
	if transaction == nil {
		return nil
	}

	commands := make([]TransactionCommand, 0, len(transaction.Transactions))
	for _, raw := range transaction.Transactions {
		command, err := ParseTransactionCommand(raw)
		if err != nil {
			commands = append(commands, TransactionCommand{
				Kind: "Invalid",
				Raw:  append(json.RawMessage(nil), raw...),
			})
			continue
		}
		commands = append(commands, *command)
	}
	return commands
}

type TransactionCommand struct {
	Kind     string
	MoveCall *MoveCallCommand
	Raw      json.RawMessage
}

func ParseTransactionCommand(data []byte) (*TransactionCommand, error) {
	var command TransactionCommand
	if err := json.Unmarshal(data, &command); err != nil {
		return nil, err
	}
	return &command, nil
}

func (c *TransactionCommand) UnmarshalJSON(data []byte) error {
	c.Raw = append(json.RawMessage(nil), data...)

	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("invalid programmable command: %w", err)
	}
	if len(object) == 0 {
		return errors.New("empty programmable command")
	}

	if payload, kind, ok := wrappedCommandPayload(object); ok {
		return c.unmarshalPayload(kind, payload)
	}
	if kind, ok := commandKind(object); ok {
		return c.unmarshalPayload(kind, data)
	}
	if len(object) == 1 {
		for kind := range object {
			c.Kind = kind
			return nil
		}
	}

	return errors.New("unknown programmable command")
}

func (c *TransactionCommand) unmarshalPayload(kind string, payload json.RawMessage) error {
	c.Kind = kind
	if kind == CommandKindMoveCall {
		command, err := unmarshalMoveCallCommand(payload)
		if err != nil {
			return fmt.Errorf("invalid MoveCall command: %w", err)
		}
		c.MoveCall = command
	}
	return nil
}

func unmarshalMoveCallCommand(data json.RawMessage) (*MoveCallCommand, error) {
	var moveCall MoveCallCommand
	_ = json.Unmarshal(data, &moveCall)

	var raw struct {
		Package            string            `json:"package"`
		PackageID          string            `json:"packageId"`
		PackageObjectID    string            `json:"packageObjectId"`
		Target             string            `json:"target"`
		Module             string            `json:"module"`
		Function           string            `json:"function"`
		TypeArgumentsSnake []string          `json:"type_arguments"`
		TypeArgumentsCamel []string          `json:"typeArguments"`
		Arguments          []json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	moveCall.Target = firstNonEmpty(moveCall.Target, raw.Target)
	moveCall.Package = firstNonEmpty(moveCall.Package, raw.Package, raw.PackageID, raw.PackageObjectID)
	moveCall.Module = firstNonEmpty(moveCall.Module, raw.Module)
	moveCall.Function = firstNonEmpty(moveCall.Function, raw.Function)
	if raw.Target != "" && (moveCall.Package == "" || moveCall.Module == "" || moveCall.Function == "") {
		parts := strings.Split(raw.Target, "::")
		if len(parts) == 3 {
			moveCall.Package = firstNonEmpty(moveCall.Package, parts[0])
			moveCall.Module = firstNonEmpty(moveCall.Module, parts[1])
			moveCall.Function = firstNonEmpty(moveCall.Function, parts[2])
		}
	}
	if len(moveCall.TypeArguments) == 0 {
		moveCall.TypeArguments = raw.TypeArgumentsSnake
	}
	if len(moveCall.TypeArguments) == 0 {
		moveCall.TypeArguments = raw.TypeArgumentsCamel
	}
	return &moveCall, nil
}

func wrappedCommandPayload(object map[string]json.RawMessage) (json.RawMessage, string, bool) {
	for key, payload := range object {
		if key == "kind" || key == "$kind" {
			continue
		}
		if kind, ok := canonicalCommandKind(key); ok {
			return payload, kind, true
		}
	}
	return nil, "", false
}

func commandKind(object map[string]json.RawMessage) (string, bool) {
	for _, key := range []string{"kind", "$kind"} {
		var value string
		if err := json.Unmarshal(object[key], &value); err == nil && value != "" {
			return canonicalCommandKindOrOriginal(value), true
		}
	}
	return "", false
}

func canonicalCommandKind(kind string) (string, bool) {
	switch normalizedCommandKind(kind) {
	case "movecall":
		return CommandKindMoveCall, true
	case "transferobjects":
		return CommandKindTransferObjects, true
	case "splitcoins":
		return CommandKindSplitCoins, true
	case "mergecoins":
		return CommandKindMergeCoins, true
	case "publish":
		return CommandKindPublish, true
	case "makemovevec":
		return CommandKindMakeMoveVec, true
	case "upgrade":
		return CommandKindUpgrade, true
	default:
		return "", false
	}
}

func canonicalCommandKindOrOriginal(kind string) string {
	if canonical, ok := canonicalCommandKind(kind); ok {
		return canonical
	}
	return kind
}

func normalizedCommandKind(kind string) string {
	kind = strings.TrimSpace(kind)
	kind = strings.ReplaceAll(kind, "_", "")
	kind = strings.ReplaceAll(kind, "-", "")
	return strings.ToLower(kind)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
