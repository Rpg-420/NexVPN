//go:build windows

package sysproxy

import (
	"encoding/binary"
	"strings"
	"testing"
)

// TestBuildConnectionBlobEnabled verifies the WinINET "DefaultConnectionSettings"
// blob structure that Windows actually consumes. A malformed blob silently
// disables the system proxy, so the layout is pinned here byte-for-byte.
func TestBuildConnectionBlobEnabled(t *testing.T) {
	const server = "http=127.0.0.1:10809;https=127.0.0.1:10809;socks=127.0.0.1:10808"
	const bypass = defaultBypass

	blob := buildConnectionBlob(true, server, bypass)

	// Header: signature 0x46, version 2, then flags 0x03 (proxy enabled).
	if blob[0] != 0x46 {
		t.Errorf("signature byte = %#x, want 0x46", blob[0])
	}
	if got := binary.LittleEndian.Uint32(blob[4:8]); got != 2 {
		t.Errorf("version = %d, want 2", got)
	}
	if got := binary.LittleEndian.Uint32(blob[8:12]); got != 0x03 {
		t.Errorf("flags = %#x, want 0x03 (proxy enabled)", got)
	}

	// Expected total length: 12 header + (4+len(server)) + (4+len(bypass)) + 4 + 32.
	wantLen := 12 + 4 + len(server) + 4 + len(bypass) + 4 + 32
	if len(blob) != wantLen {
		t.Fatalf("blob length = %d, want %d", len(blob), wantLen)
	}

	// Embedded proxy server string must round-trip exactly.
	off := 12
	sLen := int(binary.LittleEndian.Uint32(blob[off : off+4]))
	off += 4
	if sLen != len(server) {
		t.Fatalf("proxy length prefix = %d, want %d", sLen, len(server))
	}
	if got := string(blob[off : off+sLen]); got != server {
		t.Errorf("proxy payload = %q, want %q", got, server)
	}
	off += sLen

	// Embedded bypass list must round-trip exactly.
	bLen := int(binary.LittleEndian.Uint32(blob[off : off+4]))
	off += 4
	if bLen != len(bypass) {
		t.Fatalf("bypass length prefix = %d, want %d", bLen, len(bypass))
	}
	if got := string(blob[off : off+bLen]); got != bypass {
		t.Errorf("bypass payload = %q, want %q", got, bypass)
	}
	off += bLen

	// Trailing zero length + 32 reserved bytes.
	if got := binary.LittleEndian.Uint32(blob[off : off+4]); got != 0 {
		t.Errorf("trailing length = %d, want 0", got)
	}
	off += 4
	if len(blob)-off != 32 {
		t.Errorf("reserved tail = %d bytes, want 32", len(blob)-off)
	}
}

// TestBuildConnectionBlobDisabled verifies that clearing the proxy produces a
// compact blob with the "disabled" flag and no server/bypass payloads.
func TestBuildConnectionBlobDisabled(t *testing.T) {
	blob := buildConnectionBlob(false, "http=127.0.0.1:10809", "whatever")

	if got := binary.LittleEndian.Uint32(blob[8:12]); got != 0x01 {
		t.Errorf("flags = %#x, want 0x01 (proxy disabled)", got)
	}
	// 12 header + 4 + 4 + 4 + 32 reserved; no payload bytes at all.
	if want := 12 + 4 + 4 + 4 + 32; len(blob) != want {
		t.Fatalf("disabled blob length = %d, want %d", len(blob), want)
	}
	if got := binary.LittleEndian.Uint32(blob[12:16]); got != 0 {
		t.Errorf("proxy length prefix = %d, want 0", got)
	}
	if got := binary.LittleEndian.Uint32(blob[16:20]); got != 0 {
		t.Errorf("bypass length prefix = %d, want 0", got)
	}
}

// TestDefaultBypassCoversLoopbackAndPrivateRanges ensures the bypass list keeps
// localhost and RFC1918 traffic off the tunnel, which is what prevents LAN and
// loopback breakage when the system proxy is on.
func TestDefaultBypassCoversLoopbackAndPrivateRanges(t *testing.T) {
	for _, needle := range []string{"localhost", "127.*", "10.*", "192.168.*", "172.16.*", "172.31.*", "<local>"} {
		if !strings.Contains(defaultBypass, needle) {
			t.Errorf("defaultBypass is missing %q", needle)
		}
	}
}

// TestIsSystemProxyEnabledReadOnly is a smoke test: reading the current proxy
// state must never panic, regardless of the machine's configuration.
func TestIsSystemProxyEnabledReadOnly(t *testing.T) {
	_ = IsSystemProxyEnabled()
}
