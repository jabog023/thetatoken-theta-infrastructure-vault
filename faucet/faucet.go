package faucet

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"
	"time"

	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type FaucetManager struct {
	db                   *sql.DB
	processedUserInBatch int
}

func NewFaucetManager(db *sql.DB) *FaucetManager {
	return &FaucetManager{
		db:                   db,
		processedUserInBatch: 0,
	}
}

// Goroutine to process job queue.
func (fr *FaucetManager) Process() {
	logger := log.WithFields(log.Fields{"method": "FaucetManager.Process"})

	sleepBatch := viper.GetInt64("faucet.sleep_between_batches_secs")
	sleepWakeup := viper.GetInt64("faucet.sleep_between_wakeups_secs")

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

	grantsPerBatch := viper.GetInt("faucet.grants_per_batch")
	tableName := viper.GetString("DbTableName")

	if fr.processedUserInBatch >= grantsPerBatch {
		logger.Infof("Batch cap %d reached. Not granting funds.", grantsPerBatch)
		return
	}
	maxUsers := grantsPerBatch - fr.processedUserInBatch
	logger.Infof("Ready to process %d users", maxUsers)

	query := fmt.Sprintf("SELECT address::bytea, faucet_fund_claimed, created_at FROM %s WHERE faucet_fund_claimed=FALSE order by created_at limit %d", tableName, maxUsers)
	rows, err := fr.db.Query(query)
	if err != nil {
		logger.WithFields(log.Fields{"error": err}).Error("Failed to fetch users from database")
	}
	defer rows.Close()

	count, errCount := 0, 0
	for rows.Next() {
		var address []byte
		var createdAt pq.NullTime
		var faucetClaimed sql.NullBool
		if err := rows.Scan(&address, &faucetClaimed, &createdAt); err != nil {
			logger.WithFields(log.Fields{"error": err}).Error("Failed to parse results from database")
		}
		logger.WithFields(log.Fields{"address": hex.EncodeToString(address), "createAt": createdAt.Time, "faucetClaimed": faucetClaimed.Bool}).Info("Process faucet queue item")
		err := fr.addInitalFund(hex.EncodeToString(address))
		if err != nil {
			errCount++
		}
		count++
	}
	if err := rows.Err(); err != nil {
		logger.WithFields(log.Fields{"error": err}).Error("Failed to parse results from database")
	}

	fr.processedUserInBatch += count
	logger.Infof("Processed %d users with %d failures. Sleeping...", count, errCount)
}

func (fr *FaucetManager) addInitalFund(address string) error {
	thetaAmount := viper.GetInt64("InitialTheta")
	gammaAmount := viper.GetInt64("InitialGamma")
	tableName := viper.GetString("DbTableName")

	logger := log.WithFields(log.Fields{"method": "addInitalFund", "address": address, "theta": thetaAmount, "gamma": gammaAmount})

	if thetaAmount == 0 && gammaAmount == 0 {
		return nil
	}

	sm := fmt.Sprintf("UPDATE %s SET faucet_fund_claimed=TRUE WHERE encode(address::bytea,'hex')=$1", tableName)
	res, err := fr.db.Exec(sm, address)
	if err != nil {
		logger.WithFields(log.Fields{"err": err, "result": res}).Error("Failed to update database")
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		logger.WithFields(log.Fields{"err": err, "rows_affected": n}).Error("Failed to update database")
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
