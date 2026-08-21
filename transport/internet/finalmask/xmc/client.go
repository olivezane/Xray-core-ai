package xmc

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"math/big"
	"net"
	"strconv"

	"github.com/xtls/xray-core/transport/internet"
)

// clientConn is the XMC client mask connection: the Minecraft login protocol
// as a handshake script, with the connection lifecycle owned by the shared
// internet.StatefulConn module.
type clientConn struct {
	*internet.StatefulConn

	profiles        []loginProfile
	password        string
	rsaPublicKey    []byte
	hostname        string
	paddingSchedule []paddingTurn
}

func newClientConn(c net.Conn, profiles []loginProfile, password string, rsaPublicKey []byte, hostname string) (*clientConn, error) {
	if len(rsaPublicKey) == 0 {
		return nil, fmt.Errorf("empty rsa public key")
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("empty profiles")
	}
	paddingSchedule, err := newClientPaddingSchedule2612()
	if err != nil {
		return nil, fmt.Errorf("select padding profile: %w", err)
	}
	cc := &clientConn{
		profiles:        profiles,
		password:        password,
		rsaPublicKey:    rsaPublicKey,
		hostname:        hostname,
		paddingSchedule: paddingSchedule,
	}
	cc.StatefulConn = internet.NewStatefulConn(c, bufio.NewReader(c), cc.clientHandshake)
	return cc, nil
}

func (c *clientConn) clientHandshake(h *internet.Handshake) error {
	var (
		protocolVersion Varint        = Varint(775)
		serverAddress   String        = String(c.hostname)
		serverPort      UnsignedShort = UnsignedShort(25565)
		nextState       Varint        = Varint(2)
	)

	host, portString, err := net.SplitHostPort(c.RemoteAddr().String())
	if err == nil {
		port, err := strconv.Atoi(portString)
		if err == nil {
			serverPort = UnsignedShort(port)
		}

		if serverAddress == "" {
			serverAddress = String(host)
		}
	}

	err = writePacket(h.Writer, 0x00, &protocolVersion, &serverAddress, &serverPort, &nextState)
	if err != nil {
		return fmt.Errorf("write handshake packet: %w", err)
	}

	// Login Start
	randomProfile, err := rand.Int(rand.Reader, big.NewInt(int64(len(c.profiles))))
	if err != nil {
		return fmt.Errorf("select profile: %w", err)
	}
	selectedProfile := c.profiles[randomProfile.Int64()]
	username := String(selectedProfile.Username)

	err = writePacket(h.Writer, 0x00, &username, &selectedProfile.UUID)
	if err != nil {
		return fmt.Errorf("write login start: %w", err)
	}

	// Encryption Request
	pkt, err := readPacket(h.Reader)
	if err != nil {
		return fmt.Errorf("read encryption request: %w", err)
	}

	if pkt.packetID != 0x01 {
		return fmt.Errorf("bad encrypt request packet id")
	}

	var (
		serverId    String
		publicKey   Bytes
		verifyToken Bytes
	)

	err = pkt.readFields(&serverId, &publicKey, &verifyToken)
	if err != nil {
		return fmt.Errorf("read encryption request fields: %w", err)
	}

	if !bytes.Equal(publicKey, c.rsaPublicKey) {
		return fmt.Errorf("server public key mismatch")
	}

	k, err := x509.ParsePKIXPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("parse server public key: %w", err)
	}

	rsaPublicKey, ok := k.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("parse server public key: not rsa")
	}

	sharedSecret := make([]byte, 16)
	if _, err = rand.Read(sharedSecret); err != nil {
		return fmt.Errorf("generate shared secret: %w", err)
	}

	encryptedSharedSecret, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPublicKey, sharedSecret)
	if err != nil {
		return fmt.Errorf("encrypt shared secret: %w", err)
	}

	verifyToken = append(verifyToken, []byte(c.password)...) // append pre-shared password

	encryptedVerifyToken, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPublicKey, verifyToken)
	if err != nil {
		return fmt.Errorf("encrypt verify token: %w", err)
	}

	// Send Encryption Response
	err = writePacket(
		h.Writer,
		0x01,
		(*Bytes)(&encryptedSharedSecret),
		(*Bytes)(&encryptedVerifyToken),
	)
	if err != nil {
		return fmt.Errorf("write encryption response: %w", err)
	}

	// Enable encryption
	h.Reader, err = newCryptoReader(h.Reader, sharedSecret)
	if err != nil {
		return fmt.Errorf("new crypto reader: %w", err)
	}

	h.Writer, err = newCryptoWriter(h.Writer, sharedSecret)
	if err != nil {
		return fmt.Errorf("new crypto writer: %w", err)
	}

	pkt, err = readPacket(h.Reader)
	if err != nil {
		return fmt.Errorf("read login finished: %w", err)
	}
	if pkt.packetID == 0x00 {
		var reason String
		if readErr := pkt.readFields(&reason); readErr != nil {
			return fmt.Errorf("authentication rejected")
		}
		return fmt.Errorf("authentication rejected: %s", reason)
	}
	if pkt.packetID != 0x02 {
		return fmt.Errorf("bad login finished packet id: %d", pkt.packetID)
	}

	receivedProfile, err := readLoginSuccess(pkt)
	if err != nil {
		return fmt.Errorf("read login finished fields: %w", err)
	}
	if receivedProfile != selectedProfile {
		return fmt.Errorf("login profile mismatch")
	}
	loginAcknowledgedLength, err := writePacketWithLength(h.Writer, 0x03)
	if err != nil {
		return fmt.Errorf("write login acknowledged: %w", err)
	}
	if err = runPaddingSchedule(h.Reader, h.Writer, true, loginAcknowledgedLength, c.paddingSchedule); err != nil {
		return fmt.Errorf("run startup padding: %w", err)
	}

	return h.Commit(newPacketStream(h.Reader, h.Writer, true))
}
