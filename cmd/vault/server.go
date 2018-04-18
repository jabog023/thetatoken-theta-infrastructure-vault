package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	json "github.com/gorilla/rpc/v2/json2"
	"github.com/thetatoken/vault"
	rpcc "github.com/ybbus/jsonrpc"
)

// TODO: read port from config.
const RPCPort = "20000"
const MAXConnections = 1000
const Debug = false

func decompressMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("content-encoding"), "gzip") {
			handler.ServeHTTP(w, r)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}
		// And now set a new body, which will simulate the same data we read:
		r.Body, err = gzip.NewReader(bytes.NewBuffer(body))
		if err != nil {
			log.Printf("Error decompressing request body: %v", err)
			http.Error(w, "Error decompressing request body", http.StatusBadRequest)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func debugMiddleware(handler http.Handler) http.Handler {
	if !Debug {
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}

		// Loop through headers
		for name, headers := range r.Header {
			name = strings.ToLower(name)
			for _, h := range headers {
				fmt.Printf("Header: %v: %v\n", name, h)
			}
		}

		log.Printf("Body: %v\n", string(body))

		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		handler.ServeHTTP(w, r)
	})
}

func startServer() {
	s := rpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	s.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	client := rpcc.NewRPCClient("http://localhost:16888/rpc")
	// keyManager, err := vault.NewMySqlKeyManager("root", "", "theta")
	keyManager, err := vault.NewSqlKeyManager("postgres", "", "localhost", "sliver_video_serving")

	if err != nil {
		log.Fatal(err)
	}
	defer keyManager.Close()

	handler := &vault.ThetaRPCHandler{client, keyManager}
	s.RegisterService(handler, "theta")
	r := mux.NewRouter()
	r.Use(debugMiddleware)
	r.Use(decompressMiddleware)
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

func main() {
	go startServer()

	select {}
}
