package udp

import (
	"context"

	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/stat"
)

// Dial opens a datagram connection towards dest.
func Dial(ctx context.Context, dest net.Destination, streamSettings *internet.MemoryStreamConfig) (stat.Connection, error) {
	if streamSettings != nil && streamSettings.FinalMask != nil {
		return streamSettings.FinalMask.DialUDP(ctx, dest)
	}
	var sockopt *internet.SocketConfig
	if streamSettings != nil && streamSettings.SocketSettings != nil {
		sockopt = streamSettings.SocketSettings
	}
	return internet.DialSystem(ctx, dest, sockopt)
}
