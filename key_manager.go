package vault

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

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
	Close()
	FindByUserId(userid string) (Record, error)
	// FindByAddress(address string) (Record, error)
	Create(r Record) error
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

// ----------------- MySQL KeyManager ---------------------

var _ KeyManager = MySqlKeyManager{}

const TABLE_NAME = "vault"

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
		log.Printf("No record with user ID: %s. Creating keys.", userid)
		address, pubkey, privkey, _, err := genKey()
		if err != nil {
			return Record{}, err
		}
		record := Record{
			Address:    address,
			PubKey:     pubkey,
			PrivateKey: privkey,
			UserID:     userid,
		}
		err = km.Create(record)
		if err != nil {
			return Record{}, err
		}
		return record, nil
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

func (km MySqlKeyManager) Create(record Record) error {
	sm := fmt.Sprintf("INSERT INTO %s (userid, pubkey, privkey, address) VALUES (?, UNHEX(?), UNHEX(?), UNHEX(?))", TABLE_NAME)

	pubkeyBytes, err := types.ToBytes(&record.PubKey)
	if err != nil {
		return err
	}
	privBytes, err := types.ToBytes(&record.PrivateKey)
	if err != nil {
		return err
	}

	_, err = km.db.Exec(sm, record.UserID, hex.EncodeToString(pubkeyBytes), hex.EncodeToString(privBytes), record.Address)
	return err
}
