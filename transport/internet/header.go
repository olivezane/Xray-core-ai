package internet

import (
	"context"
	"reflect"

	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
)

type PacketHeader interface {
	Size() int32
	Serialize([]byte)
}

type ConnectionAuthenticator interface {
	Client(net.Conn) net.Conn
	Server(net.Conn) net.Conn
}

// Header is one header implementation: the configuration it is created from and
// the function that creates it.
//
// A header does not register itself. Every header of a build is listed in
// transport/internet/headers, and that list is installed here by a single call,
// so reading it tells you which headers exist and who creates them.
type Header struct {
	// Config is the configuration type the header is created from, e.g.
	// (*noop.Config)(nil).
	Config interface{}
	// New creates the header from a configuration of that type.
	New func(ctx context.Context, config interface{}) (interface{}, error)
}

var headers = make(map[reflect.Type]Header)

// SetHeaders installs list as the complete set of headers of this build. A
// header that is not listed cannot be created, and a broken list is reported
// where it is written instead of when the header is first needed.
func SetHeaders(list []Header) error {
	installed := make(map[reflect.Type]Header, len(list))
	for _, header := range list {
		if header.Config == nil {
			return errors.New("header without a configuration type")
		}
		if header.New == nil {
			return errors.New("header for ", reflect.TypeOf(header.Config), " has no constructor")
		}
		configType := reflect.TypeOf(header.Config)
		if _, found := installed[configType]; found {
			return errors.New("header for ", configType, " is listed twice")
		}
		installed[configType] = header
	}
	headers = installed
	return nil
}

func createHeader(config interface{}) (interface{}, error) {
	header, found := headers[reflect.TypeOf(config)]
	if !found {
		return nil, errors.New("no header for ", reflect.TypeOf(config), "; the headers of this build are listed in transport/internet/headers")
	}
	return header.New(context.Background(), config)
}

func CreatePacketHeader(config interface{}) (PacketHeader, error) {
	header, err := createHeader(config)
	if err != nil {
		return nil, err
	}
	if h, ok := header.(PacketHeader); ok {
		return h, nil
	}
	return nil, errors.New("not a packet header")
}

func CreateConnectionAuthenticator(config interface{}) (ConnectionAuthenticator, error) {
	auth, err := createHeader(config)
	if err != nil {
		return nil, err
	}
	if a, ok := auth.(ConnectionAuthenticator); ok {
		return a, nil
	}
	return nil, errors.New("not a ConnectionAuthenticator")
}
