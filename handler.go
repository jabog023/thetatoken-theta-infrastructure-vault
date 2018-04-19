package vault

import (
	"encoding/hex"
	"errors"
	"net/http"

	cmd "github.com/thetatoken/theta/cmd/thetacli/commands"
	theta "github.com/thetatoken/theta/rpc"
	"github.com/thetatoken/theta/types"
	rpcc "github.com/ybbus/jsonrpc"
)

type RPCClient interface {
	Call(method string, params ...interface{}) (*rpcc.RPCResponse, error)
}
type ThetaRPCHandler struct {
	Client     RPCClient
	KeyManager KeyManager
}

// ------------------------------- GetAccount -----------------------------------

type GetAccountArgs struct{}

func (h *ThetaRPCHandler) GetAccount(r *http.Request, args *GetAccountArgs, result *theta.GetAccountResult) (err error) {
	userid := r.Header.Get("X-Auth-User")
	if userid == "" {
		return errors.New("No userid is passed in.")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return
	}
	resp, err := h.Client.Call("theta.GetAccount", theta.GetAccountArgs{Address: record.Address})
	if err != nil {
		return
	}
	err = resp.GetObject(result)
	result.Address = record.Address
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
		return errors.New("No userid is passed in.")
	}

	record, err := h.KeyManager.FindByUserId(userid)
	if err != nil {
		return
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

	input.Address, err = hex.DecodeString(record.Address)
	if err != nil {
		return
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

	send.AddSigner(record.PubKey)
	txBytes, err := Sign(record.PubKey, record.PrivateKey, send)

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

// --------------------------- BroadcastRawTransaction ----------------------------
// --------------------------- Reserve -------------------------------
// --------------------------- Release -------------------------------
// --------------------------- CreateServicePayment -------------------------------
// --------------------------- SubmitServicePayment -------------------------------
