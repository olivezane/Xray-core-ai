package conf

import (
	"strings"

	"github.com/xtls/xray-core/common/errors"
)

// ConfigRegistry is the one shared string-keyed registry for config codecs
// (inbound/outbound protocols, transport methods, router strategies,
// finalmask masks). Every failure carries the offending key.
type ConfigRegistry struct {
	creators map[string]ConfigCreator
}

func NewConfigRegistry() *ConfigRegistry {
	return &ConfigRegistry{creators: make(map[string]ConfigCreator)}
}

// Register adds a creator under id. Lookups are lowercased (see
// JSONConfigLoader.LoadWithID), so a non-lowercase or empty id could never
// be found: it is rejected here, at registration time.
func (r *ConfigRegistry) Register(id string, creator ConfigCreator) error {
	if id == "" {
		return errors.New("config registry: empty id").AtError()
	}
	if strings.ToLower(id) != id {
		return errors.New("config registry: id must be lowercase: ", id).AtError()
	}
	if _, found := r.creators[id]; found {
		return errors.New(id, " already registered.").AtError()
	}
	r.creators[id] = creator
	return nil
}

// MustRegister panics on registration failure; use from init() so a typo'd
// or duplicated key surfaces at startup with the offending string.
func (r *ConfigRegistry) MustRegister(id string, creator ConfigCreator) {
	if err := r.Register(id, creator); err != nil {
		panic(err)
	}
}

func (r *ConfigRegistry) Create(id string) (interface{}, error) {
	creator, found := r.creators[id]
	if !found {
		return nil, errors.New("unknown config id: ", id)
	}
	return creator(), nil
}
