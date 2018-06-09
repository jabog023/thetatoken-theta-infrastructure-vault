package keymanager

import (
	"encoding/hex"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	crypto "github.com/thetatoken/theta/go-crypto"
	"github.com/thetatoken/theta/go-crypto/keys"
	"github.com/thetatoken/vault/db"
)

type KeyManager interface {
	Close()
	FindByUserId(userid string) (db.Record, error)
}

func Sign(pubKey crypto.PubKey, privKey crypto.PrivKey, tx keys.Signable) ([]byte, error) {
	sig := privKey.Sign(tx.SignBytes())
	err := tx.Sign(pubKey, sig)
	if err != nil {
		return nil, err
	}
	return tx.TxBytes()
}

func genKey() (address string, pubkey crypto.PubKey, privKey crypto.PrivKey, seed string, err error) {
	privKey = crypto.GenPrivKeyEd25519().Wrap()
	pubkey = privKey.PubKey()
	address = hex.EncodeToString(pubkey.Address())
	codec := keys.MustLoadCodec("english")
	words, err := codec.BytesToWords(privKey.Bytes())
	seed = strings.Join(words, " ")
	return
}

// ----------------- SQL KeyManager ---------------------

var _ KeyManager = SqlKeyManager{}

type SqlKeyManager struct {
	da *db.DAO
}

func NewSqlKeyManager(da *db.DAO) (*SqlKeyManager, error) {
	return &SqlKeyManager{da}, nil
}

func (km SqlKeyManager) FindByUserId(userid string) (db.Record, error) {
	record, err := km.da.FindByUserId(userid)

	if err == db.ErrNoRecord {
		log.Printf("No record with user ID: %s. Creating keys.", userid)
		address, pubkey, privkey, _, err := genKey()
		if err != nil {
			return db.Record{}, err
		}
		record := db.Record{
			Address:    address,
			PubKey:     pubkey,
			PrivateKey: privkey,
			UserID:     userid,
		}
		err = km.da.Create(record)
		if err != nil {
			log.WithError(err).WithField("userid", userid).Error("Failed to create address")
			return db.Record{}, err
		}
		return record, nil
	}

	if err != nil {
		log.Printf(err.Error())
		return db.Record{}, errors.Wrap(err, "Failed to find user by id")
	}

	return record, nil
}

func (km SqlKeyManager) Close() {}
