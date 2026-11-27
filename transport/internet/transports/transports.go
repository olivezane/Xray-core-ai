// Package transports is the one place where Xray wires its stream transports.
//
// Each line names a protocol and gives the function that dials with it and the
// one that listens with it, so this file is the whole picture: there is no
// registration from init() functions scattered over the transport packages, and
// nothing else calls into a transport to make it usable.
//
// Adding a transport means adding its line here and its entry in
// transport/internet/transports/transports_test.go.
package transports

import (
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/grpc"
	"github.com/xtls/xray-core/transport/internet/httpupgrade"
	"github.com/xtls/xray-core/transport/internet/hysteria"
	"github.com/xtls/xray-core/transport/internet/kcp"
	"github.com/xtls/xray-core/transport/internet/masque"
	"github.com/xtls/xray-core/transport/internet/splithttp"
	"github.com/xtls/xray-core/transport/internet/tcp"
	"github.com/xtls/xray-core/transport/internet/udp"
	"github.com/xtls/xray-core/transport/internet/websocket"
	"github.com/xtls/xray-core/transport/internet/xdrive"
)

// All is every stream transport of this build, one line each.
var All = []internet.Transport{
	{Name: "tcp", Dial: tcp.Dial, Listen: tcp.ListenTCP},
	{Name: "mkcp", Dial: kcp.DialKCP, Listen: kcp.ListenKCP},
	{Name: "websocket", Dial: websocket.Dial, Listen: websocket.ListenWS},
	{Name: "httpupgrade", Dial: httpupgrade.Dial, Listen: httpupgrade.ListenHTTPUpgrade},
	{Name: "grpc", Dial: grpc.Dial, Listen: grpc.Listen},
	{Name: "splithttp", Dial: splithttp.Dial, Listen: splithttp.ListenXH},
	{Name: "masque", Dial: masque.Dial, Listen: masque.Listen},
	{Name: "hysteria", Dial: hysteria.Dial, Listen: hysteria.Listen},
	{Name: "xdrive", Dial: xdrive.Dial, Listen: xdrive.Serve},
	// udp carries datagrams; it is dialed by internet.Dial for UDP destinations
	// and has nothing to listen on.
	{Name: "udp", Dial: udp.Dial},
}

func init() {
	common.Must(internet.SetTransports(All))
}
