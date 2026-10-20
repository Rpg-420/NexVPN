package protocols

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// SplitRelay carries an OpenVPN connection to a server whose own transport is
// filtered.
//
// ## The problem it solves
//
// Measured from an Iranian line: TunnelBear's endpoints answer on UDP 443 and on
// TCP 7011, and on TCP the OpenVPN control channel gets its TLS handshake cut a
// few records in. The filter is not blocking the address — it is reading the
// shape of the stream. OpenVPN over TCP frames every packet with a two-byte
// length prefix and an opcode, so a DPI box that watches for a segment
// beginning with a plausible length followed by a plausible opcode recognises
// the handshake immediately and resets the flow.
//
// The counter is to stop producing segments that line up with that framing. The
// relay sits on loopback, so the local core talks plain OpenVPN to
// 127.0.0.1:port with nothing in the way, and the relay is what faces the
// network. In the client-to-server direction it writes:
//
//   - the first record whole — the hard-reset packet, read as
//     `2 + readUInt16BE(0)` bytes. Splitting this one would break the protocol
//     rather than hide it, and it is the one record a filter can identify from
//     its length alone anyway;
//   - the next 6 KB as 1 byte, then 3–29 byte pieces, 2–8 ms apart, so that no
//     segment boundary lands on a record boundary;
//   - everything after that as a plain copy at full speed, because by then the
//     handshake has completed and there is nothing left to fingerprint.
//
// The server-to-client direction is never touched: the filter is not looking at
// what the server sends, and delaying it would only add latency.
//
// ## Host rotation
//
// One country name resolves to a pool of roughly twenty addresses, and any
// given one may be dead. A host that closes the connection before sending
// `relayHandshakeBytes` is treated as dead and the next connection attempt
// starts from the following address, so a pool is walked rather than retried.
// The rotation only reacts to the *upstream* ending early: if the client side
// goes away first (our own OpenVPN process exiting, say) that says nothing about
// the host, and treating it as a reset would walk the pool on a local fault.
//
// ## Lifetime
//
// A relay exists exactly as long as the connection it carries. It is created in
// the app's connect path and closed on disconnect; the profile it serves points
// at a loopback port that is only meaningful while this process is listening.
type SplitRelay struct {
	hosts  []string
	port   int
	logger LoggerFunc

	mu     sync.Mutex
	next   int
	ln     net.Listener
	conns  map[net.Conn]struct{}
	closed bool
}

const (
	// relaySplitBytes is how much of the client direction is sent in small
	// pieces. Sized to cover the TLS handshake inside the control channel —
	// past it there is nothing recognisable to hide.
	relaySplitBytes = 6 * 1024

	// relayHandshakeBytes is how much the server must send back before the
	// connection counts as alive. A host that closes sooner sent a reset, not a
	// handshake, and should not be tried again this session.
	relayHandshakeBytes = 2048

	// relayDialTimeoutMulti is the per-host dial budget when there are other
	// addresses to fall back on, and relayDialTimeoutSingle when there are not
	// — in which case the one address is worth waiting longer for.
	relayDialTimeoutMulti  = 4 * time.Second
	relayDialTimeoutSingle = 8 * time.Second
)

// NewSplitRelay returns a relay that will dial one of hosts on port.
func NewSplitRelay(hosts []string, port int, logger LoggerFunc) *SplitRelay {
	return &SplitRelay{
		hosts:  hosts,
		port:   port,
		logger: logger,
		conns:  make(map[net.Conn]struct{}),
	}
}

func (r *SplitRelay) log(level, msg string) {
	if r.logger != nil {
		r.logger(level, msg, "openvpn-relay")
	}
}

// Start binds the loopback listener and returns the port it got.
//
// The port is deliberately ephemeral: it is handed straight to the profile
// that is about to be dialled, and nothing else ever needs to know it. A fixed
// port would collide with whatever else the machine has running.
func (r *SplitRelay) Start() (int, error) {
	if len(r.hosts) == 0 {
		return 0, fmt.Errorf("no server addresses to relay to")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("relay could not listen on loopback: %w", err)
	}

	r.mu.Lock()
	r.ln = ln
	r.mu.Unlock()

	port := ln.Addr().(*net.TCPAddr).Port
	r.log("info", fmt.Sprintf("Split relay listening on 127.0.0.1:%d → %d address(es), TCP %d",
		port, len(r.hosts), r.port))

	go r.acceptLoop(ln)
	return port, nil
}

func (r *SplitRelay) acceptLoop(ln net.Listener) {
	for {
		client, err := ln.Accept()
		if err != nil {
			r.mu.Lock()
			closed := r.closed
			r.mu.Unlock()
			if !closed {
				r.log("warning", fmt.Sprintf("Split relay accept failed: %v", err))
			}
			return
		}
		go r.serve(client)
	}
}

// Close stops the listener and drops every socket it is carrying.
func (r *SplitRelay) Close() error {
	r.mu.Lock()
	r.closed = true
	ln := r.ln
	conns := make([]net.Conn, 0, len(r.conns))
	for c := range r.conns {
		conns = append(conns, c)
	}
	r.conns = make(map[net.Conn]struct{})
	r.mu.Unlock()

	if ln != nil {
		_ = ln.Close()
	}
	for _, c := range conns {
		_ = c.Close()
	}
	return nil
}

func (r *SplitRelay) track(c net.Conn) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		_ = c.Close()
		return
	}
	r.conns[c] = struct{}{}
	r.mu.Unlock()
}

func (r *SplitRelay) untrack(c net.Conn) {
	r.mu.Lock()
	delete(r.conns, c)
	r.mu.Unlock()
}

// dial walks the pool from wherever the last successful dial left off.
func (r *SplitRelay) dial() (net.Conn, string, error) {
	r.mu.Lock()
	start := r.next
	closed := r.closed
	r.mu.Unlock()
	if closed {
		return nil, "", fmt.Errorf("relay is closed")
	}

	timeout := relayDialTimeoutMulti
	if len(r.hosts) == 1 {
		timeout = relayDialTimeoutSingle
	}

	var lastErr error
	for i := 0; i < len(r.hosts); i++ {
		at := (start + i) % len(r.hosts)
		host := r.hosts[at]

		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", r.port)), timeout)
		if err != nil {
			lastErr = err
			r.log("warning", fmt.Sprintf("Split relay could not reach %s:%d (%v)", host, r.port, err))
			continue
		}

		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetNoDelay(true)
		}
		r.mu.Lock()
		r.next = at
		r.mu.Unlock()
		return conn, host, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no addresses to try")
	}
	return nil, "", lastErr
}

// skip advances the pool past a host that answered with a reset.
func (r *SplitRelay) skip(host string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.hosts) < 2 {
		return
	}
	if r.hosts[r.next] != host {
		return
	}
	r.next = (r.next + 1) % len(r.hosts)
	r.log("info", fmt.Sprintf("Split relay: %s sent a reset before the handshake; next attempt starts at %s",
		host, r.hosts[r.next]))
}

func (r *SplitRelay) serve(client net.Conn) {
	r.track(client)
	defer r.untrack(client)
	defer client.Close()

	if tcp, ok := client.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}

	upstream, host, err := r.dial()
	if err != nil {
		r.log("error", fmt.Sprintf("Split relay gave up: %v", err))
		return
	}
	r.track(upstream)
	defer r.untrack(upstream)
	defer upstream.Close()

	r.log("info", fmt.Sprintf("Split relay connected to %s:%d", host, r.port))

	// downBytes is written by the copy goroutine below and read by finish,
	// which may run on either goroutine, so it has to be atomic.
	var downBytes atomic.Int64
	var once sync.Once
	// finish takes the side that ended the session. Only an *upstream* ending
	// with too little data means the host is dead.
	//
	// The old version blamed the host whenever the byte count was low, whichever
	// direction ended first — so a purely local failure (our own OpenVPN process
	// dying on a WINTUN_IN_USE, which tears the client socket down) looked
	// exactly like a server sending a reset, and the relay advanced past a host
	// that had been perfectly healthy. One local fault could walk the whole pool.
	finish := func(clientEnded bool) {
		once.Do(func() {
			if !clientEnded && downBytes.Load() < relayHandshakeBytes {
				r.skip(host)
			}
			_ = client.Close()
			_ = upstream.Close()
		})
	}

	// Server → client: untouched. The filter is reading what the client sends,
	// and delaying this direction would only add latency.
	go func() {
		n, _ := io.Copy(client, upstream)
		downBytes.Add(n)
		finish(false)
	}()

	r.copyUpstream(upstream, client)
	finish(true)
}

// copyUpstream is the client-to-server direction: the reset record whole, then
// the first relaySplitBytes in small irregular pieces, then a plain copy.
func (r *SplitRelay) copyUpstream(dst, src net.Conn) {
	// Phase 1 — the hard-reset record, read and written as one unit.
	//
	// OpenVPN over TCP frames every packet as a two-byte big-endian length
	// followed by that many bytes, so the record's own length says how much of
	// it must stay together. Chopping this one up would not hide it — the
	// length is the first thing a filter reads — it would just break the
	// protocol.
	head, err := readExactly(src, 2)
	if err != nil {
		return
	}
	// The record is `2 + readUInt16BE(0)` bytes *in total* — the prefix is part
	// of it. Reading the prefix separately and then reading that many more
	// bytes would consume two bytes too many and, worse, writing only the
	// second half would drop the prefix itself, shifting every packet after it.
	// The prefix is read first only to learn the length, and is written back as
	// part of the same buffer so the record still leaves in one call.
	total := 2 + int(binary.BigEndian.Uint16(head))
	record := make([]byte, 0, total)
	record = append(record, head...)
	// A zero-length record is not something OpenVPN sends, but reading zero
	// bytes is an error here and would drop the connection over a packet that
	// can simply be passed on.
	if total > 2 {
		rest, err := readExactly(src, total-2)
		if err != nil {
			return
		}
		record = append(record, rest...)
	}

	if _, werr := dst.Write(record); werr != nil {
		return
	}

	// Phase 2 — the handshake, in pieces that do not line up with anything.
	budget := relaySplitBytes
	firstPiece := true
	buf := make([]byte, 32*1024)

	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			at := 0
			for at < len(chunk) {
				if budget <= 0 {
					// The handshake is done; from here it is a plain relay.
					if _, werr := dst.Write(chunk[at:]); werr != nil {
						return
					}
					break
				}
				size := 3 + rand.IntN(27)
				if firstPiece {
					// One byte first: it cannot contain a length prefix, an
					// opcode, or anything else worth matching on.
					size = 1
					firstPiece = false
				}
				end := at + size
				if end > len(chunk) {
					end = len(chunk)
				}
				if _, werr := dst.Write(chunk[at:end]); werr != nil {
					return
				}
				budget -= end - at
				at = end
				if at < len(chunk) && budget > 0 {
					time.Sleep(time.Duration(2+rand.IntN(7)) * time.Millisecond)
				}
			}
		}
		if readErr != nil {
			return
		}
	}
}

// readExactly reads n bytes, tolerating short reads.
//
// A TCP read is free to return fewer bytes than were asked for, and the first
// record's two-byte length prefix is small enough that this is not theoretical.
// Returning on a short read would corrupt the stream rather than fail loudly.
func readExactly(src net.Conn, n int) ([]byte, error) {
	if n <= 0 {
		return nil, fmt.Errorf("invalid read length %d", n)
	}
	out := make([]byte, 0, n)
	buf := make([]byte, n)
	for len(out) < n {
		got, err := src.Read(buf[:n-len(out)])
		if got > 0 {
			out = append(out, buf[:got]...)
		}
		if err != nil {
			return out, err
		}
	}
	return out, nil
}
