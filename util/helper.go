package util

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"github.com/thetatoken/ukulele/common"
	ukulele "github.com/thetatoken/ukulele/rpc"
	rpcc "github.com/ybbus/jsonrpc"
)

type RPCClient interface {
	Call(method string, params ...interface{}) (*rpcc.RPCResponse, error)
}

func GetSequence(client RPCClient, address common.Address) (sequence uint64, err error) {
	resp, err := client.Call("theta.GetAccount", ukulele.GetAccountArgs{Address: address.String()})
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
