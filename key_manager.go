package vault

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/go-crypto/keys"
	"github.com/thetatoken/theta/types"
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
	Close()
	// FindByAddress(address string) (Record, error)
	// Update(r Record) error
}

func Sign(pubKey crypto.PubKey, privKey crypto.PrivKey, tx keys.Signable) ([]byte, error) {
	sig := privKey.Sign(tx.SignBytes())
	err := tx.Sign(pubKey, sig)
	if err != nil {
		return nil, err
	}
	return tx.TxBytes()
}

// ----------------- MySQL KeyManager ---------------------

var _ KeyManager = MySqlKeyManager{}

type MySqlKeyManager struct {
	db *sql.DB
}

func NewMySqlKeyManager(user string, pass string, dbname string) (*MySqlKeyManager, error) {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@/%s", user, pass, dbname))
	if err != nil {
		return nil, err
	}
	return &MySqlKeyManager{db}, nil
}

func (km MySqlKeyManager) FindByUserId(userid string) (Record, error) {
	row := km.db.QueryRow("SELECT privkey, pubkey, address FROM vault WHERE userid=?", userid)
	var privkeyBytes, pubkeyBytes, address []byte
	err := row.Scan(&privkeyBytes, &pubkeyBytes, &address)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("No user with that ID.")
		return Record{}, err
	case err != nil:
		log.Printf(err.Error())
		return Record{}, err
	default:
		pubKey := crypto.PubKey{}
		types.FromBytes(pubkeyBytes, &pubKey)
		privKey := crypto.PrivKey{}
		types.FromBytes(privkeyBytes, &privKey)
		record := Record{
			UserID:     userid,
			PubKey:     pubKey,
			PrivateKey: privKey,
			Address:    hex.EncodeToString(address),
		}
		return record, nil
	}
}

func (km MySqlKeyManager) Close() {
	km.db.Close()
}
