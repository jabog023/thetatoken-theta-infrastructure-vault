package handler

import (
	"encoding/hex"
	"net/http"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	cmd "github.com/thetatoken/theta/cmd/thetacli/commands"
	"github.com/thetatoken/theta/common"
	theta "github.com/thetatoken/theta/rpc"
	"github.com/thetatoken/theta/types"
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
	UserID      string                  `json:"user_id"`
	SendAccount *theta.GetAccountResult `json:"send_account"` // Account to send from
	RecvAccount *theta.GetAccountResult `json:"recv_account"` // Account to receive into
}

// getAccount is a helper function to query account from blockchain
func (h *ThetaRPCHandler) getAccount(address string) (*theta.GetAccountResult, error) {
	resp, err := h.Client.Call("theta.GetAccount", theta.GetAccountArgs{Address: address})
	if err != nil {
		return nil, errors.Wrap(err, "Error in RPC call")
	}
	result := &theta.GetAccountResult{}
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
	sendAccount, err := h.getAccount(record.SaAddress)
	if err != nil {
		return errors.Wrapf(err, "Failed to find sendAccount for %v", userid)
	}
	result.SendAccount = sendAccount

	// Load RecvAccount
	recvAccount, err := h.getAccount(record.RaAddress)
	if err != nil {
		return errors.Wrapf(err, "Failed to find recvAccount for %v", userid)
	}
	result.RecvAccount = recvAccount

	return
}

// ------------------------------- Send -----------------------------------

type SendArgs struct {
	To       []types.TxOutput `json:"to"`       // Required. Outputs including addresses and amount.
	Fee      types.Coin       `json:"fee"`      // Optional. Transaction fee. Default to 0.
	Gas      int64            `json:"gas"`      // Optional. Amount of gas. Default to 0.
	Sequence int              `json:"sequence"` // Required. Sequence number of this transaction.
}

func (h *ThetaRPCHandler) Send(r *http.Request, args *SendArgs, result *theta.BroadcastRawTransactionResult) (err error) {
	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		return errors.New("No userid is passed in")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return
	}

	// Add minimal gas/fee.
	if args.Gas == 0 {
		args.Gas = 1
	}
	if args.Fee.Amount == 0 {
		args.Fee = types.Coin{Denom: "GammaWei", Amount: 1}
	}

	// Wrap and add signer
	total := types.Coins{}
	for _, out := range args.To {
		total = total.Plus(out.Coins)
	}
	input := types.TxInput{
		Coins:    total,
		Sequence: args.Sequence,
	}

	// We only allow user to spend his RecvAccount
	input.Address, err = hex.DecodeString(record.RaAddress)
	if err != nil {
		return errors.Wrap(err, "Failed to decode address")
	}
	inputs := []types.TxInput{input}
	tx := &types.SendTx{
		Gas:     args.Gas,
		Fee:     args.Fee,
		Inputs:  inputs,
		Outputs: args.To,
	}
	send := &cmd.SendTx{
		Tx: tx,
	}
	send.SetChainID(viper.GetString(util.CfgThetaChainId))
	send.AddSigner(record.RaPubKey)

	txBytes, err := keymanager.Sign(record.RaPubKey, record.RaPrivateKey, send)
	if err != nil {
		return errors.Wrap(err, "Failed to sign tx")
	}

	broadcastArgs := &theta.BroadcastRawTransactionArgs{TxBytes: hex.EncodeToString(txBytes)}
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

func (h *ThetaRPCHandler) BroadcastRawTransaction(r *http.Request, args *theta.BroadcastRawTransactionArgs, result *theta.BroadcastRawTransactionResult) (err error) {
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
	Fee         types.Coin `json:"fee"`          // Optional. Transaction fee. Default to 0.
	Gas         int64      `json:"gas"`          // Optional. Amount of gas. Default to 0.
	Collateral  int64      `json:"collateral"`   // Required. Amount in GammaWei as the collateral
	Fund        int64      `json:"fund"`         // Required. Amount in GammaWei to reserve.
	ResourceIds []string   `json:"resource_ids"` // List of resource ID
	Duration    uint64     `json:"duration"`     // Optional. Number of blocks to lock the fund.
	Sequence    int        `json:"sequence"`     // Required. Sequence number of this transaction.
}

func (h *ThetaRPCHandler) ReserveFund(r *http.Request, args *ReserveFundArgs, result *theta.ReserveFundResult) (err error) {
	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		return errors.New("No userid is passed in.")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return
	}

	if args.Duration == 0 {
		args.Duration = common.MinimumFundReserveDuration
	}
	// Add minimal gas/fee.
	if args.Gas == 0 {
		args.Gas = 1
	}
	if args.Fee.Amount == 0 {
		args.Fee = types.Coin{Denom: "GammaWei", Amount: 1}
	}

	// Wrap and add signer
	// Send from SendAccount
	address, err := hex.DecodeString(record.SaAddress)
	if err != nil {
		return
	}
	input := types.TxInput{
		Coins:    types.Coins{{Denom: common.DenomGammaWei, Amount: args.Fund}},
		Sequence: args.Sequence,
		Address:  address,
	}

	var resourceIds [][]byte
	for _, ridStr := range args.ResourceIds {
		resourceIds = append(resourceIds, []byte(ridStr))
	}

	collateral := types.Coins{{
		Denom:  common.DenomGammaWei,
		Amount: args.Collateral,
	}}
	tx := &types.ReserveFundTx{
		Gas:         args.Gas,
		Fee:         args.Fee,
		Source:      input,
		Collateral:  collateral,
		ResourceIds: resourceIds,
		Duration:    args.Duration,
	}

	reserveTx := &cmd.ReserveFundTx{
		Tx: tx,
	}
	reserveTx.SetChainID(viper.GetString(util.CfgThetaChainId))
	reserveTx.AddSigner(record.SaPubKey)
	txBytes, err := keymanager.Sign(record.SaPubKey, record.SaPrivateKey, reserveTx)
	if err != nil {
		return
	}

	broadcastArgs := &theta.BroadcastRawTransactionArgs{TxBytes: hex.EncodeToString(txBytes)}
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
	Fee             types.Coin `json:"fee"`              // Optional. Transaction fee. Default to 0.
	Gas             int64      `json:"gas"`              // Optional. Amount of gas. Default to 0.
	Sequence        int        `json:"sequence"`         // Required. Sequence number of this transaction.
	ReserveSequence int        `json:"reserve_sequence"` // Required. Sequence number of the fund to release.
}

func (h *ThetaRPCHandler) ReleaseFund(r *http.Request, args *ReleaseFundArgs, result *theta.ReleaseFundResult) (err error) {
	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		return errors.New("No userid is passed in.")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return
	}

	address, err := hex.DecodeString(record.SaAddress)
	if err != nil {
		return
	}

	// Add minimal gas/fee.
	if args.Gas == 0 {
		args.Gas = 1
	}
	if args.Fee.Amount == 0 {
		args.Fee = types.Coin{Denom: "GammaWei", Amount: 1}
	}

	// Wrap and add signer
	input := types.TxInput{
		Sequence: args.Sequence,
		Address:  address,
	}

	tx := &types.ReleaseFundTx{
		Gas:             args.Gas,
		Fee:             args.Fee,
		Source:          input,
		ReserveSequence: args.ReserveSequence,
	}
	releaseTx := &cmd.ReleaseFundTx{
		Tx: tx,
	}
	releaseTx.SetChainID(viper.GetString(util.CfgThetaChainId))
	releaseTx.AddSigner(record.SaPubKey)
	txBytes, err := keymanager.Sign(record.SaPubKey, record.SaPrivateKey, releaseTx)
	if err != nil {
		return
	}

	broadcastArgs := &theta.BroadcastRawTransactionArgs{TxBytes: hex.EncodeToString(txBytes)}
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
	Amount          int64  `json:"amount"`           // Required. Amount of payment in GammaWei
	ResourceId      string `json:"resource_id"`      // Required. Resource ID the payment is for.
	PaymentSequence int    `json:"payment_sequence"` // Required. each on-chain settlement needs to increase the payment sequence by 1
	ReserveSequence int    `json:"reserve_sequence"` // Required. Sequence number of the fund to send.
}

func (h *ThetaRPCHandler) CreateServicePayment(r *http.Request, args *CreateServicePaymentArgs, result *theta.CreateServicePaymentResult) (err error) {
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

	if args.To == record.RaAddress || args.To == record.SaAddress {
		// You don't need to pay yourself.
		return
	}

	// Send from SendAccount
	address, err := hex.DecodeString(record.SaAddress)
	if err != nil {
		return
	}

	// Wrap and add signer
	sourceInput := types.TxInput{}
	sourceInput.Address = address
	sourceInput.Coins = types.Coins{types.Coin{
		Denom:  common.DenomGammaWei,
		Amount: args.Amount,
	}}

	targetAddress, err := hex.DecodeString(args.To)
	if err != nil {
		return
	}
	targetInput := types.TxInput{
		Address: targetAddress,
	}

	tx := &types.ServicePaymentTx{
		Source:          sourceInput,
		Target:          targetInput,
		PaymentSequence: args.PaymentSequence,
		ReserveSequence: args.ReserveSequence,
		ResourceId:      []byte(args.ResourceId),
	}
	paymentTxWrap := (&cmd.ServicePaymentTx{
		Tx: tx,
	}).SenderSignable()
	paymentTxWrap.SetChainID(viper.GetString(util.CfgThetaChainId))
	paymentTxWrap.AddSigner(record.SaPubKey)

	txBytes, err := keymanager.Sign(record.SaPubKey, record.SaPrivateKey, paymentTxWrap)
	if err != nil {
		return
	}

	result.Payment = hex.EncodeToString(txBytes)

	return
}

// --------------------------- SubmitServicePayment -------------------------------

type SubmitServicePaymentArgs struct {
	Fee      types.Coin `json:"fee"`      // Optional. Transaction fee. Default to 0.
	Gas      int64      `json:"gas"`      // Optional. Amount of gas. Default to 0.
	Payment  string     `json:"payment"`  // Required. Hex of sender-signed payment stub.
	Sequence int        `json:"sequence"` // Required. Sequence number of this transaction.
}

func (h *ThetaRPCHandler) SubmitServicePayment(r *http.Request, args *SubmitServicePaymentArgs, result *theta.SubmitServicePaymentResult) (err error) {
	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		return errors.New("No userid is passed in.")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return
	}

	// Receive into RecvAccount
	address, err := hex.DecodeString(record.RaAddress)
	if err != nil {
		return
	}

	// Wrap and add signer
	input := types.TxInput{
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

	tx, err := types.TxFromBytes(paymentBytes)
	if err != nil {
		return
	}
	paymentTx := tx.(*types.ServicePaymentTx)
	paymentTx.Target = input

	// Add minimal gas/fee.
	if paymentTx.Gas == 0 {
		paymentTx.Gas = 1
	}
	if paymentTx.Fee.Amount == 0 {
		paymentTx.Fee = types.Coin{Denom: "GammaWei", Amount: 1}
	}

	paymentTxWrap := (&cmd.ServicePaymentTx{
		Tx: paymentTx,
	}).ReceiverSignable()
	paymentTxWrap.SetChainID(viper.GetString(util.CfgThetaChainId))
	paymentTxWrap.AddSigner(record.RaPubKey)

	// Sign the tx
	txBytes, err := keymanager.Sign(record.RaPubKey, record.RaPrivateKey, paymentTxWrap)
	if err != nil {
		return
	}

	broadcastArgs := &theta.BroadcastRawTransactionArgs{TxBytes: hex.EncodeToString(txBytes)}
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
	Fee          types.Coin `json:"fee"`          // Optional. Transaction fee. Default to 0.
	Gas          int64      `json:"gas"`          // Optional. Amount of gas. Default to 0.
	ResourceId   string     `json:"resource_id"`  // Required. The resourceId.
	Initiator    string     `json:"initiator"`    // Required. Name of initiator account.
	Participants []string   `json:"participants"` // Required. User IDs participating in the split.
	Percentages  []uint     `json:"percentages"`  // Required. The split percentage for each corresponding user.
	Duration     uint64     `json:"duration"`     // Optional. Number of blocks before the contract expires.
	Sequence     int        `json:"sequence"`     // Optional. Sequence number of this transaction.
}

func (h *ThetaRPCHandler) InstantiateSplitContract(r *http.Request, args *InstantiateSplitContractArgs, result *theta.InstantiateSplitContractArgsResult) (err error) {
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
		return errors.New("Length of participents doesn't match with length of percentages")
	}
	// Add minimal gas/fee.
	if args.Gas == 0 {
		args.Gas = 1
	}
	if args.Fee.Amount == 0 {
		args.Fee = types.Coin{Denom: "GammaWei", Amount: 1}
	}

	initiator, err := h.KeyManager.FindByUserId(args.Initiator)
	if err != nil {
		return
	}
	// Use SendAccount to fund tx fee.
	initiatorAddress, err := hex.DecodeString(initiator.SaAddress)
	if err != nil {
		return
	}

	sequence, err := h.getSequence(initiator.SaAddress)

	initiatorInput := types.TxInput{
		Address:  initiatorAddress,
		Sequence: sequence + 1,
	}

	splits := []types.Split{}
	for idx, userid := range args.Participants {
		record, err := h.KeyManager.FindByUserId(userid)
		if err != nil {
			return err
		}
		address, err := hex.DecodeString(record.SaAddress)
		if err != nil {
			return err
		}

		percentage := args.Percentages[idx]
		splits = append(splits, types.Split{
			Address:    address,
			Percentage: percentage,
		})
	}

	duration := uint64(86400 * 365 * 10)

	tx := &types.SplitContractTx{
		ResourceId: []byte(args.ResourceId),
		Initiator:  initiatorInput,
		Splits:     splits,
		Duration:   duration,
	}

	// Wrap and add signer
	splitContractTx := (&cmd.SplitContractTx{
		Tx: tx,
	})

	splitContractTx.SetChainID(viper.GetString(util.CfgThetaChainId))
	splitContractTx.AddSigner(initiator.SaPubKey)

	txBytes, err := keymanager.Sign(initiator.SaPubKey, initiator.SaPrivateKey, splitContractTx)
	if err != nil {
		return
	}

	broadcastArgs := &theta.BroadcastRawTransactionArgs{TxBytes: hex.EncodeToString(txBytes)}
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

func (h *ThetaRPCHandler) getSequence(address string) (sequence int, err error) {
	resp, err := h.Client.Call("theta.GetAccount", theta.GetAccountArgs{Address: address})
	if err != nil {
		log.WithFields(log.Fields{"address": address, "error": err}).Error("Error in RPC call: theta.GetAccount()")
		return
	}
	result := &theta.GetAccountResult{}
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
