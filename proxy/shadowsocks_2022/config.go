package shadowsocks_2022

import (
	"bytes"
	"encoding/base64"


	"github.com/xtls/xray-core/common/protocol"
)

// MemoryAccount is an account type converted from Account.
type MemoryAccount struct {
	Key []byte
}

// AsAccount implements protocol.AsAccount.
func (u *Account) AsAccount() (protocol.Account, error) {
	keyStr := u.Key
	raw, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		raw = []byte(keyStr)
	}
	return &MemoryAccount{
		Key: raw,
	}, nil
}

// Equals implements protocol.Account.Equals().
func (a *MemoryAccount) Equals(another protocol.Account) bool {
	if account, ok := another.(*MemoryAccount); ok {
		return bytes.Equal(a.Key, account.Key)
	}
	return false
}

func (a *MemoryAccount) ToProto() any {
	return &Account{
		Key: base64.StdEncoding.EncodeToString(a.Key),
	}
}
