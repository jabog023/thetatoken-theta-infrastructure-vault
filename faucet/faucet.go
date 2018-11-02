package faucet

import (
	"fmt"
	"os/exec"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/thetatoken/ukulele/common"
	"github.com/thetatoken/vault/db"
	"github.com/thetatoken/vault/util"
	rpcc "github.com/ybbus/jsonrpc"
)

type FaucetManager struct {
	da                   *db.DAO
	client               *rpcc.RPCClient
	processedUserInBatch int
}

func NewFaucetManager(da *db.DAO, client *rpcc.RPCClient) *FaucetManager {
	return &FaucetManager{
		da:                   da,
		client:               client,
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

func (fr *FaucetManager) addInitalFund(address common.Address) error {
	thetaAmount := viper.GetInt64(util.CfgFaucetThetaAmount)
	gammaAmount := viper.GetInt64(util.CfgFaucetGammaAmount)

	logger := log.WithFields(log.Fields{"method": "addInitalFund", "address": address, "theta": thetaAmount, "gamma": gammaAmount})

	if thetaAmount == 0 && gammaAmount == 0 {
		return nil
	}

	err := fr.da.MarkUserFunded(address)
	if err != nil {
		logger.WithFields(log.Fields{"error": err}).Error("Failed to mark user as funded")
		return err
	}

	faucetAddress := viper.GetString(util.CfgFaucetAddress)
	if faucetAddress == "" {
		log.Panic("faucet address is not configured")
	}
	sequence, err := util.GetSequence(fr.client, common.HexToAddress(faucetAddress))

	if err != nil {
		logger.WithFields(log.Fields{"error": err, "faucet": faucetAddress}).Error("Failed to get seqeuence number")
	}
	logger.WithFields(log.Fields{"sequence": sequence}).Info("Executing add fund command")
	cmd := exec.Command("add_fund.sh", faucetAddress, address.Hex(),
		fmt.Sprintf("%d", thetaAmount), fmt.Sprintf("%d", gammaAmount),
		fmt.Sprintf("%d", sequence+1))

	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.WithFields(log.Fields{"err": err, "output": string(out)}).Error("Add fund command failed")
		return err
	}
	log.Info("Successfully added fund")
	return err
}
