package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc/v2"
	json "github.com/gorilla/rpc/v2/json2"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/thetatoken/vault"
	rpcc "github.com/ybbus/jsonrpc"
)

var logger = log.WithFields(log.Fields{"component": "server"})

func decompressMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("content-encoding"), "gzip") {
			handler.ServeHTTP(w, r)
			return
		}

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.WithFields(log.Fields{"error": err}).Error("Error reading body")
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}
		// And now set a new body, which will simulate the same data we read:
		r.Body, err = gzip.NewReader(bytes.NewBuffer(body))
		if err != nil {
			logger.WithFields(log.Fields{"error": err}).Error("Error decompressing request body")
			http.Error(w, "Error decompressing request body", http.StatusBadRequest)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func debugMiddleware(handler http.Handler) http.Handler {
	if !viper.GetBool("Debug") {
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.WithFields(log.Fields{"error": err}).Debug("Error reading body")
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}

		logger.WithFields(log.Fields{"body": string(body), "headers": r.Header}).Debug("Request body")

		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		handler.ServeHTTP(w, r)
	})
}

func startServer() {
	s := rpc.NewServer()
	s.RegisterCodec(json.NewCodec(), "application/json")
	s.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	client := rpcc.NewRPCClient("http://localhost:16888/rpc")
	keyManager, err := vault.NewSqlKeyManager(viper.GetString("DbUser"), viper.GetString("DbPass"), viper.GetString("DbHost"), viper.GetString("DbName"))

	if err != nil {
		logger.Fatal(err)
	}
	defer keyManager.Close()

	handler := vault.NewRPCHandler(client, keyManager)
	s.RegisterService(handler, "theta")
	r := mux.NewRouter()
	r.Use(debugMiddleware)
	r.Use(decompressMiddleware)
	r.Handle("/rpc", s)

	port := viper.GetString("RPCPort")
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatalf("Listen: %v", err)
	}
	defer l.Close()

	logger.Info(fmt.Sprintf("Listening on %s\n", port))
	logger.Fatal(http.Serve(l, r))
	return
}

func readConfig() {
	viper.SetDefault("DbHost", "localhost")
	viper.SetDefault("DbName", "sliver_video_serving")
	viper.SetDefault("DbTableName", "user_theta_native_wallet")
	viper.SetDefault("Debug", false)
	viper.SetDefault("PRCPort", "20000")
	viper.SetDefault("ChainID", "test_chain_id")

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		logger.WithFields(log.Fields{"error": err}).Fatal(fmt.Errorf("Fatal error config file"))
	}

	if viper.GetBool("Debug") {
		log.SetLevel(log.DebugLevel)
	}
}

func main() {
	readConfig()

	go vault.Faucet.Process()
	go startServer()

	select {}
}
