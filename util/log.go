package util

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func SetupLogger() {
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	log.SetFormatter(customFormatter)
	customFormatter.FullTimestamp = true

	if viper.GetBool(CfgDebug) {
		log.SetLevel(log.DebugLevel)
	}
}

type httpLogWriter struct {
	http.ResponseWriter
	B *strings.Builder
}

func (w httpLogWriter) Write(b []byte) (int, error) {
	w.B.Write(b)
	return w.ResponseWriter.Write(b)
}

func LoggerMiddleware(handler http.Handler) http.Handler {
	logger := log.WithFields(log.Fields{"method": "rpc.handler"})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.WithFields(log.Fields{"error": err}).Debug("Error reading body")
			http.Error(w, "can't read body", http.StatusBadRequest)
			return
		}

		logger.WithFields(log.Fields{"body": string(body), "headers": r.Header}).Debug("RPC Request")
		start := time.Now()

		r.Body = ioutil.NopCloser(bytes.NewBuffer(body))

		lw := httpLogWriter{ResponseWriter: w, B: &strings.Builder{}}
		handler.ServeHTTP(lw, r)

		elapsed := time.Since(start)
		responseBody := lw.B.String()
		logger.WithFields(log.Fields{"time": elapsed, "body": responseBody, "headers": w.Header()}).Debug("RPC Response")
	})
}
