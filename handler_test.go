package vault

import (
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	crypto "github.com/tendermint/go-crypto"
	cmd "github.com/thetatoken/theta/cmd/thetacli/commands"
	theta "github.com/thetatoken/theta/rpc"
	"github.com/thetatoken/theta/types"
	rpcc "github.com/ybbus/jsonrpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
	handler = &ThetaRPCHandler{mockRPC, mockKeyManager}
	mockKeyManager.
		On("FindByUserId", "alice").
		Return(getRecord(), nil)
	mockRPC.
		On("Call", "theta.GetAccount", mock.Anything).
		Return(&rpcc.RPCResponse{Result: &types.Account{Balance: types.Coins{{Amount: 123}}}}, nil)
	args = &GetAccountArgs{UserId: "alice"}
	result = &theta.GetAccountResult{}
	err = handler.GetAccount(nil, args, result)
	assert.Nil(err)
	assert.Equal(int64(123), result.Balance[0].Amount)

	// Should return error when RPC call fail
	mockRPC = &MockRPCClient{}
	mockKeyManager = &MockKeyManager{}
	handler = &ThetaRPCHandler{mockRPC, mockKeyManager}
	mockKeyManager.
		On("FindByUserId", "alice").
		Return(getRecord(), nil)
	mockRPC.
		On("Call", "theta.GetAccount", mock.Anything).
		Return(nil, errors.New("rpc error"))
	result = &theta.GetAccountResult{}
	err = handler.GetAccount(nil, args, result)
	assert.NotNil(err)

	// Should return error when key manager fail
	mockRPC = &MockRPCClient{}
	mockKeyManager = &MockKeyManager{}
	handler = &ThetaRPCHandler{mockRPC, mockKeyManager}
	mockKeyManager.
		On("FindByUserId", "alice").
		Return(Record{}, errors.New("key manager error"))
	mockRPC.
		On("Call", "theta.GetAccount", mock.Anything).
		Return(&rpcc.RPCResponse{Result: &types.Account{Balance: types.Coins{{Amount: 123}}}}, nil)
	result = &theta.GetAccountResult{}
	err = handler.GetAccount(nil, args, result)
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

	expectedTxBytes, _ := hex.DecodeString("12c7010805120c0a0847616d6d6157656910041a8e010a142674ae64cb5206b2afc6b6fbd0e5a65c025b5016120c0a085468657461576569107b1801224212406c6dbdf253f520028743823c395cdb03dbf7ed399a8e6b251b5ac11d2ee1cb52c92380474884d281933288b7e7249954c8d595c94d85c19d9083c4307b811a062a221220355897db094c7aac8242e0bce8ae6a4db8b6c08b38bed3290ea3560a6515cc3b22240a14efee576f3d668674bc73e007f6abfa243311bd37120c0a085468657461576569107b")

	assert.Nil(err)
	assert.Equal(expectedTxBytes, txBytes)

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
	handler = &ThetaRPCHandler{mockRPC, mockKeyManager}
	mockKeyManager.
		On("FindByUserId", "alice").
		Return(getRecord(), nil)
	expectedTxBytes := "12c7010805120c0a0847616d6d6157656910041a8e010a142674ae64cb5206b2afc6b6fbd0e5a65c025b5016120c0a085468657461576569107b1801224212406c6dbdf253f520028743823c395cdb03dbf7ed399a8e6b251b5ac11d2ee1cb52c92380474884d281933288b7e7249954c8d595c94d85c19d9083c4307b811a062a221220355897db094c7aac8242e0bce8ae6a4db8b6c08b38bed3290ea3560a6515cc3b22240a14efee576f3d668674bc73e007f6abfa243311bd37120c0a085468657461576569107b"
	mockRPC.
		On("Call", "theta.BroadcastRawTransaction", &theta.BroadcastRawTransactionArgs{TxBytes: expectedTxBytes}).
		Return(&rpcc.RPCResponse{Result: nil}, nil).Once()

	address, _ := hex.DecodeString("EFEE576F3D668674BC73E007F6ABFA243311BD37")
	args = &SendArgs{
		UserId: "alice",
		To: []types.TxOutput{{
			Address: address,
			Coins:   types.Coins{{Amount: 123, Denom: "ThetaWei"}},
		}},
		Sequence: 1,
		Fee:      types.Coin{Amount: 4, Denom: "GammaWei"},
		Gas:      5,
	}
	result = &theta.BroadcastRawTransactionResult{}
	err = handler.Send(nil, args, result)
	fmt.Printf("%v\n", result)
	assert.Nil(err)
	mockRPC.AssertExpectations(t)
}
