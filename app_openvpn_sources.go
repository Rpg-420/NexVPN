package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"nexvpn/internal/discovery"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// OpenVPN server catalogues.
//
// NexVPN's free-config aggregator only understood the proxy family (vless, vmess,
// trojan, shadowsocks, hysteria2, tuic, wireguard), so OpenVPN — the one protocol
// with a genuinely public relay pool — could not be browsed from inside the app.
// These bound methods add that surface. See internal/discovery/openvpn_sources.go
// for the provider registry and the reasoning about which sources are included.

var (
	ovpnCatalogueMu     sync.Mutex
	ovpnCatalogueCancel context.CancelFunc
)

// ListOpenVPNSources returns the built-in OpenVPN catalogues and what they are.
func (a *App) ListOpenVPNSources() []discovery.OpenVPNSourceInfo {
	return discovery.ListOpenVPNSources()
}

// FetchOpenVPNServers downloads a catalogue.
//
// mirrorURL is optional and overrides the source's own URL. It exists because on
// many Iranian connections the upstream list host is unreachable — the name
// resolves to a block page and a handshake naming it is reset — so the user needs
// a way to route the fetch through a relay or mirror of their own.
func (a *App) FetchOpenVPNServers(sourceID string, mirrorURL string) ([]discovery.OpenVPNServer, error) {
	if sourceID == "" {
		sourceID = "vpngate"
	}

	// The overall budget, and the reason it is minutes rather than seconds.
	//
	// The VPN Gate list is ~1.35 MB of chunked CSV and takes 8-20 s through a
	// local proxy and over 40 s on a direct international route. The fetch
	// retries once, so a budget has to cover two attempts plus the pause between
	// them; 45 seconds — the old value — could not cover even one slow attempt,
	// which is what made a working source look unreachable. internal/discovery
	// owns the per-phase timeouts and is the thing that detects a genuinely dead
	// host, so this only has to be generous enough not to cut a slow transfer
	// short.
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	servers, err := discovery.FetchOpenVPNServers(ctx, sourceID, mirrorURL)
	if err != nil {
		if a.engine != nil {
			a.engine.AddLog("error", fmt.Sprintf("OpenVPN catalogue %s failed: %v", sourceID, err), "discovery")
		}
		return nil, err
	}

	if a.engine != nil {
		a.engine.AddLog("info",
			fmt.Sprintf("OpenVPN catalogue %s: %d servers available", sourceID, len(servers)), "discovery")
	}
	return servers, nil
}

// TestOpenVPNServers probes the relays for reachability and reports progress.
//
// Starting a new test cancels the previous one, matching TestFreeConfigs: without
// that, clicking "test" twice leaves the first sweep running against a list the
// user has already replaced.
func (a *App) TestOpenVPNServers(servers []discovery.OpenVPNServer) []discovery.OpenVPNServer {
	ovpnCatalogueMu.Lock()
	if ovpnCatalogueCancel != nil {
		ovpnCatalogueCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	ovpnCatalogueCancel = cancel
	ovpnCatalogueMu.Unlock()

	results := discovery.TestOpenVPNServers(ctx, servers, 24, 2500*time.Millisecond,
		func(p discovery.OpenVPNServerTestProgress) {
			if a.appCtx() != nil {
				runtime.EventsEmit(a.appCtx(), "openvpn_sources:test_progress", p)
			}
		})

	if a.appCtx() != nil {
		runtime.EventsEmit(a.appCtx(), "openvpn_sources:test_complete", results)
	}

	return results
}

// ImportOpenVPNServers adds the selected catalogue rows as OpenVPN profiles.
//
// username/password are only needed for a catalogue whose relays authenticate
// against a personal account; they are empty for public pools such as VPN Gate.
// The .ovpn body itself is resolved from the fetch cache by server ID, so the
// caller never has to round-trip a 13 KB config through the UI.
func (a *App) ImportOpenVPNServers(servers []discovery.OpenVPNServer, group string, username string, password string) (int, error) {
	if a.store == nil {
		return 0, fmt.Errorf("store not initialized")
	}
	if len(servers) == 0 {
		return 0, fmt.Errorf("no servers selected")
	}
	if group == "" {
		group = "VPN Gate"
	}

	imported, stale := 0, 0
	for _, srv := range servers {
		profile, err := discovery.BuildOpenVPNProfile(srv, group, username, password)
		if err != nil {
			// A cache miss means the list was fetched before a restart. Count it
			// and report at the end rather than importing a broken profile.
			stale++
			continue
		}
		if _, err := a.store.AddProfile(profile); err != nil {
			continue
		}
		imported++
	}

	if imported == 0 {
		if stale > 0 {
			return 0, fmt.Errorf(
				"none of the %d selected servers could be imported — the server list is stale, refresh it and try again", stale)
		}
		return 0, fmt.Errorf("no servers could be imported")
	}

	if a.engine != nil {
		a.engine.AddLog("info",
			fmt.Sprintf("Imported %d OpenVPN profiles into %q", imported, group), "discovery")
	}
	return imported, nil
}
