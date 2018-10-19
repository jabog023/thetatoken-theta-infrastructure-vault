package handler

import (
	"encoding/hex"
	"math/big"
	"net/http"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	tcmn "github.com/thetatoken/ukulele/common"
	ttypes "github.com/thetatoken/ukulele/ledger/types"
	ukulele "github.com/thetatoken/ukulele/rpc"
	"github.com/thetatoken/vault/keymanager"
	"github.com/thetatoken/vault/util"
	rpcc "github.com/ybbus/jsonrpc"
)

type RPCClient interface {
	Call(method string, params ...interface{}) (*rpcc.RPCResponse, error)
}
type ThetaRPCHandler struct {
	Client     RPCClient
	KeyManager keymanager.KeyManager
}

func NewRPCHandler(client RPCClient, km keymanager.KeyManager) *ThetaRPCHandler {
	return &ThetaRPCHandler{
		Client:     client,
		KeyManager: km,
	}
}

// ------------------------------- GetAccount -----------------------------------

type GetAccountArgs struct{}

type GetAccountResult struct {
	UserID      string                    `json:"user_id"`
	SendAccount *ukulele.GetAccountResult `json:"send_account"` // Account to send from
	RecvAccount *ukulele.GetAccountResult `json:"recv_account"` // Account to receive into
}

// getAccount is a helper function to query account from blockchain
func (h *ThetaRPCHandler) getAccount(address string) (*ukulele.GetAccountResult, error) {
	resp, err := h.Client.Call("theta.GetAccount", ukulele.GetAccountArgs{Address: address})
	if err != nil {
		return nil, errors.Wrap(err, "Error in RPC call")
	}
	result := &ukulele.GetAccountResult{}
	err = resp.GetObject(result)
	result.Address = address
	return result, nil
}

func (h *ThetaRPCHandler) GetAccount(r *http.Request, args *GetAccountArgs, result *GetAccountResult) (err error) {
	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		return errors.New("No userid is passed in")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return errors.Wrapf(err, "Failed to find userid: %v", userid)
	}

	// Load SendAccount
	sendAccount, err := h.getAccount(record.SaAddress.String())
	if err != nil {
		return errors.Wrapf(err, "Failed to find sendAccount for %v", userid)
	}
	result.SendAccount = sendAccount

	// Load RecvAccount
	recvAccount, err := h.getAccount(record.RaAddress.String())
	if err != nil {
		return errors.Wrapf(err, "Failed to find recvAccount for %v", userid)
	}
	result.RecvAccount = recvAccount

	return
}

// ------------------------------- Send -----------------------------------

type SendArgs struct {
	To       string       `json:"to"`       // Required. Outputs including addresses and amount.
	Amount   ttypes.Coins `json:"amount"`   // Required. The amount to send.
	Fee      uint64       `json:"fee"`      // Optional. Transaction fee. Default to 0.
	Gas      uint64       `json:"gas"`      // Optional. Amount of gas. Default to 0.
	Sequence uint64       `json:"sequence"` // Required. Sequence number of this transaction.
}

func (h *ThetaRPCHandler) Send(r *http.Request, args *SendArgs, result *ukulele.BroadcastRawTransactionResult) (err error) {
	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		return errors.New("No userid is passed in")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return
	}

	amount := args.Amount.NoNil()

	// Add minimal gas/fee.
	if args.Gas == 0 {
		args.Gas = 1
	}
	if args.Fee == 0 {
		args.Fee = 1
	}

	fee := ttypes.Coins{
		ThetaWei: ttypes.Zero,
		GammaWei: big.NewInt(0).SetUint64(args.Fee),
	}
	inputs := []ttypes.TxInput{{
		Address:  record.RaAddress,
		Coins:    amount.Plus(fee),
		Sequence: args.Sequence,
	}}
	outputs := []ttypes.TxOutput{{
		Address: tcmn.HexToAddress(args.To),
		Coins:   args.Amount,
	}}
	sendTx := &ttypes.SendTx{
		Fee:     fee,
		Gas:     args.Gas,
		Inputs:  inputs,
		Outputs: outputs,
	}

	chainID := viper.GetString(util.CfgThetaChainId)
	sig, err := record.RaPrivateKey.Sign(sendTx.SignBytes(chainID))
	if err != nil {
		return err
	}
	sendTx.SetSignature(record.RaAddress, sig)

	raw, err := ttypes.TxToBytes(sendTx)
	if err != nil {
		return err
	}
	signedTx := hex.EncodeToString(raw)

	broadcastArgs := &ukulele.BroadcastRawTransactionArgs{TxBytes: signedTx}
	resp, err := h.Client.Call("theta.BroadcastRawTransaction", broadcastArgs)
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
	Fee         uint64   `json:"fee"`          // Optional. Transaction fee. Default to 0.
	Gas         uint64   `json:"gas"`          // Optional. Amount of gas. Default to 0.
	Collateral  uint64   `json:"collateral"`   // Required. Amount in GammaWei as the collateral
	Fund        uint64   `json:"fund"`         // Required. Amount in GammaWei to reserve.
	ResourceIds []string `json:"resource_ids"` // List of resource ID
	Duration    uint64   `json:"duration"`     // Optional. Number of blocks to lock the fund.
	Sequence    uint64   `json:"sequence"`     // Required. Sequence number of this transaction.
}

type ReserveFundResult struct {
	*ukulele.BroadcastRawTransactionResult
	ReserveSequence uint64 `json:"reserve_sequence"` // Sequence number of the reserved fund.
}

func (h *ThetaRPCHandler) ReserveFund(r *http.Request, args *ReserveFundArgs, result *ReserveFundResult) (err error) {
	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		return errors.New("No userid is passed in.")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return
	}

	if args.Duration == 0 {
		args.Duration = uint64(viper.GetInt64(util.CfgThetaDefaultReserveDurationSecs))
	}

	// Add minimal gas/fee.
	if args.Gas == 0 {
		args.Gas = 1
	}
	if args.Fee == 0 {
		args.Fee = 1
	}

	// Send from SendAccount
	input := ttypes.TxInput{
		Coins: ttypes.Coins{
			ThetaWei: big.NewInt(0),
			GammaWei: big.NewInt(0).SetUint64(args.Fund),
		},
		Sequence: args.Sequence,
		Address:  record.SaAddress,
	}

	var resourceIds []tcmn.Bytes
	for _, ridStr := range args.ResourceIds {
		resourceIds = append(resourceIds, []byte(ridStr))
	}

	collateral := ttypes.Coins{
		ThetaWei: big.NewInt(0),
		GammaWei: big.NewInt(0).SetUint64(args.Collateral),
	}
	tx := &ttypes.ReserveFundTx{
		Gas: args.Gas,
		Fee: ttypes.Coins{
			ThetaWei: ttypes.Zero,
			GammaWei: big.NewInt(0).SetUint64(args.Fee),
		},
		Source:      input,
		Collateral:  collateral,
		ResourceIDs: resourceIds,
		Duration:    args.Duration,
	}

	chainID := viper.GetString(util.CfgThetaChainId)

	sig, err := record.RaPrivateKey.Sign(tx.SignBytes(chainID))
	if err != nil {
		return err
	}
	tx.SetSignature(record.RaAddress, sig)

	raw, err := ttypes.TxToBytes(tx)
	if err != nil {
		return err
	}
	signedTx := hex.EncodeToString(raw)

	broadcastArgs := &ukulele.BroadcastRawTransactionArgs{
		TxBytes: signedTx,
	}
	resp, err := h.Client.Call("theta.BroadcastRawTransaction", broadcastArgs)
	if err != nil {
		return
	}
	if resp.Error != nil {
		err = resp.Error
		return
	}

	err = resp.GetObject(&result)

	// Set reserve_sequence to the tx sequence number.
	result.ReserveSequence = args.Sequence

	return
}

// --------------------------- Release -------------------------------

type ReleaseFundArgs struct {
	Fee             uint64 `json:"fee"`              // Optional. Transaction fee. Default to 0.
	Gas             uint64 `json:"gas"`              // Optional. Amount of gas. Default to 0.
	Sequence        uint64 `json:"sequence"`         // Required. Sequence number of this transaction.
	ReserveSequence uint64 `json:"reserve_sequence"` // Required. Sequence number of the fund to release.
}

type ReleaseFundResult struct {
	*ukulele.BroadcastRawTransactionResult
	ReserveSequence uint64 `json:"reserve_sequence"` // Sequence number of the reserved fund.
}

func (h *ThetaRPCHandler) ReleaseFund(r *http.Request, args *ReleaseFundArgs, result *ReleaseFundResult) (err error) {
	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		return errors.New("No userid is passed in.")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return
	}

	// Add minimal gas/fee.
	if args.Gas == 0 {
		args.Gas = 1
	}
	if args.Fee == 0 {
		args.Fee = 1
	}

	// Wrap and add signer
	input := ttypes.TxInput{
		Sequence: args.Sequence,
		Address:  record.SaAddress,
	}

	tx := &ttypes.ReleaseFundTx{
		Gas: args.Gas,
		Fee: ttypes.Coins{
			ThetaWei: ttypes.Zero,
			GammaWei: big.NewInt(0).SetUint64(args.Fee),
		},
		Source:          input,
		ReserveSequence: args.ReserveSequence,
	}

	chainID := viper.GetString(util.CfgThetaChainId)

	sig, err := record.RaPrivateKey.Sign(tx.SignBytes(chainID))
	if err != nil {
		return err
	}
	tx.SetSignature(record.SaAddress, sig)
	raw, err := ttypes.TxToBytes(tx)
	if err != nil {
		return err
	}
	signedTx := hex.EncodeToString(raw)

	broadcastArgs := &ukulele.BroadcastRawTransactionArgs{TxBytes: signedTx}
	resp, err := h.Client.Call("theta.BroadcastRawTransaction", broadcastArgs)
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

// --------------------------- CreateServicePayment -------------------------------

type CreateServicePaymentArgs struct {
	To              string `json:"to"`               // Required. Address to target account.
	Amount          uint64 `json:"amount"`           // Required. Amount of payment in GammaWei
	ResourceId      string `json:"resource_id"`      // Required. Resource ID the payment is for.
	PaymentSequence uint64 `json:"payment_sequence"` // Required. each on-chain settlement needs to increase the payment sequence by 1
	ReserveSequence uint64 `json:"reserve_sequence"` // Required. Sequence number of the fund to send.
}

type CreateServicePaymentResult struct {
	Payment string `json:"payment"` // Hex encoded half-signed payment tx bytes.
}

func (h *ThetaRPCHandler) CreateServicePayment(r *http.Request, args *CreateServicePaymentArgs, result *CreateServicePaymentResult) (err error) {
	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		return errors.New("No userid is passed in.")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return
	}

	if args.ResourceId == "" {
		return errors.New("No resource_id is provided")
	}

	if args.To == record.RaAddress.String() || args.To == record.SaAddress.String() {
		// You don't need to pay yourself.
		return
	}

	// Send from SendAccount
	address := record.SaAddress

	// Wrap and add signer
	sourceInput := ttypes.TxInput{}
	sourceInput.Address = address
	sourceInput.Coins = ttypes.Coins{
		ThetaWei: ttypes.Zero,
		GammaWei: big.NewInt(0).SetUint64(args.Amount),
	}

	targetAddress := tcmn.HexToAddress(args.To)
	targetInput := ttypes.TxInput{
		Address: targetAddress,
	}

	tx := &ttypes.ServicePaymentTx{
		Source:          sourceInput,
		Target:          targetInput,
		PaymentSequence: args.PaymentSequence,
		ReserveSequence: args.ReserveSequence,
		ResourceID:      []byte(args.ResourceId),
	}

	chainID := viper.GetString(util.CfgThetaChainId)

	sig, err := record.RaPrivateKey.Sign(tx.SignBytes(chainID))
	if err != nil {
		return err
	}
	tx.SetSourceSignature(sig)

	signedTx, err := ttypes.TxToBytes(tx)
	if err != nil {
		return err
	}
	result.Payment = hex.EncodeToString(signedTx)

	return
}

// --------------------------- SubmitServicePayment -------------------------------

type SubmitServicePaymentArgs struct {
	Fee      uint64 `json:"fee"`      // Optional. Transaction fee. Default to 0.
	Gas      uint64 `json:"gas"`      // Optional. Amount of gas. Default to 0.
	Payment  string `json:"payment"`  // Required. Hex of sender-signed payment stub.
	Sequence uint64 `json:"sequence"` // Required. Sequence number of this transaction.
}

func (h *ThetaRPCHandler) SubmitServicePayment(r *http.Request, args *SubmitServicePaymentArgs, result *ukulele.BroadcastRawTransactionResult) (err error) {
	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		return errors.New("No userid is passed in.")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return
	}

	// Receive into RecvAccount
	address := record.RaAddress

	input := ttypes.TxInput{
		Sequence: args.Sequence,
	}
	input.Address = address

	if args.Payment == "" {
		return errors.Errorf("Payment is empty")
	}

	paymentBytes, err := hex.DecodeString(args.Payment)
	if err != nil {
		return
	}

	tx, err := ttypes.TxFromBytes(paymentBytes)
	if err != nil {
		return
	}
	paymentTx := tx.(*ttypes.ServicePaymentTx)
	paymentTx.Target = input

	// Add minimal gas/fee.
	if args.Gas == 0 {
		args.Gas = 1
	}
	if args.Fee == 0 {
		args.Fee = 1
	}

	paymentTx.Gas = args.Gas
	paymentTx.Fee = ttypes.Coins{
		ThetaWei: ttypes.Zero,
		GammaWei: big.NewInt(0).SetUint64(args.Fee),
	}

	// Sign the tx
	chainID := viper.GetString(util.CfgThetaChainId)
	sig, err := record.RaPrivateKey.Sign(paymentTx.SignBytes(chainID))
	if err != nil {
		return err
	}
	paymentTx.SetTargetSignature(sig)

	raw, err := ttypes.TxToBytes(paymentTx)
	if err != nil {
		return err
	}
	signedTx := hex.EncodeToString(raw)

	broadcastArgs := &ukulele.BroadcastRawTransactionArgs{TxBytes: signedTx}
	resp, err := h.Client.Call("theta.BroadcastRawTransaction", broadcastArgs)
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

// --------------------------- InstantiateSplitContract -------------------------------

type InstantiateSplitContractArgs struct {
	Fee          uint64   `json:"fee"`          // Optional. Transaction fee. Default to 0.
	Gas          uint64   `json:"gas"`          // Optional. Amount of gas. Default to 0.
	ResourceId   string   `json:"resource_id"`  // Required. The resourceId.
	Initiator    string   `json:"initiator"`    // Required. Name of initiator account.
	Participants []string `json:"participants"` // Required. User IDs participating in the split.
	Percentages  []uint   `json:"percentages"`  // Required. The split percentage for each corresponding user.
	Duration     uint64   `json:"duration"`     // Optional. Number of blocks before the contract expires.
	Sequence     uint64   `json:"sequence"`     // Optional. Sequence number of this transaction.
}

func (h *ThetaRPCHandler) InstantiateSplitContract(r *http.Request, args *InstantiateSplitContractArgs, result *ukulele.BroadcastRawTransactionResult) (err error) {
	scope := r.Header.Get("X-Scope")
	if scope != "sliver_internal" {
		return errors.New("This API is sliver internal only")
	}

	if args.ResourceId == "" {
		return errors.New("No resource_id is passed in")
	}
	if args.Initiator == "" {
		return errors.New("No initiator is passed in")
	}
	if len(args.Participants) != len(args.Percentages) {
		return errors.New("Length of participants doesn't match with length of percentages")
	}
	// Add minimal gas/fee.
	if args.Gas == 0 {
		args.Gas = 1
	}
	if args.Fee == 0 {
		args.Fee = 1
	}

	initiator, err := h.KeyManager.FindByUserId(args.Initiator)
	if err != nil {
		return
	}
	// Use SendAccount to fund tx fee.
	initiatorAddress := initiator.SaAddress

	sequence, err := h.getSequence(initiator.SaAddress)

	initiatorInput := ttypes.TxInput{
		Address:  initiatorAddress,
		Sequence: sequence + 1,
	}

	splits := []ttypes.Split{}
	for idx, userid := range args.Participants {
		record, err := h.KeyManager.FindByUserId(userid)
		if err != nil {
			return err
		}
		address := record.RaAddress
		if err != nil {
			return err
		}

		percentage := args.Percentages[idx]
		splits = append(splits, ttypes.Split{
			Address:    address,
			Percentage: percentage,
		})
	}

	// duration := uint64(86400 * 365 * 10)
	if args.Duration == 0 {
		args.Duration = uint64(86400 * 365 * 10)
	}

	tx := &ttypes.SplitContractTx{
		Gas: args.Gas,
		Fee: ttypes.Coins{
			ThetaWei: ttypes.Zero,
			GammaWei: big.NewInt(0).SetUint64(args.Fee),
		},
		ResourceID: []byte(args.ResourceId),
		Initiator:  initiatorInput,
		Splits:     splits,
		Duration:   args.Duration,
	}

	chainID := viper.GetString(util.CfgThetaChainId)
	sig, err := initiator.RaPrivateKey.Sign(tx.SignBytes(chainID))
	if err != nil {
		return err
	}
	tx.SetSignature(initiator.RaAddress, sig)

	raw, err := ttypes.TxToBytes(tx)
	if err != nil {
		return err
	}
	signedTx := hex.EncodeToString(raw)

	broadcastArgs := &ukulele.BroadcastRawTransactionArgs{TxBytes: signedTx}
	resp, err := h.Client.Call("theta.BroadcastRawTransaction", broadcastArgs)
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

// ------------------ helpers ---------------------

func (h *ThetaRPCHandler) getSequence(address tcmn.Address) (sequence uint64, err error) {
	resp, err := h.Client.Call("theta.GetAccount", ukulele.GetAccountArgs{Address: address.String()})
	if err != nil {
		log.WithFields(log.Fields{"address": address, "error": err}).Error("Error in RPC call: theta.GetAccount()")
		return
	}
	result := &ukulele.GetAccountResult{}
	err = resp.GetObject(result)
	if err != nil {
		return
	}
	if result.Account == nil {
		log.WithFields(log.Fields{"address": address, "error": err}).Error("No result from RPC call: theta.GetAccount()")
		err = errors.New("Error in getting account sequence number")
		return 0, err
	}
	return result.Sequence, err
}
