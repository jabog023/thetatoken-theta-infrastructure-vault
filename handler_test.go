package vault

import (
	"bytes"
	"encoding/hex"
	"errors"
	"net/http"
	"testing"

	log "github.com/sirupsen/logrus"
	cmd "github.com/thetatoken/theta/cmd/thetacli/commands"
	crypto "github.com/thetatoken/theta/go-crypto"
	theta "github.com/thetatoken/theta/rpc"
	core_types "github.com/thetatoken/theta/tendermint/rpc/core/types"
	"github.com/thetatoken/theta/types"
	rpcc "github.com/ybbus/jsonrpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var logger = log.WithFields(log.Fields{"component": "server"})

func getRecord() Record {
	pubKeyBytes, _ := hex.DecodeString("1220355897db094c7aac8242e0bce8ae6a4db8b6c08b38bed3290ea3560a6515cc3b")
	privKeyBytes, _ := hex.DecodeString("12406f77b49c99cb22d63f84ffc7da54da0141b91f86627dda1c37a0bfe3eb1111e7355897db094c7aac8242e0bce8ae6a4db8b6c08b38bed3290ea3560a6515cc3b")
	pubKey := crypto.PubKey{}
	types.FromBytes(pubKeyBytes, &pubKey)
	privKey := crypto.PrivKey{}
	types.FromBytes(privKeyBytes, &privKey)
	return Record{
		UserID:     "alice",
		Type:       "ed25519",
		Address:    "2674ae64cb5206b2afc6b6fbd0e5a65c025b5016",
		PubKey:     pubKey,
		PrivateKey: privKey,
	}
}

func TestSanity(t *testing.T) {
	record := getRecord()
	assert.Equal(t, record.Address, hex.EncodeToString(record.PubKey.Address()))
}

func TestGetAccount(t *testing.T) {
	assert := assert.New(t)

	var mockRPC *MockRPCClient
	var mockKeyManager *MockKeyManager
	var handler *ThetaRPCHandler
	var args *GetAccountArgs
	var result *theta.GetAccountResult
	var err error

	// Should return account successfully.
	mockRPC = &MockRPCClient{}
	mockKeyManager = &MockKeyManager{}
	handler = &ThetaRPCHandler{mockRPC, mockKeyManager, logger}
	mockKeyManager.
		On("FindByUserId", "alice").
		Return(getRecord(), nil)
	mockRPC.
		On("Call", "theta.GetAccount", mock.Anything).
		Return(&rpcc.RPCResponse{Result: &types.Account{Balance: types.Coins{{Amount: 123}}}}, nil)
	args = &GetAccountArgs{}
	result = &theta.GetAccountResult{}
	req, _ := http.NewRequest("", "", bytes.NewBufferString(""))
	req.Header.Add("X-Auth-User", "alice")
	err = handler.GetAccount(req, args, result)
	assert.Nil(err)
	assert.Equal(int64(123), result.Balance[0].Amount)

	// Should return error when RPC call fail
	mockRPC = &MockRPCClient{}
	mockKeyManager = &MockKeyManager{}
	handler = &ThetaRPCHandler{mockRPC, mockKeyManager, logger}
	mockKeyManager.
		On("FindByUserId", "alice").
		Return(getRecord(), nil)
	mockRPC.
		On("Call", "theta.GetAccount", mock.Anything).
		Return(nil, errors.New("rpc error"))
	result = &theta.GetAccountResult{}
	req, _ = http.NewRequest("", "", bytes.NewBufferString(""))
	req.Header.Add("X-Auth-User", "alice")
	err = handler.GetAccount(req, args, result)
	assert.NotNil(err)

	// Should return error when key manager fail
	mockRPC = &MockRPCClient{}
	mockKeyManager = &MockKeyManager{}
	handler = &ThetaRPCHandler{mockRPC, mockKeyManager, logger}
	mockKeyManager.
		On("FindByUserId", "alice").
		Return(Record{}, errors.New("key manager error"))
	mockRPC.
		On("Call", "theta.GetAccount", mock.Anything).
		Return(&rpcc.RPCResponse{Result: &types.Account{Balance: types.Coins{{Amount: 123}}}}, nil)
	result = &theta.GetAccountResult{}
	req, _ = http.NewRequest("", "", bytes.NewBufferString(""))
	req.Header.Add("X-Auth-User", "alice")
	err = handler.GetAccount(req, args, result)
	assert.NotNil(err)
}

func TestSign(t *testing.T) {
	assert := assert.New(t)

	record := getRecord()

	fromAddress, _ := hex.DecodeString("2674ae64cb5206b2afc6b6fbd0e5a65c025b5016")
	toAddress, _ := hex.DecodeString("EFEE576F3D668674BC73E007F6ABFA243311BD37")
	sendTx := &cmd.SendTx{
		Tx: &types.SendTx{
			Outputs: []types.TxOutput{{
				Address: toAddress,
				Coins:   types.Coins{{Amount: 123, Denom: "ThetaWei"}},
			}},
			Inputs: []types.TxInput{{
				Address:  fromAddress,
				Sequence: 1,
				Coins: types.Coins{{
					Amount: 123,
					Denom:  "ThetaWei",
				}},
			}},
			Fee: types.Coin{Amount: 4, Denom: "GammaWei"},
			Gas: 5,
		},
	}
	sendTx.SetChainID("test_chain_id")
	sendTx.AddSigner(record.PubKey)
	txBytes, err := Sign(record.PubKey, record.PrivateKey, sendTx)

	expectedTxBytes, _ := hex.DecodeString("12C7010805120C0A0847616D6D6157656910041A8E010A142674AE64CB5206B2AFC6B6FBD0E5A65C025B5016120C0A085468657461576569107B18012242124043F1E91C42DF3235A2849C886716A1749B3C563FC62BD38AF647CC716730DB32DCEA959C263C6A74CDC79DEA289D2DEA0A83C063230748391EA32C79EBE6300B2A221220355897DB094C7AAC8242E0BCE8AE6A4DB8B6C08B38BED3290EA3560A6515CC3B22240A14EFEE576F3D668674BC73E007F6ABFA243311BD37120C0A085468657461576569107B")

	assert.Nil(err)
	assert.Equal(expectedTxBytes, txBytes)

	assert.Equal(bytes.Compare(expectedTxBytes, txBytes), 0)

}

func TestSend(t *testing.T) {
	assert := assert.New(t)

	var mockRPC *MockRPCClient
	var mockKeyManager *MockKeyManager
	var handler *ThetaRPCHandler
	var args *SendArgs
	var result *theta.BroadcastRawTransactionResult
	var err error

	// Should send successfully.
	mockRPC = &MockRPCClient{}
	mockKeyManager = &MockKeyManager{}
	handler = &ThetaRPCHandler{mockRPC, mockKeyManager, logger}
	mockKeyManager.
		On("FindByUserId", "alice").
		Return(getRecord(), nil)
	expectedTxBytes := "12c7010805120c0a0847616d6d6157656910041a8e010a142674ae64cb5206b2afc6b6fbd0e5a65c025b5016120c0a085468657461576569107b180122421240efaacebb519466cc7f60598b5fe13e01b25c9bded5c33a60c0bbf61c4ae23fa8eb91de01d4fd3d1bdc88c29fdff33dff61b35769e4696f2c55789290b0d5420e2a221220355897db094c7aac8242e0bce8ae6a4db8b6c08b38bed3290ea3560a6515cc3b22240a14efee576f3d668674bc73e007f6abfa243311bd37120c0a085468657461576569107b"
	resp := theta.BroadcastRawTransactionResult{&core_types.ResultBroadcastTxCommit{Height: 123}}
	mockRPC.
		On("Call", "theta.BroadcastRawTransaction", &theta.BroadcastRawTransactionArgs{TxBytes: expectedTxBytes}).
		Return(&rpcc.RPCResponse{Result: resp}, nil).Once()

	address, _ := hex.DecodeString("EFEE576F3D668674BC73E007F6ABFA243311BD37")
	args = &SendArgs{
		To: []types.TxOutput{{
			Address: address,
			Coins:   types.Coins{{Amount: 123, Denom: "ThetaWei"}},
		}},
		Sequence: 1,
		Fee:      types.Coin{Amount: 4, Denom: "GammaWei"},
		Gas:      5,
	}
	result = &theta.BroadcastRawTransactionResult{}
	req, _ := http.NewRequest("", "", bytes.NewBufferString(""))
	req.Header.Add("X-Auth-User", "alice")
	err = handler.Send(req, args, result)
	assert.Equal(123, result.Height)
	assert.Nil(err)
	mockRPC.AssertExpectations(t)

	// Should pass the error if RPC calls has error.
	mockRPC = &MockRPCClient{}
	mockKeyManager = &MockKeyManager{}
	handler = &ThetaRPCHandler{mockRPC, mockKeyManager, logger}
	mockKeyManager.
		On("FindByUserId", "alice").
		Return(getRecord(), nil)
	mockRPC.
		On("Call", "theta.BroadcastRawTransaction", &theta.BroadcastRawTransactionArgs{TxBytes: expectedTxBytes}).
		Return(&rpcc.RPCResponse{Error: &rpcc.RPCError{Code: 3000, Message: "Failed."}}, nil).Once()

	address, _ = hex.DecodeString("EFEE576F3D668674BC73E007F6ABFA243311BD37")
	args = &SendArgs{
		To: []types.TxOutput{{
			Address: address,
			Coins:   types.Coins{{Amount: 123, Denom: "ThetaWei"}},
		}},
		Sequence: 1,
		Fee:      types.Coin{Amount: 4, Denom: "GammaWei"},
		Gas:      5,
	}
	result = &theta.BroadcastRawTransactionResult{}
	req, _ = http.NewRequest("", "", bytes.NewBufferString(""))
	req.Header.Add("X-Auth-User", "alice")
	err = handler.Send(req, args, result)
	assert.NotNil(err)
	assert.Equal("3000: Failed.", err.Error())
	mockRPC.AssertExpectations(t)

}
