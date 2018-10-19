package handler

// import (
// 	"bytes"
// 	"encoding/hex"
// 	"errors"
// 	"net/http"
// 	"testing"

// 	log "github.com/sirupsen/logrus"
// 	cmd "github.com/thetatoken/theta/cmd/thetacli/commands"
// 	crypto "github.com/thetatoken/theta/go-crypto"
// 	theta "github.com/thetatoken/theta/rpc"
// 	core_types "github.com/thetatoken/theta/tendermint/rpc/core/types"
// 	"github.com/thetatoken/theta/types"
// 	"github.com/thetatoken/vault/db"
// 	"github.com/thetatoken/vault/keymanager"
// 	rpcc "github.com/ybbus/jsonrpc"

// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/mock"
// )

// var logger = log.WithFields(log.Fields{"component": "server"})

// func getRecord() db.Record {
// 	saPrivKey := crypto.GenPrivKeyEd25519FromSecret([]byte("foo")).Wrap()
// 	saPubKey := saPrivKey.PubKey()
// 	saAddress := hex.EncodeToString(saPubKey.Address())

// 	raPrivKey := crypto.GenPrivKeyEd25519FromSecret([]byte("bar")).Wrap()
// 	raPubKey := raPrivKey.PubKey()
// 	raAddress := hex.EncodeToString(raPubKey.Address())

// 	return db.Record{
// 		UserID:       "alice",
// 		Type:         "ed25519",
// 		SaAddress:    saAddress,
// 		SaPubKey:     saPubKey,
// 		SaPrivateKey: saPrivKey,
// 		RaAddress:    raAddress,
// 		RaPubKey:     raPubKey,
// 		RaPrivateKey: raPrivKey,
// 	}
// }

// func TestSanity(t *testing.T) {
// 	record := getRecord()
// 	assert.Equal(t, record.SaAddress, hex.EncodeToString(record.SaPubKey.Address()))
// }

// func TestGetAccount(t *testing.T) {
// 	assert := assert.New(t)

// 	var mockRPC *MockRPCClient
// 	var mockKeyManager *keymanager.MockKeyManager
// 	var handler *ThetaRPCHandler
// 	var args *GetAccountArgs
// 	var result *GetAccountResult
// 	var err error

// 	// Should return account successfully.
// 	mockRPC = &MockRPCClient{}
// 	mockKeyManager = &keymanager.MockKeyManager{}
// 	handler = &ThetaRPCHandler{mockRPC, mockKeyManager}
// 	mockKeyManager.
// 		On("FindByUserId", "alice").
// 		Return(getRecord(), nil)
// 	mockRPC.
// 		On("Call", "theta.GetAccount", mock.Anything).
// 		Return(&rpcc.RPCResponse{Result: &types.Account{Balance: types.Coins{{Amount: 123}}}}, nil)
// 	args = &GetAccountArgs{}
// 	result = &GetAccountResult{}
// 	req, _ := http.NewRequest("", "", bytes.NewBufferString(""))
// 	req.Header.Add("X-Auth-User", "alice")
// 	err = handler.GetAccount(req, args, result)
// 	assert.Nil(err)
// 	assert.Equal(int64(123), result.SendAccount.Balance[0].Amount)

// 	// Should return error when RPC call fail
// 	mockRPC = &MockRPCClient{}
// 	mockKeyManager = &keymanager.MockKeyManager{}
// 	handler = &ThetaRPCHandler{mockRPC, mockKeyManager}
// 	mockKeyManager.
// 		On("FindByUserId", "alice").
// 		Return(getRecord(), nil)
// 	mockRPC.
// 		On("Call", "theta.GetAccount", mock.Anything).
// 		Return(nil, errors.New("rpc error"))
// 	result = &GetAccountResult{}
// 	req, _ = http.NewRequest("", "", bytes.NewBufferString(""))
// 	req.Header.Add("X-Auth-User", "alice")
// 	err = handler.GetAccount(req, args, result)
// 	assert.NotNil(err)

// 	// Should return error when key manager fail
// 	mockRPC = &MockRPCClient{}
// 	mockKeyManager = &keymanager.MockKeyManager{}
// 	handler = &ThetaRPCHandler{mockRPC, mockKeyManager}
// 	mockKeyManager.
// 		On("FindByUserId", "alice").
// 		Return(db.Record{}, errors.New("key manager error"))
// 	mockRPC.
// 		On("Call", "theta.GetAccount", mock.Anything).
// 		Return(&rpcc.RPCResponse{Result: &types.Account{Balance: types.Coins{{Amount: 123}}}}, nil)
// 	result = &GetAccountResult{}
// 	req, _ = http.NewRequest("", "", bytes.NewBufferString(""))
// 	req.Header.Add("X-Auth-User", "alice")
// 	err = handler.GetAccount(req, args, result)
// 	assert.NotNil(err)
// }

// func TestSign(t *testing.T) {
// 	assert := assert.New(t)

// 	record := getRecord()

// 	fromAddress, _ := hex.DecodeString("2674ae64cb5206b2afc6b6fbd0e5a65c025b5016")
// 	toAddress, _ := hex.DecodeString("EFEE576F3D668674BC73E007F6ABFA243311BD37")
// 	sendTx := &cmd.SendTx{
// 		Tx: &types.SendTx{
// 			Outputs: []types.TxOutput{{
// 				Address: toAddress,
// 				Coins:   types.Coins{{Amount: 123, Denom: "ThetaWei"}},
// 			}},
// 			Inputs: []types.TxInput{{
// 				Address:  fromAddress,
// 				Sequence: 1,
// 				Coins: types.Coins{{
// 					Amount: 123,
// 					Denom:  "ThetaWei",
// 				}},
// 			}},
// 			Fee: types.Coin{Amount: 4, Denom: "GammaWei"},
// 			Gas: 5,
// 		},
// 	}
// 	sendTx.SetChainID("test_chain_id")
// 	sendTx.AddSigner(record.SaPubKey)
// 	txBytes, err := keymanager.Sign(record.SaPubKey, record.SaPrivateKey, sendTx)

// 	expectedTxBytes := "12c7010805120c0a0847616d6d6157656910041a8e010a1468f7c99e3c68a61a538032508f36215538a3e325120c0a085468657461576569107b180122421240dc674cdbf46c5963e522c2acafad40edbf52f784dfbbc265ea4863b949aeafcb1a5731d01a044ed423c7be3aad7a4eae3d5317ccfbb9de8fc8e3f92806ae28052a22122034d26579dbb456693e540672cf922f52dde0d6532e35bf06be013a7c532f20e022240a14efee576f3d668674bc73e007f6abfa243311bd37120c0a085468657461576569107b"

// 	assert.Nil(err)
// 	assert.Equal(expectedTxBytes, hex.EncodeToString(txBytes))
// }

// func TestSend(t *testing.T) {
// 	assert := assert.New(t)

// 	var mockRPC *MockRPCClient
// 	var mockKeyManager *keymanager.MockKeyManager
// 	var handler *ThetaRPCHandler
// 	var args *SendArgs
// 	var result *theta.BroadcastRawTransactionResult
// 	var err error

// 	// Should send successfully.
// 	mockRPC = &MockRPCClient{}
// 	mockKeyManager = &keymanager.MockKeyManager{}
// 	handler = &ThetaRPCHandler{mockRPC, mockKeyManager}
// 	mockKeyManager.
// 		On("FindByUserId", "alice").
// 		Return(getRecord(), nil)
// 	expectedTxBytes := "12c7010805120c0a0847616d6d6157656910041a8e010a148a530213c34f0b8ee35fdc23c4169356f5f845db120c0a085468657461576569107b180122421240bc6044be4fe547b2f62fbd4e6ef0e10981ea4222950c32d0815ddbb7c8dd9174415b8eafe811f3adadca67a0cb36bb9e49d0c7d69ba1024b80a7f4dd9958610f2a221220cca45b406ad886997c34d86f341b212153a1cc813e3ad999a3e1516e82e8178122240a14efee576f3d668674bc73e007f6abfa243311bd37120c0a085468657461576569107b"
// 	resp := theta.BroadcastRawTransactionResult{&core_types.ResultBroadcastTxCommit{Height: 123}}
// 	mockRPC.
// 		On("Call", "theta.BroadcastRawTransaction", &theta.BroadcastRawTransactionArgs{TxBytes: expectedTxBytes}).
// 		Return(&rpcc.RPCResponse{Result: resp}, nil).Once()

// 	address, _ := hex.DecodeString("EFEE576F3D668674BC73E007F6ABFA243311BD37")
// 	args = &SendArgs{
// 		To: []types.TxOutput{{
// 			Address: address,
// 			Coins:   types.Coins{{Amount: 123, Denom: "ThetaWei"}},
// 		}},
// 		Sequence: 1,
// 		Fee:      types.Coin{Amount: 4, Denom: "GammaWei"},
// 		Gas:      5,
// 	}
// 	result = &theta.BroadcastRawTransactionResult{}
// 	req, _ := http.NewRequest("", "", bytes.NewBufferString(""))
// 	req.Header.Add("X-Auth-User", "alice")
// 	err = handler.Send(req, args, result)
// 	assert.Equal(123, result.Height)
// 	assert.Nil(err)
// 	mockRPC.AssertExpectations(t)

// 	// Should pass the error if RPC calls has error.
// 	mockRPC = &MockRPCClient{}
// 	mockKeyManager = &keymanager.MockKeyManager{}
// 	handler = &ThetaRPCHandler{mockRPC, mockKeyManager}
// 	mockKeyManager.
// 		On("FindByUserId", "alice").
// 		Return(getRecord(), nil)
// 	mockRPC.
// 		On("Call", "theta.BroadcastRawTransaction", &theta.BroadcastRawTransactionArgs{TxBytes: expectedTxBytes}).
// 		Return(&rpcc.RPCResponse{Error: &rpcc.RPCError{Code: 3000, Message: "Failed."}}, nil).Once()

// 	address, _ = hex.DecodeString("EFEE576F3D668674BC73E007F6ABFA243311BD37")
// 	args = &SendArgs{
// 		To: []types.TxOutput{{
// 			Address: address,
// 			Coins:   types.Coins{{Amount: 123, Denom: "ThetaWei"}},
// 		}},
// 		Sequence: 1,
// 		Fee:      types.Coin{Amount: 4, Denom: "GammaWei"},
// 		Gas:      5,
// 	}
// 	result = &theta.BroadcastRawTransactionResult{}
// 	req, _ = http.NewRequest("", "", bytes.NewBufferString(""))
// 	req.Header.Add("X-Auth-User", "alice")
// 	err = handler.Send(req, args, result)
// 	assert.NotNil(err)
// 	assert.Equal("3000: Failed.", err.Error())
// 	mockRPC.AssertExpectations(t)

// }
