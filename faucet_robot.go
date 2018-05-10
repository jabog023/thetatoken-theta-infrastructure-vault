package vault

import (
	"os/exec"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const NewAccountQueueLength = 100

var Faucet = NewFaucetManager()

type FaucetManager struct {
	// TODO: Later we might need to use a persisited job queue for better scalability.
	newAccounts chan string
}

func NewFaucetManager() *FaucetManager {
	return &FaucetManager{
		newAccounts: make(chan string, NewAccountQueueLength),
	}
}

func (fr *FaucetManager) AddInitalFundAsync(address string) {
	log.WithFields(log.Fields{"address": address, "queue length": len(fr.newAccounts)}).Info("Adding address to faucet queue")
	fr.newAccounts <- address
}

// Goroutine to process job queue.
func (fr *FaucetManager) Process() {
	for {
		address := <-fr.newAccounts
		addInitalFund(address)
	}
}

func addInitalFund(address string) {
	amount := viper.GetInt64("InitialFund")
	if amount <= 0 {
		return
	}
	log.WithFields(log.Fields{"address": address, "amount": amount}).Info("Adding initial fund")
	cmd := exec.Command("add_fund.sh", address, strconv.FormatInt(amount, 10))
	err := cmd.Run()
	if err != nil {
		log.WithFields(log.Fields{"err": err, "output": string(err.(*exec.ExitError).Stderr)}).Error("Failed to add fund")
	}
}
