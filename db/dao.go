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
	crypto "github.com/thetatoken/theta/go-crypto"
	"github.com/thetatoken/theta/types"
)

var ErrNoRecord = errors.New("DAO: no record in database")

type DAO struct {
	db *sql.DB
}

func NewDAO() (*DAO, error) {
	user := viper.GetString("DbUser")
	pass := viper.GetString("DbPass")
	host := viper.GetString("DbHost")
	database := viper.GetString("DbName")

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
	tableName := viper.GetString("DbTableName")

	query := fmt.Sprintf("SELECT privkey::bytea, pubkey::bytea, address::bytea, faucet_fund_claimed, created_at FROM %s WHERE userid=$1", tableName)
	row := da.db.QueryRow(query, userid)

	var privkeyBytes, pubkeyBytes, address []byte
	var faucetFunded sql.NullBool
	var createAt pq.NullTime
	err := row.Scan(&privkeyBytes, &pubkeyBytes, &address, &faucetFunded, &createAt)
	switch {
	case err == sql.ErrNoRows:
		return Record{}, ErrNoRecord
	case err != nil:
		return Record{}, err
	default:
		pubKey := crypto.PubKey{}
		types.FromBytes(pubkeyBytes, &pubKey)
		privKey := crypto.PrivKey{}
		types.FromBytes(privkeyBytes, &privKey)

		record := Record{
			UserID:       userid,
			PubKey:       pubKey,
			PrivateKey:   privKey,
			Address:      hex.EncodeToString(address),
			CreatedAt:    createAt.Time,
			FaucetFunded: faucetFunded.Bool,
		}
		return record, nil
	}
}

func (da *DAO) Create(record Record) error {
	tableName := viper.GetString("DbTableName")

	sm := fmt.Sprintf("INSERT INTO %s (userid, pubkey, privkey, address) VALUES ($1, DECODE($2, 'hex'), DECODE($3, 'hex'), DECODE($4, 'hex'))", tableName)

	pubkeyBytes, err := types.ToBytes(&record.PubKey)
	if err != nil {
		return err
	}
	privBytes, err := types.ToBytes(&record.PrivateKey)
	if err != nil {
		return err
	}

	_, err = da.db.Exec(sm, record.UserID, hex.EncodeToString(pubkeyBytes), hex.EncodeToString(privBytes), record.Address)
	return err
}

func (da *DAO) FindUnfundedUsers(limit int) ([]Record, error) {
	tableName := viper.GetString("DbTableName")

	query := fmt.Sprintf("SELECT userid, address::bytea, faucet_fund_claimed, created_at FROM %s WHERE faucet_fund_claimed=FALSE order by created_at limit %d", tableName, limit)
	rows, err := da.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var userid string
		var address []byte
		var createdAt pq.NullTime
		var faucetClaimed sql.NullBool
		if err := rows.Scan(&userid, &address, &faucetClaimed, &createdAt); err != nil {
			return records, errors.Wrap(err, "Failed to parse results from database")
		}
		records = append(records, Record{
			UserID:       userid,
			Address:      hex.EncodeToString(address),
			CreatedAt:    createdAt.Time,
			FaucetFunded: faucetClaimed.Bool,
		})
	}
	if err := rows.Err(); err != nil {
		return records, errors.Wrap(err, "Failed to parse results from database")
	}
	return records, nil
}

func (da *DAO) MarkUserFunded(address string) error {
	tableName := viper.GetString("DbTableName")

	sm := fmt.Sprintf("UPDATE %s SET faucet_fund_claimed=TRUE WHERE encode(address::bytea,'hex')=$1", tableName)
	res, err := da.db.Exec(sm, address)
	if err != nil {
		return errors.Wrap(err, "Failed to update database")
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return errors.Wrap(err, "Failed to update database")
	}
	return nil
}

type Record struct {
	UserID       string
	Address      string
	PubKey       crypto.PubKey
	PrivateKey   crypto.PrivKey
	Type         string
	CreatedAt    time.Time
	FaucetFunded bool
}
