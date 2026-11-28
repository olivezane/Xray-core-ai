// Package transports is the one place where Xray wires its stream transports.
//
// Each line names a protocol and gives the settings it decodes to, the function
// that dials with it and the one that listens with it, so this file is the whole
// picture: there is no registration from init() functions scattered over the
// transport packages, and nothing else calls into a transport to make it usable.
//
// Adding a transport means adding its line here, a NewConfig in its package and
// an entry in transport/internet/transports/transports_test.go.
package transports

import (
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/grpc"
	"github.com/xtls/xray-core/transport/internet/httpupgrade"
	"github.com/xtls/xray-core/transport/internet/hysteria"
	"github.com/xtls/xray-core/transport/internet/kcp"
	"github.com/xtls/xray-core/transport/internet/reality"
	"github.com/xtls/xray-core/transport/internet/masque"
	"github.com/xtls/xray-core/transport/internet/splithttp"
	"github.com/xtls/xray-core/transport/internet/tcp"
	"github.com/xtls/xray-core/transport/internet/tls"
	"github.com/xtls/xray-core/transport/internet/udp"
	"github.com/xtls/xray-core/transport/internet/websocket"
	"github.com/xtls/xray-core/transport/internet/xdrive"
)

// All is every stream transport of this build, one line each.
var All = []internet.Transport{
	{Name: "tcp", Config: internet.ConfigOf(tcp.NewConfig), Dial: tcp.Dial, Listen: tcp.ListenTCP},
	{Name: "mkcp", Config: internet.ConfigOf(kcp.NewConfig), Dial: kcp.DialKCP, Listen: kcp.ListenKCP},
	{Name: "websocket", Config: internet.ConfigOf(websocket.NewConfig), Dial: websocket.Dial, Listen: websocket.ListenWS},
	{Name: "httpupgrade", Config: internet.ConfigOf(httpupgrade.NewConfig), Dial: httpupgrade.Dial, Listen: httpupgrade.ListenHTTPUpgrade},
	{Name: "grpc", Config: internet.ConfigOf(grpc.NewConfig), Dial: grpc.Dial, Listen: grpc.Listen},
	{Name: "splithttp", Config: internet.ConfigOf(splithttp.NewConfig), Dial: splithttp.Dial, Listen: splithttp.ListenXH},
	{Name: "masque", Config: internet.ConfigOf(masque.NewConfig), Dial: masque.Dial, Listen: masque.Listen},
	{Name: "hysteria", Config: internet.ConfigOf(hysteria.NewConfig), Dial: hysteria.Dial, Listen: hysteria.Listen},
	{Name: "xdrive", Config: internet.ConfigOf(xdrive.NewConfig), Dial: xdrive.Dial, Listen: xdrive.Serve},
	// udp carries datagrams; it is dialed by internet.Dial for UDP destinations
	// and has nothing to listen on.
	{Name: "udp", Config: internet.ConfigOf(udp.NewConfig), Dial: udp.Dial},
}

// Security is every stream security layer of this build, one line each. A
// security layer wraps a transport rather than being one, so it is listed here
// beside the transports: its settings cross the API by name the same way.
var Security = []interface{}{
	(*tls.Config)(nil),
	(*reality.Config)(nil),
	(*xdrive.Config)(nil),
}

func init() {
	common.Must(internet.SetTransports(All))
	common.Must(internet.SetStreamSettings(Security))
}
