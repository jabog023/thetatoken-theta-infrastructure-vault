package db

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/thetatoken/ukulele/common"
	crypto "github.com/thetatoken/ukulele/crypto"

	"github.com/thetatoken/vault/util"
)

var ErrNoRecord = errors.New("DAO: no record in database")

type DAO struct {
	db *sql.DB
}

func NewDAO() (*DAO, error) {
	user := viper.GetString(util.CfgDbUser)
	pass := viper.GetString(util.CfgDbPass)
	host := viper.GetString(util.CfgDbHost)
	database := viper.GetString(util.CfgDbDatabase)

	dbURL := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", user, pass, host, database)
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "dbURL": dbURL}).Fatal("Failed to connect to database")
		return nil, err
	}
	return &DAO{db}, nil
}

func (da *DAO) Close() {
	da.db.Close()
}

func (da *DAO) FindByUserId(userid string) (Record, error) {
	tableName := viper.GetString(util.CfgDbTable)

	query := fmt.Sprintf("SELECT ra_privkey::bytea, ra_pubkey::bytea, ra_address::bytea, sa_privkey::bytea, sa_pubkey::bytea, sa_address::bytea, faucet_fund_claimed, created_at FROM %s WHERE userid=$1", tableName)
	row := da.db.QueryRow(query, userid)

	var raPrivkeyBytes, raPubkeyBytes, raAddress []byte
	var saPrivkeyBytes, saPubkeyBytes, saAddress []byte
	var faucetFunded sql.NullBool
	var createAt pq.NullTime
	err := row.Scan(&raPrivkeyBytes, &raPubkeyBytes, &raAddress, &saPrivkeyBytes, &saPubkeyBytes, &saAddress, &faucetFunded, &createAt)
	switch {
	case err == sql.ErrNoRows:
		return Record{}, ErrNoRecord
	case err != nil:
		return Record{}, err
	default:
		raPubKey, _ := crypto.PublicKeyFromBytes(raPubkeyBytes)
		raPrivKey, _ := crypto.PrivateKeyFromBytes(raPrivkeyBytes)
		saPubKey, _ := crypto.PublicKeyFromBytes(saPubkeyBytes)
		saPrivKey, _ := crypto.PrivateKeyFromBytes(saPrivkeyBytes)

		record := Record{
			UserID:       userid,
			RaPubKey:     raPubKey,
			RaPrivateKey: raPrivKey,
			RaAddress:    common.BytesToAddress(raAddress),
			SaPubKey:     saPubKey,
			SaPrivateKey: saPrivKey,
			SaAddress:    common.BytesToAddress(saAddress),
			CreatedAt:    createAt.Time,
			FaucetFunded: faucetFunded.Bool,
		}
		return record, nil
	}
}

func (da *DAO) Create(record Record) error {
	tableName := viper.GetString(util.CfgDbTable)

	sm := fmt.Sprintf("INSERT INTO %s (userid, ra_pubkey, ra_privkey, ra_address, sa_pubkey, sa_privkey, sa_address) VALUES ($1, DECODE($2, 'hex'), DECODE($3, 'hex'), DECODE($4, 'hex'), DECODE($5, 'hex'), DECODE($6, 'hex'), DECODE($7, 'hex'))", tableName)

	raPubkeyBytes := record.RaPubKey.ToBytes()
	raPrivBytes := record.RaPrivateKey.ToBytes()
	saPubkeyBytes := record.SaPubKey.ToBytes()
	saPrivBytes := record.SaPrivateKey.ToBytes()

	_, err := da.db.Exec(sm, record.UserID, hex.EncodeToString(raPubkeyBytes), hex.EncodeToString(raPrivBytes), hex.EncodeToString(record.RaAddress.Bytes()), hex.EncodeToString(saPubkeyBytes), hex.EncodeToString(saPrivBytes), hex.EncodeToString(record.SaAddress.Bytes()))
	return err
}

func (da *DAO) FindUnfundedUsers(limit int) ([]Record, error) {
	tableName := viper.GetString(util.CfgDbTable)

	query := fmt.Sprintf("SELECT userid, sa_address::bytea, faucet_fund_claimed, created_at FROM %s WHERE faucet_fund_claimed=FALSE order by created_at limit %d", tableName, limit)
	rows, err := da.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var userid string
		var saAddress []byte
		var createdAt pq.NullTime
		var faucetClaimed sql.NullBool
		if err := rows.Scan(&userid, &saAddress, &faucetClaimed, &createdAt); err != nil {
			return records, errors.Wrap(err, "Failed to parse results from database")
		}
		records = append(records, Record{
			UserID:       userid,
			SaAddress:    common.BytesToAddress(saAddress),
			CreatedAt:    createdAt.Time,
			FaucetFunded: faucetClaimed.Bool,
		})
	}
	if err := rows.Err(); err != nil {
		return records, errors.Wrap(err, "Failed to parse results from database")
	}
	return records, nil
}

func (da *DAO) MarkUserFunded(address common.Address) error {
	tableName := viper.GetString(util.CfgDbTable)

	sm := fmt.Sprintf("UPDATE %s SET faucet_fund_claimed=TRUE WHERE encode(sa_address::bytea,'hex')=$1", tableName)
	res, err := da.db.Exec(sm, hex.EncodeToString(address.Bytes()))
	if err != nil {
		return errors.Wrap(err, "Failed to update database")
	}
	n, err := res.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "Failed to update database")
	}
	if n != 1 {
		return fmt.Errorf("Failed to update database: affected rows = %v\n", n)
	}
	return nil
}

type Record struct {
	UserID       string
	RaAddress    common.Address
	RaPubKey     *crypto.PublicKey
	RaPrivateKey *crypto.PrivateKey
	SaAddress    common.Address
	SaPubKey     *crypto.PublicKey
	SaPrivateKey *crypto.PrivateKey
	Type         string
	CreatedAt    time.Time
	FaucetFunded bool
}
