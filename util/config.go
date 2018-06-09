package util

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func ReadConfig() {
	logger := log.WithFields(log.Fields{"method": "readConfig"})

	viper.SetDefault("DbHost", "localhost")
	viper.SetDefault("DbName", "sliver_video_serving")
	viper.SetDefault("DbTableName", "user_theta_native_wallet")
	viper.SetDefault("Debug", false)
	viper.SetDefault("PRCPort", "20000")
	viper.SetDefault("ChainID", "test_chain_id")
	viper.SetDefault("MaxConnections", 200)
	viper.SetDefault("faucet.grants_per_batch", 100)
	viper.SetDefault("faucet.sleep_between_batches_secs", 3600)
	viper.SetDefault("faucet.sleep_between_wakeups_secs", 10)

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
