package protocols

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"
)

// fakeTunnelBear is a stand-in for one of the operator's endpoints. It records
// how the bytes arrived — which is the whole point of the relay — and answers
// with enough data to look alive.
type fakeTunnelBear struct {
	ln     net.Listener
	reads  chan int
	got    chan []byte
	closed chan struct{}
}

func newFakeTunnelBear(t *testing.T) *fakeTunnelBear {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not start the fake server: %v", err)
	}

	f := &fakeTunnelBear{
		ln:     ln,
		reads:  make(chan int, 4096),
		got:    make(chan []byte, 1),
		closed: make(chan struct{}),
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetNoDelay(true)
		}

		var collected []byte
		buf := make([]byte, 64*1024)
		for {
			n, readErr := conn.Read(buf)
			if n > 0 {
				f.reads <- n
				collected = append(collected, buf[:n]...)
			}
			if readErr != nil {
				select {
				case f.got <- collected:
				default:
				}
				return
			}
		}
	}()

	t.Cleanup(func() { _ = ln.Close() })
	return f
}

func (f *fakeTunnelBear) host() string {
	host, _, _ := net.SplitHostPort(f.ln.Addr().String())
	return host
}

func (f *fakeTunnelBear) port() int {
	_, portStr, _ := net.SplitHostPort(f.ln.Addr().String())
	var port int
	_, _ = fmt.Sscanf(portStr, "%d", &port)
	return port
}

// resetRecord builds the packet OpenVPN over TCP sends first: a two-byte
// big-endian length followed by that many bytes.
func resetRecord(payload []byte) []byte {
	out := make([]byte, 2, 2+len(payload))
	binary.BigEndian.PutUint16(out, uint16(len(payload)))
	return append(out, payload...)
}

// The relay must hand the first record over whole and then break up what
// follows, so that no segment boundary lines up with an OpenVPN record.
func TestSplitRelaySplitsTheHandshakeAndKeepsItIntact(t *testing.T) {
	server := newFakeTunnelBear(t)

	relay := NewSplitRelay([]string{server.host()}, server.port(), nil)
	port, err := relay.Start()
	if err != nil {
		t.Fatalf("relay did not start: %v", err)
	}
	defer func() { _ = relay.Close() }()

	client, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		t.Fatalf("could not reach the relay: %v", err)
	}
	defer func() { _ = client.Close() }()

	reset := resetRecord([]byte("hello-opcode"))
	// Comfortably more than relaySplitBytes, so the test exercises both the
	// fragmented region and the plain-copy tail.
	payload := bytes.Repeat([]byte("ABCDEFGH"), 1024) // 8 KB
	want := append(append([]byte{}, reset...), payload...)

	if _, err := client.Write(want); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	// Half-close, so the relay sees EOF on the client direction and tears the
	// upstream down. The fake server only publishes what it collected once its
	// read fails, so without this the test waits for a close that nothing will
	// send — the relay is still blocked reading from a client with nothing more
	// to say.
	if tcp, ok := client.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}

	var collected []byte
	select {
	case collected = <-server.got:
	case <-time.After(30 * time.Second):
		t.Fatal("the fake server never saw the connection close")
	}

	if !bytes.Equal(collected, want) {
		t.Fatalf("the relay corrupted the stream: sent %d bytes, received %d", len(want), len(collected))
	}

	// The reset record has to arrive as one write, because a filter can read
	// its length straight off the first two bytes and splitting it would break
	// the protocol rather than hide it.
	var sizes []int
drain:
	for {
		select {
		case n := <-server.reads:
			sizes = append(sizes, n)
		default:
			break drain
		}
	}
	if len(sizes) == 0 {
		t.Fatal("no reads were recorded")
	}
	// The record must not be torn: the first read has to contain all of it.
	// It may also contain the first byte of what follows — the relay sends that
	// immediately after, and on loopback the two can share one read — so this
	// is a lower bound, not an equality.
	if sizes[0] < len(reset) {
		t.Errorf("the first read was %d bytes; the %d-byte reset record must arrive whole",
			sizes[0], len(reset))
	}

	// Evidence that the rest was actually fragmented rather than passed
	// through: 6 KB in pieces of at most 29 bytes cannot arrive in a handful
	// of reads.
	if len(sizes) < 20 {
		t.Errorf("expected the handshake to arrive in many small pieces, saw %d reads: %v", len(sizes), sizes)
	}

	// Every piece after the reset and inside the split budget is small.
	small := 0
	for _, n := range sizes[1:] {
		if n <= 29 {
			small++
		}
	}
	if small < 20 {
		t.Errorf("only %d of %d reads were small pieces", small, len(sizes)-1)
	}
}

// The first record's two-byte length prefix is part of the record, not a header
// in front of it. Dropping it shifts every byte after it, and OpenVPN reads
// garbage from then on — which is exactly what the relay did before this test
// existed, because the prefix was read to learn the length and then never
// written back.
func TestSplitRelayKeepsTheLengthPrefix(t *testing.T) {
	server := newFakeTunnelBear(t)

	relay := NewSplitRelay([]string{server.host()}, server.port(), nil)
	port, err := relay.Start()
	if err != nil {
		t.Fatalf("relay did not start: %v", err)
	}
	defer func() { _ = relay.Close() }()

	client, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		t.Fatalf("could not reach the relay: %v", err)
	}
	defer func() { _ = client.Close() }()

	// Nothing after the record, so only the first phase runs.
	want := resetRecord([]byte("hello-opcode"))
	if _, err := client.Write(want); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if tcp, ok := client.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}

	select {
	case collected := <-server.got:
		if !bytes.Equal(collected, want) {
			t.Fatalf("the record was altered: sent %d bytes %v, received %d bytes %v",
				len(want), want, len(collected), collected)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the fake server never saw the connection close")
	}
}

// A relay that never reaches its upstream must fail loudly rather than accept a
// client and silently drop it.
func TestSplitRelayWithoutHostsRefusesToStart(t *testing.T) {
	relay := NewSplitRelay(nil, 7011, nil)
	if _, err := relay.Start(); err == nil {
		_ = relay.Close()
		t.Fatal("a relay with no addresses should not start")
	}
}

// An upstream that accepts and then closes at once must not leave the client
// hanging: the relay has to drop it so the core reports a failure instead of
// waiting out its own timeout.
func TestSplitRelayDropsTheClientWhenTheUpstreamResets(t *testing.T) {
	// A listener that accepts and closes at once — an address that answers but
	// has nothing behind it.
	dead, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not start the dead listener: %v", err)
	}
	defer func() { _ = dead.Close() }()
	go func() {
		for {
			conn, acceptErr := dead.Accept()
			if acceptErr != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	deadHost, deadPortStr, _ := net.SplitHostPort(dead.Addr().String())
	var deadPort int
	_, _ = fmt.Sscanf(deadPortStr, "%d", &deadPort)

	relay := NewSplitRelay([]string{deadHost}, deadPort, nil)
	if _, err := relay.Start(); err != nil {
		t.Fatalf("relay did not start: %v", err)
	}
	defer relay.Close()

	client, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", relayPort(t, relay)), 3*time.Second)
	if err != nil {
		t.Fatalf("could not reach the relay: %v", err)
	}
	defer client.Close()

	_ = client.SetReadDeadline(time.Now().Add(15 * time.Second))
	buf := make([]byte, 16)
	if _, err := client.Read(buf); err == nil {
		t.Error("the relay kept a client alive on an upstream that reset")
	}
}

// relayPort reads back the port a started relay is listening on. The relay does
// not expose it after Start, so the test keeps its own handle on the listener.
func relayPort(t *testing.T, r *SplitRelay) int {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ln == nil {
		t.Fatal("relay is not listening")
	}
	return r.ln.Addr().(*net.TCPAddr).Port
}

// Closing the relay must not leave the loopback port held, because the next
// connection is given a fresh one and a leaked listener would keep the old
// profile dialable.
func TestSplitRelayCloseReleasesTheListener(t *testing.T) {
	server := newFakeTunnelBear(t)

	relay := NewSplitRelay([]string{server.host()}, server.port(), nil)
	port, err := relay.Start()
	if err != nil {
		t.Fatalf("relay did not start: %v", err)
	}

	if err := relay.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Error("the relay still accepts connections after Close")
	}
}

// readExactly is what keeps the reset record from being torn apart when the
// prefix arrives in more than one read.
func TestReadExactlyHandlesShortReads(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer func() { _ = server.Close() }()

	go func() {
		// Deliberately dribbled out one byte at a time.
		for _, b := range []byte{0x00, 0x04, 'a', 'b', 'c', 'd'} {
			_, _ = server.Write([]byte{b})
			time.Sleep(time.Millisecond)
		}
	}()

	got, err := readExactly(client, 6)
	if err != nil {
		t.Fatalf("readExactly failed: %v", err)
	}
	if !bytes.Equal(got, []byte{0x00, 0x04, 'a', 'b', 'c', 'd'}) {
		t.Errorf("readExactly returned %v", got)
	}
}

func TestReadExactlyRejectsNonsense(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer func() { _ = server.Close() }()

	if _, err := readExactly(client, 0); err == nil {
		t.Error("a zero-length read should be rejected")
	}
}

// A client that disappears mid-handshake must not leave the relay goroutine
// blocked forever.
func TestSplitRelaySurvivesAClientThatVanishes(t *testing.T) {
	server := newFakeTunnelBear(t)

	relay := NewSplitRelay([]string{server.host()}, server.port(), nil)
	port, err := relay.Start()
	if err != nil {
		t.Fatalf("relay did not start: %v", err)
	}
	defer relay.Close()

	client, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		t.Fatalf("could not reach the relay: %v", err)
	}
	_, _ = client.Write([]byte{0x00, 0x02, 'a', 'b'})
	_ = client.Close()

	// A second client must still be served, which it cannot be if the first
	// one wedged the accept loop.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, dialErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
		if dialErr == nil {
			_, _ = conn.Write([]byte{0x00, 0x02, 'c', 'd'})
			_ = conn.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("the relay stopped accepting connections after a client vanished")
}

// TestSplitRelayDoesNotBlameTheHostWhenTheClientVanishes pins the host-rotation
// rule: only the upstream ending early counts as a dead host.
//
// The regression is subtle and cost a real session. `finish()` used to advance
// the pool whenever the byte count was low, whichever direction ended first. So
// when our own OpenVPN process died on a local fault (a WINTUN_IN_USE, which
// closes the client socket), the relay recorded the host as having "sent a reset
// before the handshake" — and the next attempt started one address further
// along. A purely local problem could therefore walk the entire pool, and the
// log blamed the operator's servers for it.
func TestSplitRelayDoesNotBlameTheHostWhenTheClientVanishes(t *testing.T) {
	healthy := newFakeTunnelBear(t)
	other := newFakeTunnelBear(t)

	relay := NewSplitRelay([]string{healthy.host(), other.host()}, healthy.port(), nil)
	port, err := relay.Start()
	if err != nil {
		t.Fatalf("relay did not start: %v", err)
	}
	defer relay.Close()

	client, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		t.Fatalf("could not reach the relay: %v", err)
	}
	// Connect, send nothing the server would answer, then vanish — exactly what
	// a dying OpenVPN process does.
	_ = client.Close()

	// Give the copy goroutines a moment to notice and call finish().
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		relay.mu.Lock()
		next := relay.next
		relay.mu.Unlock()
		if next != 0 {
			t.Fatalf("the client vanishing advanced the pool to index %d; only an upstream reset may do that", next)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// The converse: a host that accepts and immediately closes must still be
// skipped, or a dead address would be retried forever.
func TestSplitRelaySkipsAHostThatResetsImmediately(t *testing.T) {
	dead, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not start the dead listener: %v", err)
	}
	defer dead.Close()
	go func() {
		for {
			conn, acceptErr := dead.Accept()
			if acceptErr != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	alive := newFakeTunnelBear(t)
	deadHost, deadPortStr, _ := net.SplitHostPort(dead.Addr().String())
	var deadPort int
	_, _ = fmt.Sscanf(deadPortStr, "%d", &deadPort)

	relay := NewSplitRelay([]string{deadHost, alive.host()}, deadPort, nil)
	port, err := relay.Start()
	if err != nil {
		t.Fatalf("relay did not start: %v", err)
	}
	defer relay.Close()

	client, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		t.Fatalf("could not reach the relay: %v", err)
	}
	defer client.Close()
	// Deliberately send nothing: copyUpstream then blocks on its first read, so
	// the only direction that can end the session is the upstream's own reset.
	// Writing here would race the two directions against each other.

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		relay.mu.Lock()
		next := relay.next
		relay.mu.Unlock()
		if next == 1 {
			return // rotated past the dead address, as intended
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Error("a host that reset before the handshake must be skipped, but the pool never advanced")
}
