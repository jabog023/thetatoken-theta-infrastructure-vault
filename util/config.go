package util

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	CfgDbHost                          = "db.host"
	CfgDbUser                          = "db.user"
	CfgDbPass                          = "db.pass"
	CfgDbDatabase                      = "db.database"
	CfgDbTable                         = "db.table"
	CfgDebug                           = "debug"
	CfgServerPort                      = "server.port"
	CfgServerMaxConnections            = "server.max_connections"
	CfgThetaChainId                    = "theta.chain_id"
	CfgThetaRPCEndpoint                = "theta.rpc_endpoint"
	CfgThetaDefaultReserveDurationSecs = "theta.default_reserve_duration_secs"
	CfgFaucetGrantsPerBatch            = "faucet.grants_per_batch"
	CfgFaucetBatchDuration             = "faucet.sleep_between_batches_secs"
	CfgFaucetWakeupInterval            = "faucet.sleep_between_wakeups_secs"
	CfgFaucetThetaAmount               = "faucet.theta"
	CfgFaucetGammaAmount               = "faucet.gamma"
)

func ReadConfig() {
	logger := log.WithFields(log.Fields{"method": "readConfig"})

	viper.SetDefault(CfgDbHost, "localhost")
	viper.SetDefault(CfgDbDatabase, "sliver_video_serving")
	viper.SetDefault(CfgDbTable, "user_theta_native_wallet")
	viper.SetDefault(CfgDebug, false)
	viper.SetDefault(CfgServerPort, "20000")
	viper.SetDefault(CfgServerMaxConnections, 200)
	viper.SetDefault(CfgThetaChainId, "test_chain_id")
	viper.SetDefault(CfgThetaDefaultReserveDurationSecs, 900)
	viper.SetDefault(CfgFaucetGrantsPerBatch, 100)
	viper.SetDefault(CfgFaucetBatchDuration, 3600)
	viper.SetDefault(CfgFaucetWakeupInterval, 10)

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		logger.WithFields(log.Fields{"error": err}).Fatal(fmt.Errorf("Fatal error config file"))
	}
}
