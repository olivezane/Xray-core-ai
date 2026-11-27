package conf

import (
	"context"
	"slices"
	"strings"

	utls "github.com/refraction-networking/utls"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/serial"
	coreconfig "github.com/xtls/xray-core/core/config"
	"github.com/xtls/xray-core/transport/internet"
	"github.com/xtls/xray-core/transport/internet/tls"
)

// TransportProtocol is a transport name as a configuration writes it.
type TransportProtocol = coreconfig.TransportProtocol

// buildTransportProtocol turns a transport name of the configuration into the
// protocol name of the transport it selects.
func buildTransportProtocol(p coreconfig.TransportProtocol) (string, error) {
	switch strings.ToLower(string(p)) {
	case "raw", "tcp":
		return "tcp", nil
	case "xhttp", "splithttp":
		return "splithttp", nil
	case "kcp", "mkcp":
		return "mkcp", nil
	case "grpc":
		errors.PrintNonRemovalDeprecatedFeatureWarning("gRPC transport (with unnecessary costs, etc.)", "XHTTP stream-up H2")
		return "grpc", nil
	case "ws", "websocket":
		errors.PrintNonRemovalDeprecatedFeatureWarning("WebSocket transport (with ALPN http/1.1, etc.)", "XHTTP H2 & H3")
		return "websocket", nil
	case "httpupgrade":
		errors.PrintNonRemovalDeprecatedFeatureWarning("HTTPUpgrade transport (with ALPN http/1.1, etc.)", "XHTTP H2 & H3")
		return "httpupgrade", nil
	case "h2", "h3", "http":
		return "", errors.PrintRemovedFeatureError("HTTP transport (without header padding, etc.)", "XHTTP stream-one H2 & H3")
	case "quic":
		return "", errors.PrintRemovedFeatureError("QUIC transport (without web service, etc.)", "XHTTP stream-one H3")
	case "hysteria":
		return "hysteria", nil
	case "masque":
		return "masque", nil
	case "xdrive":
		return "xdrive", nil
	default:
		return "", errors.New("Config: unknown transport protocol: ", p)
	}
}

// StreamConfig is the "streamSettings" section of the configuration.
type StreamConfig struct {
	coreconfig.Stream
}

// Build implements Buildable.
// Build converts the section to its message.
func (c *StreamConfig) Build() (*internet.StreamConfig, error) {
	return buildStream(&c.Stream)
}

func buildStream(c *coreconfig.Stream) (*internet.StreamConfig, error) {
	config := &internet.StreamConfig{
		Port:         uint32(c.Port),
		ProtocolName: "tcp",
	}
	if c.Address != nil {
		config.Address = BuildAddress(c.Address)
	}
	if c.Method != nil {
		c.Network = c.Method
	}
	if c.Network != nil {
		protocol, err := buildTransportProtocol(*c.Network)
		if err != nil {
			return nil, err
		}
		config.ProtocolName = protocol
	}

	var xdriveH3 bool
	switch strings.ToLower(c.Security) {
	case "", "none":
	case "tls":
		tlsSettings := c.TLSSettings
		if tlsSettings == nil {
			empty := coreconfig.TLS{}
			tlsSettings = &empty
		}
		ts, err := (&TLSConfig{*tlsSettings}).Build()
		if err != nil {
			return nil, errors.New("Failed to build TLS config.").Base(err)
		}
		tm := serial.ToTypedMessage(ts)
		config.SecuritySettings = append(config.SecuritySettings, tm)
		config.SecurityType = tm.Type
		xdriveH3 = config.ProtocolName == "xdrive" && tlsSettings.ALPN != nil && slices.Equal(*tlsSettings.ALPN, StringList{"h3"})
		if config.ProtocolName == "xdrive" && tlsSettings.ALPN != nil && slices.Equal(*tlsSettings.ALPN, StringList{"http/1.1"}) && tlsSettings.ECHConfigList == "" {
			// a fingerprint puts its own ALPN into the hello, except Go TLS and the
			// randomized ones, which take it from the config or send none
			if fp := tls.GetFingerprint(strings.ToLower(tlsSettings.Fingerprint)); fp != nil && fp.Client != utls.HelloGolang.Client &&
				!strings.HasPrefix(fp.Client, utls.HelloRandomized.Client) {
				errors.LogWarning(context.Background(), `XDRIVE: "alpn" ["http/1.1"] reaches the wire only inside ECH, without "echConfigList" the hello carries the ALPN of the fingerprint and the server may pick h2`)
			}
		}
	case "reality":
		if config.ProtocolName != "tcp" && config.ProtocolName != "splithttp" && config.ProtocolName != "grpc" {
			return nil, errors.New("REALITY only supports RAW, XHTTP and gRPC for now.")
		}
		if c.REALITYSettings == nil {
			return nil, errors.New(`REALITY: Empty "realitySettings".`)
		}
		ts, err := (&REALITYConfig{*c.REALITYSettings}).Build()
		if err != nil {
			return nil, errors.New("Failed to build REALITY config.").Base(err)
		}
		tm := serial.ToTypedMessage(ts)
		config.SecuritySettings = append(config.SecuritySettings, tm)
		config.SecurityType = tm.Type
	case "xtls":
		return nil, errors.PrintRemovedFeatureError(`Legacy XTLS`, `xtls-rprx-vision with TLS or REALITY`)
	default:
		return nil, errors.New(`Unknown security "` + c.Security + `".`)
	}

	if c.RAWSettings != nil {
		c.TCPSettings = c.RAWSettings
	}
	if c.TCPSettings != nil {
		ts, err := (&TCPConfig{*c.TCPSettings}).Build()
		if err != nil {
			return nil, errors.New("Failed to build RAW config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "tcp",
			Settings:     serial.ToTypedMessage(ts),
		})
	}
	if c.XHTTPSettings != nil {
		c.SplitHTTPSettings = c.XHTTPSettings
	}
	if c.SplitHTTPSettings != nil {
		hs, err := (&SplitHTTPConfig{*c.SplitHTTPSettings}).Build()
		if err != nil {
			return nil, errors.New("Failed to build XHTTP config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "splithttp",
			Settings:     serial.ToTypedMessage(hs),
		})
	}
	if c.KCPSettings != nil {
		ts, err := (&KCPConfig{*c.KCPSettings}).Build()
		if err != nil {
			return nil, errors.New("Failed to build mKCP config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "mkcp",
			Settings:     serial.ToTypedMessage(ts),
		})
	}
	if c.GRPCSettings != nil {
		gs, err := (&GRPCConfig{*c.GRPCSettings}).Build()
		if err != nil {
			return nil, errors.New("Failed to build gRPC config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "grpc",
			Settings:     serial.ToTypedMessage(gs),
		})
	}
	if c.WSSettings != nil {
		ts, err := (&WebSocketConfig{*c.WSSettings}).Build()
		if err != nil {
			return nil, errors.New("Failed to build WebSocket config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "websocket",
			Settings:     serial.ToTypedMessage(ts),
		})
	}
	if c.HTTPUPGRADESettings != nil {
		hs, err := (&HttpUpgradeConfig{*c.HTTPUPGRADESettings}).Build()
		if err != nil {
			return nil, errors.New("Failed to build HTTPUpgrade config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "httpupgrade",
			Settings:     serial.ToTypedMessage(hs),
		})
	}
	if c.HysteriaSettings != nil {
		hs, err := (&HysteriaConfig{*c.HysteriaSettings}).Build()
		if err != nil {
			return nil, errors.New("Failed to build Hysteria config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "hysteria",
			Settings:     serial.ToTypedMessage(hs),
		})
	}
	if c.MASQUESettings != nil {
		ms, err := (&MasqueConfig{*c.MASQUESettings}).Build()
		if err != nil {
			return nil, errors.New("Failed to build MASQUE config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "masque",
			Settings:     serial.ToTypedMessage(ms),
		})
	}
	if c.XDRIVESettings != nil {
		xdrive := &XDriveConfig{XDrive: *c.XDRIVESettings}
		xdrive.h3 = xdriveH3
		xs, err := xdrive.Build()
		if err != nil {
			return nil, errors.New("Failed to build XDRIVE config.").Base(err)
		}
		config.TransportSettings = append(config.TransportSettings, &internet.TransportConfig{
			ProtocolName: "xdrive",
			Settings:     serial.ToTypedMessage(xs),
		})
	}
	if c.SocketSettings != nil {
		ss, err := (&SocketConfig{*c.SocketSettings}).Build()
		if err != nil {
			return nil, errors.New("Failed to build sockopt.").Base(err)
		}
		config.SocketSettings = ss
	}

	if c.FinalMask != nil {
		for _, mask := range c.FinalMask.Tcp {
			proto, err := buildMask(&mask, true)
			if err != nil {
				return nil, errors.New("failed to build mask with type ", mask.Type).Base(err)
			}
			config.Tcpmasks = append(config.Tcpmasks, serial.ToTypedMessage(proto))
		}
		for _, mask := range c.FinalMask.Udp {
			proto, err := buildMask(&mask, false)
			if err != nil {
				return nil, errors.New("failed to build mask with type ", mask.Type).Base(err)
			}
			config.Udpmasks = append(config.Udpmasks, serial.ToTypedMessage(proto))
		}
		if c.FinalMask.QuicParams != nil {
			proto, err := (&QuicParamsConfig{*c.FinalMask.QuicParams}).Build()
			if err != nil {
				return nil, err
			}
			config.QuicParams = proto.(*internet.QuicParams)
		}
	}

	if xdriveH3 && c.FinalMask != nil {
		if c.XDRIVESettings != nil && c.XDRIVESettings.Service == "google-drive" {
			// noise only sends datagrams of its own ahead of the QUIC ones, the
			// other masks change those or need a peer that undoes them
			for _, mask := range c.FinalMask.Udp {
				if strings.ToLower(mask.Type) != "noise" {
					return nil, errors.New(`XDRIVE: over h3 to Google Drive the only "udp" mask that works is "noise", not "`, mask.Type, `"`)
				}
			}
		}
		if config.QuicParams != nil && (config.QuicParams.Congestion == "brutal" || config.QuicParams.Congestion == "force-brutal") {
			return nil, errors.New(`XDRIVE: brutal is for Hysteria servers, over h3 "congestion" is "bbr" or "reno", not "`, config.QuicParams.Congestion, `"`)
		}
	}

	return config, nil
}
