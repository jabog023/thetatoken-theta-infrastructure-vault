package vault

import (
	"net/http"

	theta "github.com/thetatoken/theta/rpc"
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

type GetAccountArgs struct {
	UserId string
}

// type GetAccountResult struct {
// 	*types.Account
// 	Address string `json:"address"`
// }

func (h *ThetaRPCHandler) GetAccount(r *http.Request, args *GetAccountArgs, result *theta.GetAccountResult) (err error) {
	record, err := h.KeyManager.FindByUserId(args.UserId)
	if err != nil {
		return
	}
	resp, err := h.Client.Call("theta.GetAccount", theta.GetAccountArgs{Address: record.Address})
	if err != nil {
		return
	}
	err = resp.GetObject(result)
	return
}

// // ------------------------------- CreateAccount -----------------------------------

// type CreateAccountArgs struct {
// 	Name       string `json:"name"`
// 	Passphrase string `json:"passphrase"`
// 	Type       string `json:"type"`
// }

// type CreateAccountResult struct {
// 	Key  keys.Info `json:"key"`
// 	Seed string    `json:"seed"`
// }

// const DefaultType = "ed25519"

// func (h *ThetaRPCHandler) CreateAccount(r *http.Request, args *CreateAccountArgs, result *CreateAccountResult) (err error) {
// 	if args.Name == "" {
// 		return errors.New("You must provide a name for the account")
// 	}
// 	var keyType string
// 	if args.Type != "" {
// 		keyType = args.Type
// 	} else {
// 		keyType = DefaultType
// 	}
// 	info, seed, err := context.GetKeyManager().Create(args.Name, args.Passphrase, keyType)
// 	if err != nil {
// 		return
// 	}
// 	result.Key = info
// 	result.Seed = seed
// 	return
// }
