package vault

import (
	"encoding/base64"
	"errors"
	"testing"

	theta "github.com/thetatoken/theta/rpc"
	"github.com/thetatoken/theta/types"
	rpcc "github.com/ybbus/jsonrpc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func getRecord() Record {
	pubkey, _ := base64.StdEncoding.DecodeString("AWFYTr1kJkZrPB7DnRtlDtq3tXr+B1ruU44D174zbYYD=g5FW")
	privkey, _ := base64.StdEncoding.DecodeString("EVRyaYMpimPW1Lp6QTt5G2ALnbEeBtgbDF4QQZZKttqEsldmdCu7YL95LZtjfAUdEsfluY9GtTMglVZyLCx3w8mJxqR1+3PBEsD7IckSUeQpoGnQDoi/5zMYlAKfe1mmbRxYOU3k6j/B=MBb1")
	return Record{
		UserID:     "teddy",
		Type:       "ed25519",
		Address:    "57D97324DE04E6FD3730D822A1A3DEEBE59E5FC1",
		PubKey:     pubkey,
		PrivateKey: privkey,
	}
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
		On("FindByUserId", "teddy").
		Return(getRecord(), nil)
	mockRPC.
		On("Call", "theta.GetAccount", mock.Anything).
		Return(&rpcc.RPCResponse{Result: &types.Account{Balance: types.Coins{{Amount: 123}}}}, nil)
	args = &GetAccountArgs{UserId: "teddy"}
	result = &theta.GetAccountResult{}
	err = handler.GetAccount(nil, args, result)
	assert.Nil(err)
	assert.Equal(int64(123), result.Balance[0].Amount)

	// Should return error when RPC call fail
	mockRPC = &MockRPCClient{}
	mockKeyManager = &MockKeyManager{}
	handler = &ThetaRPCHandler{mockRPC, mockKeyManager}
	mockKeyManager.
		On("FindByUserId", "teddy").
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
		On("FindByUserId", "teddy").
		Return(Record{}, errors.New("key manager error"))
	mockRPC.
		On("Call", "theta.GetAccount", mock.Anything).
		Return(&rpcc.RPCResponse{Result: &types.Account{Balance: types.Coins{{Amount: 123}}}}, nil)
	result = &theta.GetAccountResult{}
	err = handler.GetAccount(nil, args, result)
	assert.NotNil(err)
}
