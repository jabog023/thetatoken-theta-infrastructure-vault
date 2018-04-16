package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	json "github.com/gorilla/rpc/v2/json2"
	theta "github.com/thetatoken/theta/rpc"
	"github.com/thetatoken/vault"
	rpcc "github.com/ybbus/jsonrpc"
)

// TODO: read port from config.
const RPCPort = "20000"
const MAXConnections = 1000

func startServer() {
	s := rpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	s.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	client := rpcc.NewRPCClient("http://localhost:16888/rpc")
	keyManager, err := vault.NewMySqlKeyManager("root", "", "theta")
	if err != nil {
		log.Fatal(err)
	}
	defer keyManager.Close()

	handler := &vault.ThetaRPCHandler{client, keyManager}
	s.RegisterService(handler, "theta")
	r := mux.NewRouter()
	r.Handle("/rpc", s)
	// TODO: add a filter to translate lower case method name to uppper case.

	l, err := net.Listen("tcp", ":"+RPCPort)
	if err != nil {
		log.Fatalf("Listen: %v", err)
	}
	defer l.Close()

	log.Fatal(http.Serve(l, r))
	return
}

func test() {
	client := rpcc.NewRPCClient("http://localhost:16888/rpc")
	result, err := client.Call("theta.GetAccount", theta.GetAccountArgs{Name: "faucet"})

	fmt.Printf("result: %v, %v\n", result, err)
}

func main() {
	// test()

	go startServer()

	select {}
}
