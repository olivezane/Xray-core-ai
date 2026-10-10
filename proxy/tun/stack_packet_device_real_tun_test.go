//go:build tuntest

package tun

import (
	"bytes"
	"encoding/binary"
	stdnet "net"
	"net/netip"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// openRealTun opens a real kernel TUN device carrying bare IP packets, the way
// the Linux device does, and gives it an address the kernel routes.
func openRealTun(t *testing.T, nonblocking bool) int {
	t.Helper()

	fd, err := unix.Open("/dev/net/tun", unix.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open /dev/net/tun: %v", err)
	}
	ifr, err := unix.NewIfreq("xraytest0")
	if err != nil {
		t.Fatalf("build ifreq: %v", err)
	}
	// IFF_TUN | IFF_NO_PI: bare IP packets, exactly like the Linux device
	ifr.SetUint16(unix.IFF_TUN | unix.IFF_NO_PI)
	if err := unix.IoctlIfreq(fd, unix.TUNSETIFF, ifr); err != nil {
		unix.Close(fd)
		t.Fatalf("TUNSETIFF (needs CAP_NET_ADMIN): %v", err)
	}
	if nonblocking {
		if err := unix.SetNonblock(fd, true); err != nil {
			t.Fatalf("set non-blocking: %v", err)
		}
	}

	// the interface has to be up and addressed, since the kernel drops a write
	// to a device that is down and routes nothing to an unaddressed one
	for _, args := range [][]string{
		{"link", "set", "dev", "xraytest0", "up"},
		{"addr", "add", "10.9.9.1/24", "dev", "xraytest0"},
	} {
		if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
			t.Fatalf("ip %v: %v (%s)", args, err, out)
		}
	}
	t.Cleanup(func() { _ = exec.Command("ip", "link", "del", "xraytest0").Run() })
	t.Cleanup(func() { unix.Close(fd) })

	return fd
}

// readFromDevice reads one packet off the device, waiting up to the timeout
func readFromDevice(t *testing.T, device *fdPacketDevice, timeout time.Duration) []byte {
	t.Helper()

	buffers := [][]byte{make([]byte, 1500)}
	deadline := time.Now().Add(timeout)
	for {
		n, err := device.ReadPackets(buffers)
		if err != nil && err != ErrQueueEmpty {
			t.Fatalf("read error: %v", err)
		}
		if n > 0 {
			return append([]byte(nil), buffers[0][:packetLength(buffers[0])]...)
		}
		if time.Now().After(deadline) {
			t.Fatal("the device produced no packet")
		}
		device.Wait()
	}
}

// packetLength reports how long a complete IP packet is, from its own header
func packetLength(packet []byte) int {
	if len(packet) < 20 || packet[0]>>4 != 4 {
		return len(packet)
	}
	if total := int(binary.BigEndian.Uint16(packet[2:4])); total <= len(packet) {
		return total
	}
	return len(packet)
}

// TestRealTunDeviceReadsAndWritesBatches drives the fd Packet Device against a
// real kernel TUN descriptor, which is the only place the poll-then-read loop
// meets the device semantics rather than a socket pair standing in for them.
func TestRealTunDeviceReadsAndWritesBatches(t *testing.T) {
	for _, nonblocking := range []bool{true, false} {
		name := map[bool]string{true: "non-blocking", false: "blocking"}[nonblocking]
		t.Run(name, func(t *testing.T) {
			fd := openRealTun(t, nonblocking)
			device := &fdPacketDevice{fd: fd}

			// an empty read must report an empty queue rather than park the
			// caller: that is what lets the loop keep the descriptor blocking
			empty := make(chan error, 1)
			go func() {
				_, err := device.ReadPackets([][]byte{make([]byte, 1500)})
				empty <- err
			}()
			select {
			case err := <-empty:
				if err != nil && err != ErrQueueEmpty {
					t.Fatalf("read error = %v on a quiet device", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("reading a quiet real tun device parked the caller")
			}

			// Wait must return within its bound on a quiet device
			start := time.Now()
			device.Wait()
			elapsed := time.Since(start)
			if elapsed > 3*time.Second {
				t.Fatalf("Wait took %s on a quiet device, want a bounded park", elapsed)
			}
			t.Logf("%s: empty read and Wait both returned (Wait %s)", name, elapsed)

			// read path: a local socket sends a datagram out of the tun, and
			// the device has to hand the very packet the kernel emitted to the
			// stack
			host, err := stdnet.ListenUDP("udp4", stdnet.UDPAddrFromAddrPort(netip.MustParseAddrPort("10.9.9.1:0")))
			if err != nil {
				t.Fatalf("listen on the tun address: %v", err)
			}
			defer host.Close()

			payload := []byte("xray")
			if _, err := host.WriteToUDP(payload, stdnet.UDPAddrFromAddrPort(netip.MustParseAddrPort("10.9.9.2:9999"))); err != nil {
				t.Fatalf("send through the tun: %v", err)
			}

			emitted := readFromDevice(t, device, 5*time.Second)
			if len(emitted) < 28 || emitted[9] != 17 {
				t.Fatalf("the device handed over a %d byte packet with protocol %d, want a udp packet", len(emitted), emitted[9])
			}
			if got := netip.AddrFrom4([4]byte(emitted[16:20])); got != netip.MustParseAddr("10.9.9.2") {
				t.Fatalf("the packet is addressed to %s, want the destination the socket sent to", got)
			}
			if !bytes.HasSuffix(emitted, payload) {
				t.Fatalf("the packet does not carry the payload the socket sent: % x", emitted)
			}
			t.Logf("%s: read the packet the kernel emitted (%d bytes)", name, len(emitted))

			// write path: a packet written to the device has to reach a socket
			// bound on the address the packet is addressed to
			server, err := stdnet.ListenUDP("udp4", stdnet.UDPAddrFromAddrPort(netip.MustParseAddrPort("10.9.9.1:9999")))
			if err != nil {
				t.Fatalf("listen on the tun address: %v", err)
			}
			defer server.Close()

			injected := buildUDPPacket(t, netip.MustParseAddrPort("10.9.9.2:1234"), netip.MustParseAddrPort("10.9.9.1:9999"), []byte("from-stack"))
			if err := device.WritePackets([][]byte{injected}); err != nil {
				t.Fatalf("write error: %v", err)
			}

			if err := server.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatalf("set the read deadline: %v", err)
			}
			received := make([]byte, 64)
			n, _, err := server.ReadFromUDP(received)
			if err != nil {
				t.Fatalf("the device's packet never reached the socket: %v", err)
			}
			if !bytes.Equal(received[:n], []byte("from-stack")) {
				t.Fatalf("the socket received %q, want %q", received[:n], "from-stack")
			}
			t.Logf("%s: a packet written to the device reached a socket on the kernel side", name)

			// batch path: many packets the kernel has waiting for the device have to
			// come back in ONE call, which is the whole point of the native device.
			// They are produced by real kernel sends, not injected into the device,
			// so they are packets the device would really have to carry.
			const batch = 16
			for i := range batch {
				if _, err := host.WriteToUDP([]byte{byte(i)}, stdnet.UDPAddrFromAddrPort(netip.MustParseAddrPort("10.9.9.2:9999"))); err != nil {
					t.Fatalf("send datagram %d through the tun: %v", i, err)
				}
			}

			buffers := make([][]byte, batch)
			for i := range buffers {
				buffers[i] = make([]byte, 1500)
			}
			var count, biggest int
			deadline := time.Now().Add(5 * time.Second)
			for count < batch && time.Now().Before(deadline) {
				n, err := device.ReadPackets(buffers[count:])
				if err != nil && err != ErrQueueEmpty {
					t.Fatalf("read error: %v", err)
				}
				count += n
				if n > biggest {
					biggest = n
				}
				if n == 0 {
					device.Wait()
				}
			}
			if count != batch {
				t.Fatalf("read %d packets, want the %d the kernel sent", count, batch)
			}
			// the point of the native device: one call has to be able to return
			// more than the single packet the gVisor adapter could
			if biggest < 2 {
				t.Fatalf("the biggest batch was %d packet(s), want several from one call", biggest)
			}
			t.Logf("%s: drained %d packets off a real tun device, up to %d in one call", name, count, biggest)
		})
	}
}

// buildUDPPacket builds an IPv4 UDP packet from source to destination
func buildUDPPacket(t *testing.T, source, destination netip.AddrPort, payload []byte) []byte {
	t.Helper()

	packet := make([]byte, 20+8+len(payload))
	packet[0] = 0x45
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	packet[8] = 64
	packet[9] = 17 // UDP
	src, dst := source.Addr().As4(), destination.Addr().As4()
	copy(packet[12:16], src[:])
	copy(packet[16:20], dst[:])
	binary.BigEndian.PutUint16(packet[10:12], internetChecksum(packet[:20]))

	udp := packet[20:]
	binary.BigEndian.PutUint16(udp[0:2], source.Port())
	binary.BigEndian.PutUint16(udp[2:4], destination.Port())
	binary.BigEndian.PutUint16(udp[4:6], uint16(8+len(payload)))
	copy(udp[8:], payload)

	// the checksum covers the pseudo header, the udp header and the payload
	pseudo := make([]byte, 0, 12+len(udp))
	pseudo = append(pseudo, src[:]...)
	pseudo = append(pseudo, dst[:]...)
	pseudo = append(pseudo, 0, 17)
	pseudo = binary.BigEndian.AppendUint16(pseudo, uint16(len(udp)))
	binary.BigEndian.PutUint16(udp[6:8], internetChecksum(append(pseudo, udp...)))

	return packet
}

// internetChecksum computes the internet checksum of a buffer, as the kernel does
func internetChecksum(buffer []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(buffer); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(buffer[i : i+2]))
	}
	if len(buffer)%2 == 1 {
		sum += uint32(buffer[len(buffer)-1]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}
