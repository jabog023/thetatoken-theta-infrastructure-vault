package keymanager

import (
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	crypto "github.com/thetatoken/ukulele/crypto"
	"github.com/thetatoken/vault/db"
)

type KeyManager interface {
	Close()
	FindByUserId(userid string) (db.Record, error)
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
		raPrivkey, raPubkey, err := crypto.GenerateKeyPair()
		if err != nil {
			return db.Record{}, err
		}
		saPrivkey, saPubkey, err := crypto.GenerateKeyPair()
		if err != nil {
			return db.Record{}, err
		}
		record := db.Record{
			RaAddress:    raPubkey.Address(),
			RaPubKey:     raPubkey,
			RaPrivateKey: raPrivkey,
			SaAddress:    saPubkey.Address(),
			SaPubKey:     saPubkey,
			SaPrivateKey: saPrivkey,
			UserID:       userid,
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

func (km SqlKeyManager) Close() {
	km.da.Close()
}
