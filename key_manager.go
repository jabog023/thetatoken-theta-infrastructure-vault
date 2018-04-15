package vault

import (
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/go-crypto/keys"
)

type Record struct {
	UserID     string
	Address    string
	PubKey     crypto.PubKey
	PrivateKey crypto.PrivKey
	Type       string
}

type KeyManager interface {
	FindByUserId(userid string) (Record, error)
	FindByAddress(address string) (Record, error)
	Update(r Record) error
}

func Sign(pubKey crypto.PubKey, privKey crypto.PrivKey, tx keys.Signable) ([]byte, error) {
	sig := privKey.Sign(tx.SignBytes())
	err := tx.Sign(pubKey, sig)
	if err != nil {
		return nil, err
	}

	return tx.TxBytes()
}
