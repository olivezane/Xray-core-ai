package reality

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha512"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

// realityCert returns an ed25519 certificate whose signature is replaced by the
// HMAC of its public key, which is how REALITY proves the server holds the secret.
func realityCert(t *testing.T, auth []byte) []byte {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "reality"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	if err != nil {
		t.Fatal(err)
	}
	h := hmac.New(sha512.New, auth)
	h.Write(pub)
	copy(der[len(der)-ed25519.SignatureSize:], h.Sum(nil)) // signatureValue is the last DER element
	return der
}

func TestVerifyPeerCertificate(t *testing.T) {
	auth := make([]byte, 32)
	der := realityCert(t, auth)

	c := &UConn{Config: &Config{}, ServerName: "localhost", AuthKey: auth}
	if err := c.VerifyPeerCertificate([][]byte{der}, nil); err != nil {
		t.Fatalf("authentic REALITY certificate rejected: %v", err)
	}
	if !c.Verified {
		t.Fatal("authentic REALITY certificate not marked verified")
	}

	// Same certificate, wrong key: falls through to x509 verification and fails.
	c = &UConn{Config: &Config{}, ServerName: "localhost", AuthKey: []byte("wrong-key")}
	if err := c.VerifyPeerCertificate([][]byte{der}, nil); err == nil {
		t.Fatal("certificate accepted with the wrong auth key")
	}
}

func TestVerifyPeerCertificateEmptyChain(t *testing.T) {
	c := &UConn{Config: &Config{}, ServerName: "localhost"}
	if err := c.VerifyPeerCertificate(nil, nil); err == nil {
		t.Fatal("empty certificate chain accepted")
	}
}
