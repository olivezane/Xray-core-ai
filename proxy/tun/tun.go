package tun

// Tun interface implements tun interface interaction
type Tun interface {
	// SetUpdater hands the outbound-interface updater of the owning Handler
	// to the device, so its platform monitor can refresh it on network
	// changes. May be nil when outbound interface binding is not configured.
	SetUpdater(updater *InterfaceUpdater)
	Start() error
	Close() error
	Name() (string, error)
	Index() (int, error)
}
