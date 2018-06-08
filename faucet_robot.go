package vault

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
	db *sql.DB
}

func NewFaucetManager(db *sql.DB) *FaucetManager {
	return &FaucetManager{
		db: db,
	}
}

// Goroutine to process job queue.
func (fr *FaucetManager) Process() {
	logger := log.WithFields(log.Fields{"method": "FaucetManager.Process"})

	for {
		logger.Infof("Trying to process %d faucet items", viper.GetInt("faucet.grants_per_batch"))
		query := fmt.Sprintf("SELECT address::bytea, faucet_fund_claimed, created_at FROM %s WHERE faucet_fund_claimed=FALSE order by created_at limit %d", TableName, viper.GetInt("faucet.grants_per_batch"))
		rows, err := fr.db.Query(query)
		if err != nil {
			logger.WithFields(log.Fields{"error": err}).Error("Failed to fetch users from database")
		}
		defer rows.Close()
		for rows.Next() {
			var address []byte
			var createdAt pq.NullTime
			var faucetClaimed sql.NullBool
			if err := rows.Scan(&address, &faucetClaimed, &createdAt); err != nil {
				logger.WithFields(log.Fields{"error": err}).Error("Failed to parse results from database")
			}
			logger.WithFields(log.Fields{"address": hex.EncodeToString(address), "createAt": createdAt.Time, "faucetClaimed": faucetClaimed.Bool}).Info("Process faucet queue item")
			fr.addInitalFund(hex.EncodeToString(address))
		}
		if err := rows.Err(); err != nil {
			logger.WithFields(log.Fields{"error": err}).Error("Failed to parse results from database")
		}

		logger.Info("Batch done. Sleeping...")
		time.Sleep(time.Duration(viper.GetInt64("faucet.sleep_between_batches_secs")) * time.Second)
	}
}

func (fr *FaucetManager) addInitalFund(address string) error {
	thetaAmount := viper.GetInt64("InitialTheta")
	gammaAmount := viper.GetInt64("InitialGamma")

	logger := log.WithFields(log.Fields{"method": "addInitalFund", "address": address, "theta": thetaAmount, "gamma": gammaAmount})

	if thetaAmount == 0 && gammaAmount == 0 {
		return nil
	}

	sm := fmt.Sprintf("UPDATE %s SET faucet_fund_claimed=TRUE WHERE encode(address::bytea,'hex')=$1", TableName)
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
