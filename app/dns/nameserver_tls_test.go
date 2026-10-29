package dns

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	stdtls "crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"io"
	"math/big"
	stdnet "net"
	"net/url"
	"testing"
	"time"

	"github.com/xtls/xray-core/common/net"
	dns_feature "github.com/xtls/xray-core/features/dns"
	"golang.org/x/net/dns/dnsmessage"
)

func TestTLSNameServerDefaultPort(t *testing.T) {
	u, err := url.Parse("tls://1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewTLSNameServer(u, nil, false, false, 0, net.IP(nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.destination.NetAddr(); got != "1.1.1.1:853" {
		t.Error("expect DNS over TLS to default to port 853 (RFC7858), but got ", got)
	}

	u, err = url.Parse("tls+local://[2606:4700:4700::1111]:8853")
	if err != nil {
		t.Fatal(err)
	}
	s, err = NewTLSLocalNameServer(u, false, false, 0, net.IP(nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.destination.NetAddr(); got != "[2606:4700:4700::1111]:8853" {
		t.Error("expect the port in the address to win, but got ", got)
	}
}

// TestTLSNameServer queries a local DNS over TLS server, so it covers the
// TLS handshake and the RFC1035 two-octet length field without the network.
func TestTLSNameServer(t *testing.T) {
	listener, err := stdtls.Listen("tcp", "127.0.0.1:0", &stdtls.Config{
		Certificates: []stdtls.Certificate{testCertificate(t)},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveOneQuery(conn)
		}
	}()

	u, err := url.Parse("tls+local://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewTLSLocalNameServer(u, false, false, 0, net.IP(nil))
	if err != nil {
		t.Fatal(err)
	}
	s.tlsConfig.InsecureSkipVerify = true // self-signed test certificate

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	ips, ttl, err := s.QueryIP(ctx, "example.com", dns_feature.IPOption{IPv4Enable: true})
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) != 1 {
		t.Fatal("expect one ip, but got ", ips)
	}
	if !bytes.Equal(ips[0], net.IP{198, 51, 100, 211}) {
		t.Fatal("unexpected ip: ", ips[0])
	}
	if ttl == 0 {
		t.Error("expect a non-zero ttl")
	}
}

func serveOneQuery(conn stdnet.Conn) {
	defer conn.Close()

	length := make([]byte, 2)
	if _, err := io.ReadFull(conn, length); err != nil {
		return
	}
	query := make([]byte, binary.BigEndian.Uint16(length))
	if _, err := io.ReadFull(conn, query); err != nil {
		return
	}

	var p dnsmessage.Parser
	h, err := p.Start(query)
	if err != nil {
		return
	}
	questions, err := p.AllQuestions()
	if err != nil {
		return
	}
	response := dnsmessage.Message{
		Header: dnsmessage.Header{
			ID:                 h.ID,
			Response:           true,
			RecursionAvailable: true,
		},
		Questions: questions,
	}
	if len(questions) == 1 && questions[0].Type == dnsmessage.TypeA {
		response.Answers = append(response.Answers, dnsmessage.Resource{
			Header: dnsmessage.ResourceHeader{
				Name:  questions[0].Name,
				Type:  dnsmessage.TypeA,
				Class: dnsmessage.ClassINET,
				TTL:   60,
			},
			Body: &dnsmessage.AResource{A: [4]byte{198, 51, 100, 211}},
		})
	}

	msg, err := response.Pack()
	if err != nil {
		return
	}
	length = make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(len(msg)))
	conn.Write(append(length, msg...))
}

func testCertificate(t *testing.T) stdtls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "dot.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []stdnet.IP{stdnet.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return stdtls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
