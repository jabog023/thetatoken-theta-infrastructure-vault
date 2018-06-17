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
		raPubKey := crypto.PubKey{}
		types.FromBytes(raPubkeyBytes, &raPubKey)
		raPrivKey := crypto.PrivKey{}
		types.FromBytes(raPrivkeyBytes, &raPrivKey)
		saPubKey := crypto.PubKey{}
		types.FromBytes(saPubkeyBytes, &saPubKey)
		saPrivKey := crypto.PrivKey{}
		types.FromBytes(saPrivkeyBytes, &saPrivKey)

		record := Record{
			UserID:       userid,
			RaPubKey:     raPubKey,
			RaPrivateKey: raPrivKey,
			RaAddress:    hex.EncodeToString(raAddress),
			SaPubKey:     saPubKey,
			SaPrivateKey: saPrivKey,
			SaAddress:    hex.EncodeToString(saAddress),
			CreatedAt:    createAt.Time,
			FaucetFunded: faucetFunded.Bool,
		}
		return record, nil
	}
}

func (da *DAO) Create(record Record) error {
	tableName := viper.GetString(util.CfgDbTable)

	sm := fmt.Sprintf("INSERT INTO %s (userid, ra_pubkey, ra_privkey, ra_address, sa_pubkey, sa_privkey, sa_address) VALUES ($1, DECODE($2, 'hex'), DECODE($3, 'hex'), DECODE($4, 'hex'), DECODE($5, 'hex'), DECODE($6, 'hex'), DECODE($7, 'hex'))", tableName)

	raPubkeyBytes, err := types.ToBytes(&record.RaPubKey)
	if err != nil {
		return err
	}
	raPrivBytes, err := types.ToBytes(&record.RaPrivateKey)
	if err != nil {
		return err
	}
	saPubkeyBytes, err := types.ToBytes(&record.SaPubKey)
	if err != nil {
		return err
	}
	saPrivBytes, err := types.ToBytes(&record.SaPrivateKey)
	if err != nil {
		return err
	}

	_, err = da.db.Exec(sm, record.UserID, hex.EncodeToString(raPubkeyBytes), hex.EncodeToString(raPrivBytes), record.RaAddress, hex.EncodeToString(saPubkeyBytes), hex.EncodeToString(saPrivBytes), record.SaAddress)
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
			SaAddress:    hex.EncodeToString(saAddress),
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
	tableName := viper.GetString(util.CfgDbTable)

	sm := fmt.Sprintf("UPDATE %s SET faucet_fund_claimed=TRUE WHERE encode(sa_address::bytea,'hex')=$1", tableName)
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
	RaAddress    string
	RaPubKey     crypto.PubKey
	RaPrivateKey crypto.PrivKey
	SaAddress    string
	SaPubKey     crypto.PubKey
	SaPrivateKey crypto.PrivKey
	Type         string
	CreatedAt    time.Time
	FaucetFunded bool
}
