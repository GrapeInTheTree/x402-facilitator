package sui

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"testing"

	bcs "github.com/iotaledger/bcs-go"
	"github.com/stretchr/testify/require"
)

func TestTransactionBlockDataParsesProgrammableCommands(t *testing.T) {
	raw := []byte(`{
		"transaction": {
			"kind": "ProgrammableTransaction",
			"inputs": [{"Pure": [1, 2, 3]}],
			"transactions": [
				{"MoveCall": {
					"package": "0x2",
					"module": "balance",
					"function": "send_funds",
					"type_arguments": ["` + USDCType + `"],
					"arguments": [{"Input": 0}]
				}},
				{"TransferObjects": {"objects": [{"Result": 0}], "address": {"Input": 1}}},
				{"SplitCoins": {"coin": {"Input": 0}, "amounts": [{"Input": 1}]}},
				{"MergeCoins": {"destination": {"Input": 0}, "sources": [{"Input": 1}]}},
				{"Publish": {"modules": ["AA=="], "dependencies": ["0x1"]}},
				{"MakeMoveVec": {"type": "0x2::sui::SUI", "elements": [{"Input": 0}]}},
				{"Upgrade": {"modules": ["AA=="], "dependencies": ["0x1"], "package": "0x2", "ticket": {"Input": 0}}}
			]
		},
		"gasData": {
			"payment": [],
			"owner": "0xabc",
			"price": "0",
			"budget": "0"
		}
	}`)

	parsed, err := ParseTransactionBlockData(raw)
	require.NoError(t, err)
	require.Equal(t, TransactionKindProgrammable, parsed.Transaction.Kind)
	require.Equal(t, "0", parsed.GasData.Price)
	require.Equal(t, "0", parsed.GasData.Budget)

	commands := TransactionCommands(parsed.Transaction)
	require.Len(t, commands, 7)
	require.Equal(t, CommandKindMoveCall, commands[0].Kind)
	require.Equal(t, "0x2", commands[0].MoveCall.Package)
	require.Equal(t, "balance", commands[0].MoveCall.Module)
	require.Equal(t, "send_funds", commands[0].MoveCall.Function)
	require.Equal(t, []string{USDCType}, commands[0].MoveCall.TypeArguments)
	require.Equal(t, CommandKindTransferObjects, commands[1].Kind)
	require.Equal(t, CommandKindSplitCoins, commands[2].Kind)
	require.Equal(t, CommandKindMergeCoins, commands[3].Kind)
	require.Equal(t, CommandKindPublish, commands[4].Kind)
	require.Equal(t, CommandKindMakeMoveVec, commands[5].Kind)
	require.Equal(t, CommandKindUpgrade, commands[6].Kind)
}

func TestTransactionCommandParsesKindFormAndTarget(t *testing.T) {
	moveCall, err := ParseTransactionCommand([]byte(`{
		"kind": "MoveCall",
		"target": "0x2::balance::send_funds",
		"typeArguments": ["` + USDCType + `"],
		"arguments": []
	}`))
	require.NoError(t, err)
	require.Equal(t, CommandKindMoveCall, moveCall.Kind)
	require.Equal(t, "0x2", moveCall.MoveCall.Package)
	require.Equal(t, "balance", moveCall.MoveCall.Module)
	require.Equal(t, "send_funds", moveCall.MoveCall.Function)
	require.Equal(t, []string{USDCType}, moveCall.MoveCall.TypeArguments)

	splitCoins, err := ParseTransactionCommand([]byte(`{
		"$kind": "SplitCoins",
		"coin": {"Input": 0},
		"amounts": [{"Input": 1}]
	}`))
	require.NoError(t, err)
	require.Equal(t, CommandKindSplitCoins, splitCoins.Kind)
}

func TestTransactionCommandKeepsUnknownWrappedKind(t *testing.T) {
	command, err := ParseTransactionCommand([]byte(`{"MysteryCommand": {"value": 1}}`))
	require.NoError(t, err)
	require.Equal(t, "MysteryCommand", command.Kind)
	require.NotEmpty(t, command.Raw)
}

func TestGaslessStablecoinPaymentRejectsUnsupportedMoveCall(t *testing.T) {
	dryRun := DryRunTransactionBlock{
		Input: TransactionBlockData{
			Transaction: &TransactionKind{
				Transactions: []json.RawMessage{
					moveCallTransactionCommand("0x2", "transfer", "public_transfer", []string{USDCType}),
				},
			},
			GasData: &GasData{
				Payment: nil,
				Price:   "0",
				Budget:  "0",
			},
		},
		Effects: TransactionEffects{Status: &TransactionExecutionStatus{Status: "success"}},
	}

	require.ErrorContains(t, dryRun.ValidateGaslessStablecoinPayment(USDCType), "unsupported Move call")
}

func TestGaslessStablecoinPaymentRejectsObjectWrites(t *testing.T) {
	dryRun := DryRunTransactionBlock{
		Input: TransactionBlockData{
			Transaction: &TransactionKind{
				Transactions: []json.RawMessage{
					moveCallTransactionCommand("0x2", "balance", "send_funds", []string{USDCType}),
				},
			},
			GasData: &GasData{
				Payment: nil,
				Price:   "0",
				Budget:  "0",
			},
		},
		Effects: TransactionEffects{
			Status:  &TransactionExecutionStatus{Status: "success"},
			Mutated: []ObjectOwnerResult{{}},
		},
	}

	require.ErrorContains(t, dryRun.ValidateGaslessStablecoinPayment(USDCType), "transaction writes objects")
}

func TestGaslessStablecoinPaymentRejectsGasPayment(t *testing.T) {
	dryRun := DryRunTransactionBlock{
		Input: TransactionBlockData{
			Transaction: &TransactionKind{
				Transactions: []json.RawMessage{
					moveCallTransactionCommand("0x2", "balance", "send_funds", []string{USDCType}),
				},
			},
			GasData: &GasData{
				Payment: []ObjectRefResult{{}},
				Price:   "1000",
				Budget:  "1000000",
			},
		},
		Effects: TransactionEffects{Status: &TransactionExecutionStatus{Status: "success"}},
	}

	require.ErrorContains(t, dryRun.ValidateGaslessStablecoinPayment(USDCType), "transaction is not gasless")
}

func TestOwnerAddressRejectsEmptyInputs(t *testing.T) {
	require.Empty(t, NormalizeAddress(""))
	require.Empty(t, NormalizeAddress("0x"))
	require.Empty(t, OwnerAddress(nil))
	require.Empty(t, OwnerAddress(json.RawMessage("null")))
}

func TestBuildGaslessStablecoinTransferTransaction(t *testing.T) {
	signer := newTestSigner(t)

	txBytes, err := BuildGaslessStablecoinTransferTransaction(context.Background(), GaslessStablecoinTransfer{
		Sender:    signer.Address(),
		Recipient: "0xabc",
		Network:   "sui:mainnet",
		Asset:     "USDC",
		Amount:    "1000000",
	})
	require.NoError(t, err)
	require.NotEmpty(t, txBytes)
}

func TestNewGaslessStablecoinPaymentPayload(t *testing.T) {
	signer := newTestSigner(t)

	payload, err := NewGaslessStablecoinPaymentPayloadForNetwork(context.Background(), "sui:mainnet", "USDC", "0xabc", "1000000", signer)
	require.NoError(t, err)
	require.NotEmpty(t, payload.Transaction)
	require.NotEmpty(t, payload.Signature)

	txBytes, err := payload.DecodeTransaction()
	require.NoError(t, err)
	payer, err := VerifySignature(payload.Signature, txBytes)
	require.NoError(t, err)
	require.Equal(t, signer.Address(), payer)
}

func TestBuildGaslessStablecoinTransferTransactionRejectsInvalidInput(t *testing.T) {
	signer := newTestSigner(t)

	_, err := BuildGaslessStablecoinTransferTransaction(context.Background(), GaslessStablecoinTransfer{
		Sender:    signer.Address(),
		Recipient: "0xabc",
		Network:   "sui:mainnet",
		Asset:     "0x2::sui::SUI",
		Amount:    "1000000",
	})
	require.ErrorContains(t, err, "not gasless stablecoin allowlisted")

	_, err = BuildGaslessStablecoinTransferTransaction(context.Background(), GaslessStablecoinTransfer{
		Sender:    signer.Address(),
		Recipient: "0xabc",
		Asset:     USDCType,
		Amount:    "0",
	})
	require.ErrorContains(t, err, "invalid amount")
}

func TestNewGaslessStablecoinPaymentPayloadFromPrivateKeyHex(t *testing.T) {
	privateKey := make([]byte, 32)
	for i := range privateKey {
		privateKey[i] = byte(i + 1)
	}

	payload, err := NewGaslessStablecoinPaymentPayloadFromPrivateKeyHex(
		context.Background(),
		"sui:mainnet",
		"USDC",
		"0xabc",
		"1000000",
		hex.EncodeToString(privateKey),
	)
	require.NoError(t, err)
	require.NotEmpty(t, payload.Transaction)
	require.NotEmpty(t, payload.Signature)
}

func moveCallTransactionCommand(pkg string, module string, function string, typeArguments []string) json.RawMessage {
	raw, err := json.Marshal(map[string]interface{}{
		"MoveCall": map[string]interface{}{
			"package":        pkg,
			"module":         module,
			"function":       function,
			"type_arguments": typeArguments,
		},
	})
	if err != nil {
		panic(err)
	}
	return raw
}

func TestBuildGaslessStablecoinTransferTransactionUsesFundsWithdrawal(t *testing.T) {
	txBytes, err := BuildGaslessStablecoinTransferTransaction(context.Background(), GaslessStablecoinTransfer{
		Sender:    "0x123",
		Recipient: "0xabc",
		Network:   "sui:mainnet",
		Asset:     "USDC",
		Amount:    "1000000",
	})
	require.NoError(t, err)
	require.Equal(t, "000002020040420f00000000000007dba34672e30cb065b1f93e3ab55318768fd6fef66c15942c9f7cb846e2f900e704757364630455534443000000200000000000000000000000000000000000000000000000000000000000000abc010000000000000000000000000000000000000000000000000000000000000000020762616c616e63650a73656e645f66756e64730107dba34672e30cb065b1f93e3ab55318768fd6fef66c15942c9f7cb846e2f900e704757364630455534443000201000001010000000000000000000000000000000000000000000000000000000000000001230000000000000000000000000000000000000000000000000000000000000001230000000000000000000000000000000000", hex.EncodeToString(txBytes))
	sender, err := TransactionSender(txBytes)
	require.NoError(t, err)
	require.Equal(t, NormalizeAddress("0x123"), sender)
	digest, err := TransactionDigest(txBytes)
	require.NoError(t, err)
	require.Equal(t, "7fJTuWTvTzfU53y6MYiffc1LniEBxbCPdpgZNZagKfec", digest)

	txData, err := bcs.Unmarshal[gaslessStablecoinTransactionData](txBytes)
	require.NoError(t, err)
	require.NotNil(t, txData.V1)
	require.Equal(t, NormalizeAddress("0x123"), txData.V1.Sender.String())
	require.Empty(t, txData.V1.GasData.Payment)
	require.Equal(t, uint64(0), txData.V1.GasData.Price)
	require.Equal(t, uint64(0), txData.V1.GasData.Budget)

	programmable := txData.V1.Kind.ProgrammableTransaction
	require.NotNil(t, programmable)
	require.Len(t, programmable.Inputs, 2)
	require.NotNil(t, programmable.Inputs[0].FundsWithdrawal)
	require.NotNil(t, programmable.Inputs[0].FundsWithdrawal.Reservation.MaxAmountU64)
	require.Equal(t, uint64(1000000), *programmable.Inputs[0].FundsWithdrawal.Reservation.MaxAmountU64)
	require.NotNil(t, programmable.Inputs[0].FundsWithdrawal.TypeArg.Balance)
	require.Equal(t, NormalizeType(USDCType), NormalizeType(programmable.Inputs[0].FundsWithdrawal.TypeArg.Balance.String()))
	require.NotNil(t, programmable.Inputs[0].FundsWithdrawal.WithdrawFrom.Sender)
	require.NotNil(t, programmable.Inputs[1].Pure)

	require.Len(t, programmable.Commands, 1)
	require.NotNil(t, programmable.Commands[0].MoveCall)
	moveCall := programmable.Commands[0].MoveCall
	require.Equal(t, NormalizeAddress("0x2"), moveCall.Package.String())
	require.Equal(t, "balance", moveCall.Module)
	require.Equal(t, "send_funds", moveCall.Function)
	require.Len(t, moveCall.TypeArguments, 1)
	require.Equal(t, NormalizeType(USDCType), NormalizeType(moveCall.TypeArguments[0].String()))
	require.Len(t, moveCall.Arguments, 2)
	require.NotNil(t, moveCall.Arguments[0].Input)
	require.Equal(t, uint16(0), *moveCall.Arguments[0].Input)
	require.NotNil(t, moveCall.Arguments[1].Input)
	require.Equal(t, uint16(1), *moveCall.Arguments[1].Input)
}
