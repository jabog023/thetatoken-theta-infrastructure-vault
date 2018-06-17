package faucet

import (
	"os/exec"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/thetatoken/vault/db"
	"github.com/thetatoken/vault/util"
)

type FaucetManager struct {
	da                   *db.DAO
	processedUserInBatch int
}

func NewFaucetManager(da *db.DAO) *FaucetManager {
	return &FaucetManager{
		da:                   da,
		processedUserInBatch: 0,
	}
}

// Goroutine to process job queue.
func (fr *FaucetManager) Process() {
	logger := log.WithFields(log.Fields{"method": "FaucetManager.Process"})

	sleepBatch := viper.GetInt64(util.CfgFaucetBatchDuration)
	sleepWakeup := viper.GetInt64(util.CfgFaucetWakeupInterval)

	resetTicker := time.NewTicker(time.Duration(sleepBatch) * time.Second)
	wakeupTicker := time.NewTicker(time.Duration(sleepWakeup) * time.Second)
	defer resetTicker.Stop()
	defer wakeupTicker.Stop()

	for {
		select {
		case <-resetTicker.C:
			logger.Info("Resetting batch count")
			fr.processedUserInBatch = 0
		case <-wakeupTicker.C:
			fr.tryGrantFunds()
		}
	}
}

func (fr *FaucetManager) tryGrantFunds() {
	logger := log.WithFields(log.Fields{"method": "FaucetManager.tryGrantFunds"})

	grantsPerBatch := viper.GetInt(util.CfgFaucetGrantsPerBatch)

	if fr.processedUserInBatch >= grantsPerBatch {
		logger.Infof("Batch cap %d reached. Not granting funds.", grantsPerBatch)
		return
	}
	maxUsers := grantsPerBatch - fr.processedUserInBatch
	logger.Infof("Ready to process %d users", maxUsers)

	records, err := fr.da.FindUnfundedUsers(maxUsers)
	if err != nil {
		logger.WithFields(log.Fields{"error": err}).Error("Failed to fetch users from database")
	}

	count, errCount := 0, 0
	for _, record := range records {
		logger.WithFields(log.Fields{"record": record}).Info("Process faucet queue item")
		err := fr.addInitalFund(record.SaAddress)
		if err != nil {
			errCount++
		}
		count++
	}
	fr.processedUserInBatch += count
	logger.Infof("Processed %d users with %d failures. Sleeping...", count, errCount)
}

func (fr *FaucetManager) addInitalFund(address string) error {
	thetaAmount := viper.GetInt64(util.CfgFaucetThetaAmount)
	gammaAmount := viper.GetInt64(util.CfgFaucetGammaAmount)

	logger := log.WithFields(log.Fields{"method": "addInitalFund", "address": address, "theta": thetaAmount, "gamma": gammaAmount})

	if thetaAmount == 0 && gammaAmount == 0 {
		return nil
	}

	err := fr.da.MarkUserFunded(address)
	if err != nil {
		logger.WithFields(log.Fields{"err": err}).Error("Failed to mark user as funded")
		return err
	}

	logger.Info("Executing add fund command")
	cmd := exec.Command("add_fund.sh", address, strconv.FormatInt(thetaAmount, 10), strconv.FormatInt(gammaAmount, 10))
	err = cmd.Run()
	if err != nil {
		logger.WithFields(log.Fields{"err": err, "output": err}).Error("Add fund command failed")
		return err
	}
	log.Info("Successfully added fund")
	return err
}
