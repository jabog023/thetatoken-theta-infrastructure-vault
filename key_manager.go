package vault

type Record struct {
	UserID     string
	Address    string
	PubKey     []byte
	PrivateKey []byte
	Type       string
}

type KeyManager interface {
	FindByUserId(userid string) (Record, error)
	FindByAddress(address string) (Record, error)
	Update(r Record) error
}
