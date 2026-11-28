package protocol

// Account is a user identity used for authentication.
type Account interface {
	Equals(Account) bool
	// ToProto is the account as the API carries it: a protobuf message by the
	// name and encoding protobuf gives it, or a plain Go value as JSON under the
	// name it says for itself.
	ToProto() any
}

// AsAccount is an object can be converted into account.
type AsAccount interface {
	AsAccount() (Account, error)
}
