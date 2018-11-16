package handler

import (
	"encoding/hex"
	"math/big"
	"net/http"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	tcmn "github.com/thetatoken/ukulele/common"
	ttypes "github.com/thetatoken/ukulele/ledger/types"
	ukulele "github.com/thetatoken/ukulele/rpc"
	"github.com/thetatoken/vault/db"
	"github.com/thetatoken/vault/keymanager"
	"github.com/thetatoken/vault/util"
)

type ThetaRPCHandler struct {
	Client     util.RPCClient
	KeyManager keymanager.KeyManager
}

func NewRPCHandler(client util.RPCClient, km keymanager.KeyManager) *ThetaRPCHandler {
	return &ThetaRPCHandler{
		Client:     client,
		KeyManager: km,
	}
}

// ------------------------------- GetAccount -----------------------------------

type GetAccountArgs struct{}

type Account struct {
	Sequence               tcmn.JSONUint64       `json:"sequence"`
	Balance                ttypes.Coins          `json:"coins"`
	ReservedFunds          []ttypes.ReservedFund `json:"reserved_funds"`
	LastUpdatedBlockHeight tcmn.JSONUint64       `json:"last_updated_block_height"`
	Root                   tcmn.Hash             `json:"root"`
	CodeHash               tcmn.Hash             `json:"code"`
	Address                string                `json:"address"`
}

type GetAccountResult struct {
	UserID      string  `json:"user_id"`
	SendAccount Account `json:"send_account"` // Account to send from
	RecvAccount Account `json:"recv_account"` // Account to receive into
}

// getAccount is a helper function to query account from blockchain
func (h *ThetaRPCHandler) getAccount(address string) Account {
	acc := Account{Address: address}
	resp, err := h.Client.Call("theta.GetAccount", ukulele.GetAccountArgs{Address: address})
	if err != nil || resp.Error != nil {
		return acc
	}
	result := &ukulele.GetAccountResult{Account: ttypes.NewAccount()}
	err = resp.GetObject(result)
	if err != nil {
		return acc
	}
	acc.Sequence = tcmn.JSONUint64(result.Sequence)
	acc.Balance = result.Balance
	acc.ReservedFunds = result.ReservedFunds
	acc.LastUpdatedBlockHeight = tcmn.JSONUint64(result.LastUpdatedBlockHeight)
	acc.Root = result.Root
	acc.CodeHash = result.CodeHash
	return acc
}

func (h *ThetaRPCHandler) GetAccount(r *http.Request, args *GetAccountArgs, result *GetAccountResult) error {
	record, err := h.getRecord(r)
	if err != nil {
		return errors.Wrapf(err, "Failed to find userid: %v", record.UserID)
	}
	userid := record.UserID

	// Load SendAccount
	sendAccount := h.getAccount(record.SaAddress.String())
	result.SendAccount = sendAccount

	// Load RecvAccount
	recvAccount := h.getAccount(record.RaAddress.String())
	result.RecvAccount = recvAccount

	result.UserID = userid
	return nil
}

// ------------------------------- Send -----------------------------------

type SendArgs struct {
	To       string          `json:"to"`       // Required. Outputs including addresses and amount.
	Amount   ttypes.Coins    `json:"amount"`   // Required. The amount to send.
	Fee      *tcmn.JSONBig   `json:"fee"`      // Optional. Transaction fee. Default to 0.
	Sequence tcmn.JSONUint64 `json:"sequence"` // Required. Sequence number of this transaction.
}

func (h *ThetaRPCHandler) Send(r *http.Request, args *SendArgs, result *ukulele.BroadcastRawTransactionResult) (err error) {
	record, err := h.getRecord(r)
	if err != nil {
		return
	}

	signedTx, err := prepareSendTx(args, record, viper.GetString(util.CfgThetaChainId))
	if err != nil {
		return err
	}
	return h.broadcastTx(signedTx, result)
}

func prepareSendTx(args *SendArgs, record db.Record, chainID string) (*ttypes.SendTx, error) {
	amount := args.Amount.NoNil()

	// Add minimal fee.
	feeAmount := new(big.Int)
	if args.Fee == nil {
		feeAmount.SetUint64(ttypes.MinimumTransactionFeeGammaWei)
	} else {
		feeAmount = (*big.Int)(args.Fee)
	}

	fee := ttypes.Coins{
		ThetaWei: ttypes.Zero,
		GammaWei: feeAmount,
	}
	inputs := []ttypes.TxInput{{
		Address:  record.RaAddress,
		Coins:    amount.Plus(fee),
		Sequence: (uint64)(args.Sequence),
	}}
	if args.Sequence == 1 {
		inputs[0].PubKey = record.RaPubKey
	}
	outputs := []ttypes.TxOutput{{
		Address: tcmn.HexToAddress(args.To),
		Coins:   args.Amount,
	}}
	sendTx := &ttypes.SendTx{
		Fee:     fee,
		Inputs:  inputs,
		Outputs: outputs,
	}

	sig, err := record.RaPrivateKey.Sign(sendTx.SignBytes(chainID))
	if err != nil {
		return nil, err
	}
	sendTx.SetSignature(record.RaAddress, sig)
	return sendTx, nil
}

// --------------------------- BroadcastRawTransaction ----------------------------

func (h *ThetaRPCHandler) BroadcastRawTransaction(r *http.Request, args *ukulele.BroadcastRawTransactionArgs, result *ukulele.BroadcastRawTransactionResult) (err error) {
	resp, err := h.Client.Call("theta.BroadcastRawTransaction", args)
	if err != nil {
		return
	}
	if resp.Error != nil {
		err = resp.Error
		return
	}

	err = resp.GetObject(&result)
	return
}

// --------------------------- Reserve -------------------------------

type ReserveFundArgs struct {
	Fee         *tcmn.JSONBig   `json:"fee"`          // Optional. Transaction fee. Default to 0.
	Collateral  *tcmn.JSONBig   `json:"collateral"`   // Required. Amount in GammaWei as the collateral
	Fund        *tcmn.JSONBig   `json:"fund"`         // Required. Amount in GammaWei to reserve.
	ResourceIds []string        `json:"resource_ids"` // List of resource ID
	Duration    tcmn.JSONUint64 `json:"duration"`     // Optional. Number of blocks to lock the fund.
	Sequence    tcmn.JSONUint64 `json:"sequence"`     // Required. Sequence number of this transaction.
}

type ReserveFundResult struct {
	*ukulele.BroadcastRawTransactionResult
	ReserveSequence tcmn.JSONUint64 `json:"reserve_sequence"` // Sequence number of the reserved fund.
}

func (h *ThetaRPCHandler) ReserveFund(r *http.Request, args *ReserveFundArgs, result *ReserveFundResult) (err error) {
	record, err := h.getRecord(r)
	if err != nil {
		return
	}

	signedTx, err := prepareReserveFundTx(args, record, viper.GetString(util.CfgThetaChainId))
	if err != nil {
		return err
	}
	err = h.broadcastTx(signedTx, result)
	if err != nil {
		return err
	}

	result.ReserveSequence = args.Sequence
	return nil
}

func prepareReserveFundTx(args *ReserveFundArgs, record db.Record, chainID string) (*ttypes.ReserveFundTx, error) {
	if args.Duration == 0 {
		args.Duration = tcmn.JSONUint64(viper.GetInt64(util.CfgThetaDefaultReserveDurationSecs))
	}

	// Add minimal fee.
	feeAmount := new(big.Int)
	if args.Fee == nil {
		feeAmount.SetUint64(ttypes.MinimumTransactionFeeGammaWei)
	} else {
		feeAmount = (*big.Int)(args.Fee)
	}

	// Send from SendAccount
	input := ttypes.TxInput{
		Coins: ttypes.Coins{
			ThetaWei: big.NewInt(0),
			GammaWei: (*big.Int)(args.Fund),
		},
		Sequence: uint64(args.Sequence),
		Address:  record.SaAddress,
	}
	if args.Sequence == 1 {
		input.PubKey = record.SaPubKey
	}

	var resourceIds []string
	for _, ridStr := range args.ResourceIds {
		resourceIds = append(resourceIds, ridStr)
	}

	collateral := ttypes.Coins{
		ThetaWei: big.NewInt(0),
		GammaWei: (*big.Int)(args.Collateral),
	}
	tx := &ttypes.ReserveFundTx{
		Fee: ttypes.Coins{
			ThetaWei: ttypes.Zero,
			GammaWei: feeAmount,
		},
		Source:      input,
		Collateral:  collateral,
		ResourceIDs: resourceIds,
		Duration:    uint64(args.Duration),
	}

	sig, err := record.SaPrivateKey.Sign(tx.SignBytes(chainID))
	if err != nil {
		return nil, err
	}
	tx.SetSignature(record.SaAddress, sig)

	return tx, nil
}

// --------------------------- Release -------------------------------

type ReleaseFundArgs struct {
	Fee             *tcmn.JSONBig   `json:"fee"`              // Optional. Transaction fee. Default to 0.
	Sequence        tcmn.JSONUint64 `json:"sequence"`         // Required. Sequence number of this transaction.
	ReserveSequence tcmn.JSONUint64 `json:"reserve_sequence"` // Required. Sequence number of the fund to release.
}

type ReleaseFundResult struct {
	*ukulele.BroadcastRawTransactionResult
	ReserveSequence uint64 `json:"reserve_sequence"` // Sequence number of the reserved fund.
}

func (h *ThetaRPCHandler) ReleaseFund(r *http.Request, args *ReleaseFundArgs, result *ReleaseFundResult) (err error) {
	record, err := h.getRecord(r)
	if err != nil {
		return
	}

	signedTx, err := prepareReleaseFundTx(args, record, viper.GetString(util.CfgThetaChainId))
	if err != nil {
		return err
	}
	return h.broadcastTx(signedTx, result)
}

func prepareReleaseFundTx(args *ReleaseFundArgs, record db.Record, chainID string) (*ttypes.ReleaseFundTx, error) {
	// Add minimal fee.
	feeAmount := new(big.Int)
	if args.Fee == nil {
		feeAmount.SetUint64(ttypes.MinimumTransactionFeeGammaWei)
	} else {
		feeAmount = (*big.Int)(args.Fee)
	}

	// Wrap and add signer
	input := ttypes.TxInput{
		Sequence: uint64(args.Sequence),
		Address:  record.SaAddress,
	}
	if args.Sequence == 1 {
		input.PubKey = record.SaPubKey
	}

	tx := &ttypes.ReleaseFundTx{
		Fee: ttypes.Coins{
			ThetaWei: ttypes.Zero,
			GammaWei: feeAmount,
		},
		Source:          input,
		ReserveSequence: uint64(args.ReserveSequence),
	}

	sig, err := record.RaPrivateKey.Sign(tx.SignBytes(chainID))
	if err != nil {
		return nil, err
	}
	tx.SetSignature(record.SaAddress, sig)
	return tx, nil
}

// --------------------------- CreateServicePayment -------------------------------

type CreateServicePaymentArgs struct {
	To              string          `json:"to"`               // Required. Address to target account.
	Amount          *tcmn.JSONBig   `json:"amount"`           // Required. Amount of payment in GammaWei
	ResourceId      string          `json:"resource_id"`      // Required. Resource ID the payment is for.
	PaymentSequence tcmn.JSONUint64 `json:"payment_sequence"` // Required. each on-chain settlement needs to increase the payment sequence by 1
	ReserveSequence tcmn.JSONUint64 `json:"reserve_sequence"` // Required. Sequence number of the fund to send.
}

type CreateServicePaymentResult struct {
	Payment string `json:"payment"` // Hex encoded half-signed payment tx bytes.
}

func (h *ThetaRPCHandler) CreateServicePayment(r *http.Request, args *CreateServicePaymentArgs, result *CreateServicePaymentResult) (err error) {
	record, err := h.getRecord(r)
	if err != nil {
		return
	}
	signedTx, err := prepareCreateServicePaymentTx(args, record, viper.GetString(util.CfgThetaChainId))
	if err != nil {
		return
	}
	result.Payment = signedTx
	return nil
}

func prepareCreateServicePaymentTx(args *CreateServicePaymentArgs, record db.Record, chainID string) (string, error) {
	if args.ResourceId == "" {
		return "", errors.New("No resource_id is provided")
	}

	if args.To == record.RaAddress.String() || args.To == record.SaAddress.String() {
		// You don't need to pay yourself.
		return "", nil
	}

	// Send from SendAccount
	address := record.SaAddress

	// Wrap and add signer
	sourceInput := ttypes.TxInput{}
	sourceInput.Address = address
	sourceInput.Coins = ttypes.Coins{
		ThetaWei: ttypes.Zero,
		GammaWei: (*big.Int)(args.Amount),
	}

	targetAddress := tcmn.HexToAddress(args.To)
	targetInput := ttypes.TxInput{
		Address: targetAddress,
	}

	tx := &ttypes.ServicePaymentTx{
		Source:          sourceInput,
		Target:          targetInput,
		PaymentSequence: uint64(args.PaymentSequence),
		ReserveSequence: uint64(args.ReserveSequence),
		ResourceID:      args.ResourceId,
	}

	sig, err := record.SaPrivateKey.Sign(tx.SourceSignBytes(chainID))
	if err != nil {
		return "", err
	}
	tx.SetSourceSignature(sig)
	signedTx, err := ttypes.TxToBytes(tx)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(signedTx), nil
}

// --------------------------- SubmitServicePayment -------------------------------

type SubmitServicePaymentArgs struct {
	Fee      *tcmn.JSONBig   `json:"fee"`      // Optional. Transaction fee. Default to 0.
	Payment  string          `json:"payment"`  // Required. Hex of sender-signed payment stub.
	Sequence tcmn.JSONUint64 `json:"sequence"` // Required. Sequence number of this transaction.
}

func (h *ThetaRPCHandler) SubmitServicePayment(r *http.Request, args *SubmitServicePaymentArgs, result *ukulele.BroadcastRawTransactionResult) (err error) {
	record, err := h.getRecord(r)
	if err != nil {
		return
	}
	signedTx, err := prepareSubmitServicePaymentTx(args, record, viper.GetString(util.CfgThetaChainId))
	if err != nil {
		return err
	}
	return h.broadcastTx(signedTx, result)
}

func prepareSubmitServicePaymentTx(args *SubmitServicePaymentArgs, record db.Record, chainID string) (*ttypes.ServicePaymentTx, error) {
	// Receive into RecvAccount
	address := record.RaAddress

	input := ttypes.TxInput{
		Sequence: uint64(args.Sequence),
	}
	input.Address = address
	if args.Sequence == 1 {
		input.PubKey = record.RaPubKey
	}

	if args.Payment == "" {
		return nil, errors.Errorf("Payment is empty")
	}

	paymentBytes, err := hex.DecodeString(args.Payment)
	if err != nil {
		return nil, err
	}

	tx, err := ttypes.TxFromBytes(paymentBytes)
	if err != nil {
		return nil, err
	}
	paymentTx := tx.(*ttypes.ServicePaymentTx)
	paymentTx.Target = input

	// Add minimal fee.
	feeAmount := new(big.Int)
	if args.Fee == nil {
		feeAmount.SetUint64(ttypes.MinimumTransactionFeeGammaWei)
	} else {
		feeAmount = (*big.Int)(args.Fee)
	}

	paymentTx.Fee = ttypes.Coins{
		ThetaWei: ttypes.Zero,
		GammaWei: feeAmount,
	}

	// Sign the tx
	sig, err := record.RaPrivateKey.Sign(paymentTx.TargetSignBytes(chainID))
	if err != nil {
		return nil, err
	}
	paymentTx.SetTargetSignature(sig)
	return paymentTx, nil
}

// --------------------------- InstantiateSplitContract -------------------------------

type InstantiateSplitContractArgs struct {
	Fee          *tcmn.JSONBig   `json:"fee"`          // Optional. Transaction fee. Default to 0.
	ResourceId   string          `json:"resource_id"`  // Required. The resourceId.
	Initiator    string          `json:"initiator"`    // Required. Name of initiator account.
	Participants []string        `json:"participants"` // Required. User IDs participating in the split.
	Percentages  []uint          `json:"percentages"`  // Required. The split percentage for each corresponding user.
	Duration     tcmn.JSONUint64 `json:"duration"`     // Optional. Number of blocks before the contract expires.
	Sequence     tcmn.JSONUint64 `json:"sequence"`     // Optional. Sequence number of this transaction.
}

func (h *ThetaRPCHandler) InstantiateSplitContract(r *http.Request, args *InstantiateSplitContractArgs, result *ukulele.BroadcastRawTransactionResult) (err error) {
	if args.Initiator == "" {
		return errors.New("No initiator is passed in")
	}
	initiator, err := h.KeyManager.FindByUserId(args.Initiator)
	if err != nil {
		return
	}
	participants := []db.Record{}
	for _, userid := range args.Participants {
		record, err := h.KeyManager.FindByUserId(userid)
		if err != nil {
			return err
		}
		participants = append(participants, record)
	}
	sequence, err := util.GetSequence(h.Client, initiator.SaAddress)
	signedTx, err := prepareInstantiateSplitContractTx(args, initiator, sequence, participants, viper.GetString(util.CfgThetaChainId))
	if err != nil {
		return err
	}
	return h.broadcastTx(signedTx, result)
}

func prepareInstantiateSplitContractTx(args *InstantiateSplitContractArgs, initiator db.Record, initiatorSeq uint64, participants []db.Record, chainID string) (*ttypes.SplitRuleTx, error) {
	if args.ResourceId == "" {
		return nil, errors.New("No resource_id is passed in")
	}

	if len(args.Participants) != len(args.Percentages) {
		return nil, errors.New("Length of participants doesn't match with length of percentages")
	}
	// Add minimal fee.
	feeAmount := new(big.Int)
	if args.Fee == nil {
		feeAmount.SetUint64(ttypes.MinimumTransactionFeeGammaWei)
	} else {
		feeAmount = (*big.Int)(args.Fee)
	}

	// Use SendAccount to fund tx fee.
	initiatorAddress := initiator.SaAddress

	initiatorInput := ttypes.TxInput{
		Address:  initiatorAddress,
		Sequence: initiatorSeq + 1,
	}
	if args.Sequence == 1 {
		initiatorInput.PubKey = initiator.SaPubKey
	}

	splits := []ttypes.Split{}
	for idx, record := range participants {
		address := record.RaAddress
		percentage := args.Percentages[idx]
		splits = append(splits, ttypes.Split{
			Address:    address,
			Percentage: percentage,
		})
	}

	duration := uint64(86400 * 365 * 10)
	if args.Duration != 0 {
		duration = uint64(args.Duration)
	}

	tx := &ttypes.SplitRuleTx{
		Fee: ttypes.Coins{
			ThetaWei: ttypes.Zero,
			GammaWei: feeAmount,
		},
		ResourceID: args.ResourceId,
		Initiator:  initiatorInput,
		Splits:     splits,
		Duration:   duration,
	}

	sig, err := initiator.RaPrivateKey.Sign(tx.SignBytes(chainID))
	if err != nil {
		return nil, err
	}
	tx.SetSignature(initiator.RaAddress, sig)
	return tx, nil
}

//
// --------------------------- helpers -------------------------------
//

// getRecord retrieves user record from database based on user id in request header.
func (h *ThetaRPCHandler) getRecord(r *http.Request) (record db.Record, err error) {

	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		err = errors.New("No userid is passed in")
		return
	}
	return h.KeyManager.FindByUserId(userid)
}

// broadcastTx takes a signed TX and broadcast to Theta backend. The response is filled into
// the result argument.
func (h *ThetaRPCHandler) broadcastTx(tx ttypes.Tx, result interface{}) error {
	raw, err := ttypes.TxToBytes(tx)
	if err != nil {
		return err
	}
	signedTx := hex.EncodeToString(raw)
	broadcastArgs := &ukulele.BroadcastRawTransactionArgs{TxBytes: signedTx}
	resp, err := h.Client.Call("theta.BroadcastRawTransaction", broadcastArgs)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return resp.Error
	}
	return resp.GetObject(&result)
}
