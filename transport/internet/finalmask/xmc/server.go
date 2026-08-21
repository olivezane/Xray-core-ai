package xmc

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/x509"
	"fmt"
	"net"

	"github.com/xtls/xray-core/transport/internet"
)

// Response by vanilla 26.1.2 server.
var statusResponse = `{"description":"A Minecraft Server","players":{"max":20,"online":0},"version":{"name":"26.1.2","protocol":775},"enforcesSecureChat":true}`

// serverConn is the XMC server mask connection: the Minecraft login protocol
// (including the ping/status interlude) as a handshake script, with the
// connection lifecycle owned by the shared internet.StatefulConn module.
type serverConn struct {
	*internet.StatefulConn

	profiles        []loginProfile
	password        string
	rsaPrivateKey   *rsa.PrivateKey
	rsaPublicKey    []byte
	paddingSchedule []paddingTurn
}

func wrapConnServer(c net.Conn, profiles []loginProfile, password string, rsaPrivateKeyDER []byte, rsaPublicKey []byte) (*serverConn, error) {
	if len(profiles) == 0 {
		return nil, fmt.Errorf("empty profiles")
	}
	if len(rsaPrivateKeyDER) == 0 {
		return nil, fmt.Errorf("empty rsa private key")
	}
	if len(rsaPublicKey) == 0 {
		return nil, fmt.Errorf("empty rsa public key")
	}
	rsaPrivateKey, err := x509.ParsePKCS1PrivateKey(rsaPrivateKeyDER)
	if err != nil {
		return nil, fmt.Errorf("parse rsa private key: %w", err)
	}
	paddingSchedule, err := newServerPaddingSchedule2612()
	if err != nil {
		return nil, fmt.Errorf("select padding profile: %w", err)
	}

	s := &serverConn{
		profiles:        profiles,
		password:        password,
		rsaPrivateKey:   rsaPrivateKey,
		rsaPublicKey:    rsaPublicKey,
		paddingSchedule: paddingSchedule,
	}
	s.StatefulConn = internet.NewStatefulConn(c, bufio.NewReader(c), s.serverHandshake)
	return s, nil
}

func (c *serverConn) serverHandshake(h *internet.Handshake) error {
	var (
		protocolVersion Varint
		serverAddress   String
		serverPort      UnsignedShort
		nextState       Varint
	)

	// handshake packet

	pkt, err := readPacket(h.Reader)
	if err != nil {
		return fmt.Errorf("read handshake packet: %w", err)
	}

	if pkt.packetID != 0 {
		return fmt.Errorf("bad handshake packet id")
	}

	err = pkt.readFields(&protocolVersion, &serverAddress, &serverPort, &nextState)
	if err != nil {
		return fmt.Errorf("read handshake packet: %w", err)
	}

	switch nextState {
	case 1:

		// Ping

		for range 2 {

			pkt, err := readPacket(h.Reader)
			if err != nil {
				return fmt.Errorf("read packet: %w", err)
			}

			switch pkt.packetID {
			case 0: // Status Request

				err = writePacket(h.Writer, 0, new(String(statusResponse)))
				if err != nil {
					return fmt.Errorf("write status response: %w", err)
				}

			case 1: // Ping

				var payload Long
				err = pkt.readFields(&payload)
				if err != nil {
					return fmt.Errorf("read ping packet: %w", err)
				}

				err = writePacket(h.Writer, 1, &payload)
				if err != nil {
					return fmt.Errorf("write ping response: %w", err)
				}

			}

		}

		return fmt.Errorf("ping")

	case 2:

		// Login

		// login start

		pkt, err := readPacket(h.Reader)
		if err != nil {
			return fmt.Errorf("read login start packet: %w", err)
		}

		if pkt.packetID != 0 {
			return fmt.Errorf("bad login start packet id")
		}

		var (
			username String
			uuid     UUID
		)

		err = pkt.readFields(&username, &uuid)
		if err != nil {
			return fmt.Errorf("read login start packet: %w", err)
		}
		profile, found := findProfile(c.profiles, string(username), uuid)

		// encrypt request

		var (
			serverId           String  = String("")
			publicKey          Bytes   = Bytes(c.rsaPublicKey)
			verifyToken        Bytes   = Bytes(make([]byte, 4))
			shouldAuthenticate Boolean = true
		)

		if _, err = rand.Read(verifyToken); err != nil {
			return fmt.Errorf("generate verify token: %w", err)
		}

		err = writePacket(h.Writer, 0x01, &serverId, &publicKey, &verifyToken, &shouldAuthenticate)
		if err != nil {
			return fmt.Errorf("write encryption request: %w", err)
		}

		// encrypt response

		var (
			encryptedSharedSecret Bytes
			encryptedVerifyToken  Bytes

			sharedSecret         []byte
			decryptedVerifyToken []byte
		)

		pkt, err = readPacket(h.Reader)
		if err != nil {
			return fmt.Errorf("read encrypt response: %w", err)
		}

		if pkt.packetID != 0x01 {
			return fmt.Errorf("bad encrypt response packet id")
		}

		err = pkt.readFields(&encryptedSharedSecret, &encryptedVerifyToken)
		if err != nil {
			return fmt.Errorf("read encrypt response: %w", err)
		}

		sharedSecret, err = rsa.DecryptPKCS1v15(rand.Reader, c.rsaPrivateKey, encryptedSharedSecret)
		if err != nil {
			return fmt.Errorf("decrypt shared secret: %w", err)
		}
		if len(sharedSecret) != 16 {
			return fmt.Errorf("bad shared secret length: %d", len(sharedSecret))
		}

		decryptedVerifyToken, err = rsa.DecryptPKCS1v15(rand.Reader, c.rsaPrivateKey, encryptedVerifyToken)
		if err != nil {
			return fmt.Errorf("decrypt verify token: %w", err)
		}

		if len(decryptedVerifyToken) < 4 || !bytes.Equal(verifyToken, decryptedVerifyToken[:4]) {
			return fmt.Errorf("verify token mismatch")
		}

		h.Reader, err = newCryptoReader(h.Reader, sharedSecret)
		if err != nil {
			return fmt.Errorf("new crypto reader: %w", err)
		}

		h.Writer, err = newCryptoWriter(h.Writer, sharedSecret)
		if err != nil {
			return fmt.Errorf("new crypto writer: %w", err)
		}

		// verify password
		receivedPassword := decryptedVerifyToken[4:]

		if subtle.ConstantTimeCompare(receivedPassword, []byte(c.password)) != 1 {
			writeDisconnectPacket(h.Writer, `{"type":"translatable","translate":"multiplayer.disconnect.authservers_down"}`)
			return fmt.Errorf("bad password")
		}
		if !found {
			if err = writeDisconnectPacket(h.Writer, `{"text":"You are not white-listed on this server!"}`); err != nil {
				return fmt.Errorf("write unknown login profile disconnect: %w", err)
			}
			return fmt.Errorf("unknown login profile")
		}

		loginName := String(profile.Username)
		propertyCount := Varint(1)
		propertyName := String("textures")
		texturesValue := String(profile.TexturesValue)
		signed := Boolean(true)
		texturesSignature := String(profile.TexturesSignature)
		if err = writePacket(h.Writer, 0x02, &profile.UUID, &loginName, &propertyCount, &propertyName, &texturesValue, &signed, &texturesSignature); err != nil {
			return fmt.Errorf("write login finished: %w", err)
		}

		var loginAcknowledgedLength int
		pkt, loginAcknowledgedLength, err = readPacketWithLength(h.Reader)
		if err != nil {
			return fmt.Errorf("read login acknowledged: %w", err)
		}
		if err = validateLoginAcknowledgedPacket(pkt); err != nil {
			return err
		}
		if err = runPaddingSchedule(h.Reader, h.Writer, false, loginAcknowledgedLength, c.paddingSchedule); err != nil {
			return fmt.Errorf("run startup padding: %w", err)
		}

		return h.Commit(newPacketStream(h.Reader, h.Writer, false))

	default:
		return fmt.Errorf("bad handshake packet: bad next state: %d", nextState)
	}
}

func validateLoginAcknowledgedPacket(pkt *mcPacket) error {
	if pkt.packetID != 0x03 {
		return fmt.Errorf("bad login acknowledged packet id: %d", pkt.packetID)
	}
	if len(pkt.data) != 0 {
		return fmt.Errorf("bad login acknowledged packet data length: %d", len(pkt.data))
	}
	return nil
}
