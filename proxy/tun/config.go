package tun

import (
	"context"
	"errors"
	"net"
	"sync"

	xerrors "github.com/xtls/xray-core/common/errors"
)

// InterfaceUpdater caches the physical outbound interface that outbound
// traffic binds to, and refreshes it when the route table changes. It is
// owned by a single Handler instance (no process-global state) and handed to
// the platform monitor of that instance's device by reference.
type InterfaceUpdater struct {
	sync.Mutex

	table     RouteTable // nil -> platform default (newRouteTable)
	tunIndex  int
	fixedName string
	iface     *net.Interface
}

func (updater *InterfaceUpdater) Get() *net.Interface {
	updater.Lock()
	defer updater.Unlock()

	return updater.iface
}

// Update refreshes the cached interface through the RouteTable seam. On
// lookup failure the last known good interface is kept, so transient network
// transitions (e.g. wifi -> cellular) do not drop the binding.
func (updater *InterfaceUpdater) Update() {
	updater.Lock()
	defer updater.Unlock()

	table := updater.table
	if table == nil {
		table = newRouteTable()
	}
	got, err := table.OutboundInterface(updater.tunIndex, updater.fixedName)
	if err != nil {
		if errors.Is(err, ErrNoOutboundInterface) {
			xerrors.LogDebug(context.Background(), "[tun] no outbound interface found")
		} else {
			xerrors.LogInfoInner(context.Background(), err, "[tun] failed to update interface")
		}
		return
	}

	if updater.iface != nil && updater.iface.Index == got.Index && updater.iface.Name == got.Name {
		return
	}

	updater.iface = got
	xerrors.LogInfo(context.Background(), "[tun] update interface ", got.Name, " ", got.Index)
}

// Reset clears the cached interface, preventing stale interface references
// from persisting after TUN shutdown.
func (updater *InterfaceUpdater) Reset() {
	updater.Lock()
	defer updater.Unlock()
	updater.iface = nil
}
