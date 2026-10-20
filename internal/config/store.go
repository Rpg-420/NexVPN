package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"nexvpn/internal/diary"
	"nexvpn/internal/secret"
)

// sealedPasswordPrefix marks a profile password that has been through
// secret.Seal before being written to disk.
//
// The marker lives in the value rather than in a companion boolean field so that
// a config file is self-describing: a reader can tell an encrypted password from
// a legacy plaintext one without any extra state, and an older build that does
// not know about sealing still stores the value verbatim instead of corrupting
// it. It also keeps the Profile struct — and therefore the TypeScript mirror of
// it — unchanged.
const sealedPasswordPrefix = "nexvpn-sealed-v1:"

type Protocol string

const (
	ProtocolWireGuard Protocol = "wireguard"
	ProtocolXray      Protocol = "xray"
	ProtocolSingBox   Protocol = "singbox"
	ProtocolOpenVPN   Protocol = "openvpn"
	ProtocolHysteria2 Protocol = "hysteria2"
	ProtocolTUIC      Protocol = "tuic"
	ProtocolWARP      Protocol = "warp"
	ProtocolAether    Protocol = "aether"
	ProtocolPsiphon   Protocol = "psiphon"
)

type ProfileCategory string

const (
	CategoryV2Ray         ProfileCategory = "v2ray"
	CategoryWireGuardWarp ProfileCategory = "wireguard_warp"
	CategoryOpenVPN       ProfileCategory = "openvpn"
	CategoryAether        ProfileCategory = "aether"
	CategoryPsiphon       ProfileCategory = "psiphon"
)

type Profile struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Protocol         Protocol        `json:"protocol"`
	Category         ProfileCategory `json:"category"` // v2ray, wireguard_warp, openvpn
	ConfigContent    string          `json:"config_content"`
	RawURI           string          `json:"raw_uri,omitempty"`
	Server           string          `json:"server,omitempty"`
	Port             int             `json:"port,omitempty"`
	Group            string          `json:"group,omitempty"`
	Country          string          `json:"country,omitempty"`
	CountryCode      string          `json:"country_code,omitempty"`
	LatencyMs        int             `json:"latency_ms,omitempty"`
	RealDelayMs      int             `json:"real_delay_ms,omitempty"`
	SubscriptionURL  string          `json:"subscription_url,omitempty"`
	SubscriptionName string          `json:"subscription_name,omitempty"`
	Description      string          `json:"description,omitempty"`
	Tags             []string        `json:"tags,omitempty"`

	// Credentials for OpenVPN / Custom
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	// Cloudflare WARP specific
	WarpEndpoint string `json:"warp_endpoint,omitempty"`
	WarpCleanIP  string `json:"warp_clean_ip,omitempty"`
	WarpReserved string `json:"warp_reserved,omitempty"`

	// Favorite pins the profile to the top of its list and into the tray
	// submenu. It is a plain bool rather than a rank so that pinning stays a
	// one-tap action; ordering among pinned profiles is by name.
	Favorite bool `json:"favorite,omitempty"`

	// SessionDataCapMB is a per-connection download+upload ceiling in
	// megabytes. 0 means "no cap".
	//
	// It is a cap, not a rate limit: the cores expose no portable way to shape
	// bandwidth, so enforcing a rate would mean lying about it. A byte ceiling
	// is measurable from the counters the engine already keeps, and it is what
	// a metered-connection user actually needs.
	SessionDataCapMB int `json:"session_data_cap_mb,omitempty"`

	// LastFailure records why this server was last given up on and when. The
	// engine reports it when a connect attempt proves the server unreachable, and
	// the cooldown derived from it keeps auto-failover and "fastest server" from
	// walking straight back onto a host that refused a connection seconds ago.
	//
	// It is deliberately not a permanent ban: subscription servers flap, and a
	// node that was down at 09:43 may be the best one at 10:00.
	ConsecutiveFailures int       `json:"consecutive_failures,omitempty"`
	LastFailureAt       time.Time `json:"last_failure_at,omitempty"`
	LastFailureReason   string    `json:"last_failure_reason,omitempty"`

	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	LastUsed   time.Time `json:"last_used,omitempty"`
	UsageCount int       `json:"usage_count"`
}

type Settings struct {
	AutoConnect       bool   `json:"auto_connect"`
	StartMinimized    bool   `json:"start_minimized"`
	StartOnBoot       bool   `json:"start_on_boot"`
	KillSwitchEnabled bool   `json:"kill_switch_enabled"`
	DNSServer         string `json:"dns_server"`
	DNSFallback       string `json:"dns_fallback"`
	Language          string `json:"language"`
	Theme             string `json:"theme"`
	RoutingMode       string `json:"routing_mode"` // "global", "bypass_iran", "direct_iran_block_ads"
	ProxyMode         string `json:"proxy_mode"`   // "tun", "system_proxy"
	// TunSystemProxy decides whether the machine-wide Windows proxy is applied on
	// top of an already-working TUN adapter.
	//
	// "dual" keeps both paths active and is the default, because that is the
	// behaviour every existing install has been running with: an app that ignores
	// the virtual adapter still gets proxied. "tun_only" sets up the adapter alone.
	// The reason to choose it is that a system HTTP proxy is not a harmless
	// addition — applications that route non-HTTP traffic through the system proxy
	// break on it. Telegram is the standard case: it sends DNS queries for its own
	// IPv6-only endpoints through the proxy, an HTTP inbound cannot carry them, and
	// every one of those queries arrives at the core as
	// `parse "http://2001:b28:f23f:f005::a:443": invalid port`.
	TunSystemProxy              string   `json:"tun_system_proxy"` // "dual", "tun_only"
	RealDelayUrl                string   `json:"real_delay_url"`
	LastProfileID               string   `json:"last_profile_id,omitempty"`
	LogLevel                    string   `json:"log_level"`
	MaxLogEntries               int      `json:"max_log_entries"`
	AutoReconnect               bool     `json:"auto_reconnect"`
	ReconnectAttempts           int      `json:"reconnect_attempts"`
	WarpCleanIP                 string   `json:"warp_clean_ip,omitempty"`
	WarpPort                    int      `json:"warp_port,omitempty"`
	WarpLicenseKey              string   `json:"warp_license_key,omitempty"`
	WarpAccountType             string   `json:"warp_account_type,omitempty"` // "free", "plus"
	FragmentEnabled             bool     `json:"fragment_enabled"`
	FragmentSize                string   `json:"fragment_size"`
	FragmentSleep               string   `json:"fragment_sleep"`
	AppRoutingMode              string   `json:"app_routing_mode"` // "all", "proxy_selected", "bypass_selected"
	SelectedApps                []string `json:"selected_apps"`
	AutoFailover                bool     `json:"auto_failover"`
	AutoFailoverEnabled         bool     `json:"auto_failover_enabled"`
	SubscriptionAutoUpdateHours int      `json:"subscription_auto_update_hours"`
	MultiHopWarp                bool     `json:"multi_hop_warp"`
	IPStrategy                  string   `json:"ip_strategy"` // "prefer_ipv4", "prefer_ipv6", "ipv4_only", "ipv6_only"
	BlockAds                    bool     `json:"block_ads"`
	BypassLAN                   bool     `json:"bypass_lan"`
	CustomDirectDomains         []string `json:"custom_direct_domains"`
	CustomProxyDomains          []string `json:"custom_proxy_domains"`
	CustomBlockDomains          []string `json:"custom_block_domains"`
	AmneziaJc                   int      `json:"amnezia_jc"`
	AmneziaJmin                 int      `json:"amnezia_jmin"`
	AmneziaJmax                 int      `json:"amnezia_jmax"`
	AmneziaS1                   int      `json:"amnezia_s1"`
	AmneziaS2                   int      `json:"amnezia_s2"`
	AmneziaH1                   int      `json:"amnezia_h1"`
	AmneziaH2                   int      `json:"amnezia_h2"`
	AmneziaH3                   int      `json:"amnezia_h3"`
	AmneziaH4                   int      `json:"amnezia_h4"`
	ShowMiniWidget              bool     `json:"show_mini_widget"`
	V2RayCore                   string   `json:"v2ray_core"` // "auto", "xray", "singbox"
	AllowLan                    bool     `json:"allow_lan"`
	GameModeEnabled             bool     `json:"game_mode_enabled"`
	SoundEffectsEnabled         bool     `json:"sound_effects_enabled"`
	MonthlyQuotaGB              int      `json:"monthly_quota_gb"`
	QuotaAlertPercentage        int      `json:"quota_alert_percentage"`
	StealthModeEnabled          bool     `json:"stealth_mode_enabled"`
	PrivacyShieldEnabled        bool     `json:"privacy_shield_enabled"` // Auto-redacts sensitive tokens/UUIDs in logs
	// RemoteControllerLANOnly refuses non-RFC1918 peers at the LAN controller
	// listener. Defaults to true; disabling it exposes the PIN prompt to any
	// routable address that can reach the port.
	RemoteControllerLANOnly bool `json:"remote_controller_lan_only"`
	// RemoteControllerTLS serves the LAN controller over HTTPS with an
	// ephemeral self-signed certificate so the pairing PIN is not sent in the
	// clear. Defaults to true.
	RemoteControllerTLS bool `json:"remote_controller_tls"`
	// AllowUnverifiedCoreDownloads permits installing a downloaded proxy core
	// when no trusted SHA-256 digest is available. Off by default: it is the
	// only way a compromised mirror could substitute a binary.
	AllowUnverifiedCoreDownloads bool `json:"allow_unverified_core_downloads"`

	// WebviewDisableGpu turns off WebView2's GPU-composited rendering.
	//
	// The failure this exists for is not a Go failure: on some driver
	// combinations the window paints black (or nothing at all) while the process
	// is perfectly healthy and connected. Every setting inside that window is
	// then unreachable, so the lever is persisted from the tray menu as well, and
	// read exactly once — before the webview exists. Restart to apply.
	WebviewDisableGpu bool `json:"webview_disable_gpu"`

	// WebviewDisableCodeIntegrity keeps WebView2's renderer code-integrity check
	// off. Endpoint-security products inject unsigned DLLs into the renderer, and
	// WebView2 answers by terminating it — a window that opens and closes itself.
	// Off by default: it relaxes a real sandbox guarantee, so it is a lever for a
	// machine that is demonstrably broken, not a setting to ship enabled.
	WebviewDisableCodeIntegrity bool     `json:"webview_disable_code_integrity"`
	NotificationsEnabled        bool     `json:"notifications_enabled"` // Native Windows Toast notifications
	CustomDoHUrl                string   `json:"custom_doh_url,omitempty"`
	SmartRoutingEnabled         bool     `json:"smart_routing_enabled"`
	AIRoutingMode               string   `json:"ai_routing_mode"`        // "proxy", "direct", "us_clean"
	StreamingRoutingMode        string   `json:"streaming_routing_mode"` // "proxy", "direct", "fastest"
	DomesticRoutingMode         string   `json:"domestic_routing_mode"`  // "direct", "proxy"
	AutoMTUEnabled              bool     `json:"auto_mtu_enabled"`
	CustomMTU                   int      `json:"custom_mtu"`
	QuotaWebhookURL             string   `json:"quota_webhook_url,omitempty"`
	CustomSNIPool               []string `json:"custom_sni_pool,omitempty"`

	// NetworkRules drive automatic connect/disconnect from the joined Wi-Fi
	// network's SSID. Empty means "no SSID automation".
	NetworkRules []NetworkRule `json:"network_rules,omitempty"`

	// RoutingRules are user overrides layered on top of RoutingMode. They are
	// emitted after the built-in rules so a manual entry always wins.
	RoutingRules []RoutingRule `json:"routing_rules,omitempty"`

	// ScheduleRules connect or disconnect at a wall-clock time on selected
	// weekdays.
	ScheduleRules []ScheduleRule `json:"schedule_rules,omitempty"`

	// AppUpdateCheckEnabled lets the app look for a newer release. It is
	// opt-out rather than opt-in because the check is a single unauthenticated
	// request that carries no identifiers, but it stays a setting so a fully
	// offline machine can silence it.
	AppUpdateCheckEnabled bool `json:"app_update_check_enabled"`

	// TunInterfaceName is the Wintun adapter name the next connection should ask
	// the core to use. Runtime-only — never persisted — because it is decided per
	// connection.
	//
	// It exists because a *ghost* adapter can permanently occupy the default
	// name: the Wintun driver still answers to it, so `WintunCreateAdapter`
	// reports the name as taken and `WintunOpenAdapter` hands back a handle to
	// nothing, while Windows itself cannot enumerate the adapter at all. A core
	// told to use that name starts, is seen as "up" by the readiness probe, and
	// dies a moment later. The engine detects the ghost and passes a different
	// name here. Empty means DefaultTunInterfaceName.
	TunInterfaceName string `json:"-"`
}

// DefaultTunInterfaceName is the Wintun adapter name a TUN inbound asks for when
// nothing overrides it. It lives here because both the config generator and the
// engine's adapter bookkeeping (MTU, cleanup, logging) have to agree on it.
const DefaultTunInterfaceName = "NexVPN-TUN"

// AdapterNamePrefix is the name prefix every network adapter this application
// creates carries (the TUN adapter, its ghost-avoidance variants, the dedicated
// OpenVPN adapter).
//
// It is exported because a consumer outside the adapter bookkeeping has to tell
// "the user's network changed" apart from "our own adapter appeared or vanished":
// the network watcher would otherwise treat every connect and every disconnect as
// a network change and re-verify the tunnel in response to itself.
const AdapterNamePrefix = "NexVPN"

// IsNexVPNAdapterName reports whether an interface belongs to this application.
func IsNexVPNAdapterName(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), AdapterNamePrefix)
}

// EffectiveTunInterfaceName returns the adapter name to use for a connection:
// the per-connection override when the engine set one, otherwise the default.
func (s Settings) EffectiveTunInterfaceName() string {
	if n := strings.TrimSpace(s.TunInterfaceName); n != "" {
		return n
	}
	return DefaultTunInterfaceName
}

// IsZero reports whether the receiver is the zero Settings value, i.e. nothing
// has ever been persisted for this store.
//
// Settings embeds []string fields (SelectedApps, CustomSNIPool, ...), which
// makes the struct non-comparable: a plain `s == Settings{}` is a compile error
// ("struct containing []string cannot be compared"). The comparison therefore
// has to go through reflect.DeepEqual.
//
// Note the asymmetry that a non-nil but empty slice ([]string{}) is *not*
// DeepEqual to a nil slice, so such a Settings reports IsZero() == false. That
// is harmless for the only caller: preserveSettingsFields is itself a no-op
// whenever the stored values are zero.
func (s Settings) IsZero() bool {
	return reflect.DeepEqual(s, Settings{})
}

type ConnectionSession struct {
	ID             string    `json:"id"`
	ProfileID      string    `json:"profile_id"`
	ProfileName    string    `json:"profile_name"`
	Protocol       Protocol  `json:"protocol"`
	Server         string    `json:"server"`
	ConnectedAt    time.Time `json:"connected_at"`
	DisconnectedAt time.Time `json:"disconnected_at"`
	DurationSec    int64     `json:"duration_sec"`
	BytesReceived  int64     `json:"bytes_received"`
	BytesSent      int64     `json:"bytes_sent"`
	QualityScore   int       `json:"quality_score"`
	Reason         string    `json:"reason"` // "manual", "error", "failover", "reconnect"
}

type FullBackupBundle struct {
	Version      string    `json:"version"`
	ExportedAt   time.Time `json:"exported_at"`
	Profiles     []Profile `json:"profiles"`
	Settings     Settings  `json:"settings"`
	ProfileCount int       `json:"profile_count"`
}

// CurrentSchemaVersion is the on-disk layout revision of StoreData.
//
// Bump it whenever a field is added, renamed, or removed, and add a matching
// case to migrateSchema. The version lets Load() tell a genuinely-new field
// from a stale file, and lets a future reader refuse a file that is too new to
// interpret safely instead of silently discarding fields it does not know.
const CurrentSchemaVersion = 2

type StoreData struct {
	// SchemaVersion is written on every save. Absent (0) means the file
	// predates versioning, which is treated as version 1.
	SchemaVersion int       `json:"schema_version"`
	Profiles      []Profile `json:"profiles"`
	Settings      Settings  `json:"settings"`
}

type Store struct {
	mu          sync.RWMutex
	dataDir     string
	dataFile    string
	historyFile string
	data        StoreData
	idIndex     map[string]int

	history   []ConnectionSession
	historyMu sync.RWMutex

	// subscriptionChanges records what each subscription refresh changed. It is
	// in-memory only: it describes the running session's refreshes, and
	// persisting it would mean writing the file on every background update just
	// to remember a notice the user has already seen.
	subscriptionChanges []SubscriptionChange
}

// DefaultDataDir returns the directory NexVPN keeps its state in.
//
// It is a plain function rather than a Store method so that callers which only
// need the path — the startup diary, which must be initialised before anything
// else can fail — do not have to construct a whole Store first.
func DefaultDataDir() string {
	home, _ := os.UserConfigDir()
	if home == "" {
		home = os.Getenv("APPDATA")
	}
	return filepath.Join(home, "NexVPN")
}

func NewStore() *Store {
	dir := DefaultDataDir()
	return &Store{
		dataDir:     dir,
		dataFile:    filepath.Join(dir, "config.json"),
		historyFile: filepath.Join(dir, "history.json"),
		idIndex:     make(map[string]int),
		data: StoreData{
			Profiles: []Profile{},
			Settings: Settings{
				AutoConnect:                  false,
				StartMinimized:               false,
				StartOnBoot:                  false,
				KillSwitchEnabled:            false,
				PrivacyShieldEnabled:         true,
				RemoteControllerLANOnly:      true,
				RemoteControllerTLS:          true,
				AllowUnverifiedCoreDownloads: false,
				NotificationsEnabled:         true,
				AppUpdateCheckEnabled:        true,
				DNSServer:                    DefaultPrimaryResolver,
				DNSFallback:                  DefaultSecondaryResolver,
				Language:                     "fa",
				Theme:                        "dark",
				RoutingMode:                  "bypass_iran",
				ProxyMode:                    "tun", // Default to high-performance TUN mode
				RealDelayUrl:                 "http://cp.cloudflare.com/generate_204",
				LogLevel:                     "info",
				MaxLogEntries:                1000,
				AutoReconnect:                true,
				ReconnectAttempts:            3,
				WarpCleanIP:                  "162.159.192.1",
				WarpPort:                     2408,
				WarpAccountType:              "free",
				FragmentEnabled:              true,
				FragmentSize:                 "100-200",
				FragmentSleep:                "10-20",
				AppRoutingMode:               "all",
				SelectedApps:                 []string{"telegram.exe", "discord.exe", "chrome.exe", "spotify.exe"},
				AutoFailover:                 true,
				SubscriptionAutoUpdateHours:  12,
				MultiHopWarp:                 false,
				IPStrategy:                   "prefer_ipv4",
				BlockAds:                     false,
				BypassLAN:                    true,
				CustomDirectDomains:          []string{},
				CustomProxyDomains:           []string{},
				CustomBlockDomains:           []string{},
				AmneziaJc:                    4,
				AmneziaJmin:                  40,
				AmneziaJmax:                  70,
				AmneziaS1:                    15,
				AmneziaS2:                    20,
				AmneziaH1:                    1,
				AmneziaH2:                    2,
				AmneziaH3:                    3,
				AmneziaH4:                    4,
				ShowMiniWidget:               false,
			},
		},
	}
}

func (s *Store) GetDataDir() string {
	return s.dataDir
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		return err
	}

	if _, err := os.Stat(s.dataFile); os.IsNotExist(err) {
		return s.saveUnlocked()
	}

	content, err := os.ReadFile(s.dataFile)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(content, &s.data); err != nil {
		return err
	}

	// Refuse a file written by a newer build: continuing would silently drop
	// whatever fields this build does not know about on the next save.
	if s.data.SchemaVersion > CurrentSchemaVersion {
		return fmt.Errorf(
			"config file %s uses schema version %d but this build understands at most %d; "+
				"upgrade NexVPN or restore a backup rather than opening it with an older version",
			s.dataFile, s.data.SchemaVersion, CurrentSchemaVersion)
	}
	// Migrate from any older layout up to the current one. A zero value means
	// the file predates versioning entirely.
	migrated, err := migrateSchema(&s.data, s.data.SchemaVersion)
	if err != nil {
		return fmt.Errorf("config migration failed: %w", err)
	}

	if s.historyFile == "" {
		s.historyFile = filepath.Join(s.dataDir, "history.json")
	}
	if data, err := os.ReadFile(s.historyFile); err == nil {
		_ = json.Unmarshal(data, &s.history)
	} else {
		s.history = []ConnectionSession{}
	}

	// Migrate categories for legacy profiles
	for i := range s.data.Profiles {
		p := &s.data.Profiles[i]
		if p.Category == "" {
			p.Category = InferCategory(p.Protocol)
		}
	}

	// Decrypt sealed profile passwords, and note whether the file on disk is
	// still in the legacy layout with passwords in clear.
	legacyPasswords := openProfilePasswords(s.data.Profiles)
	s.rebuildIndexLocked()

	// Persist immediately when a migration changed the on-disk layout, so the
	// upgraded file is the one a crash would recover, not the pre-migration one.
	// The same applies to a file that still holds plaintext passwords: rewriting
	// it now is what actually gets them encrypted, and waiting for the next
	// unrelated save would leave them exposed for as long as the app is open.
	if migrated || legacyPasswords {
		return s.saveUnlocked()
	}
	return nil
}

// migrateSchema upgrades data in place from fromVersion to CurrentSchemaVersion.
// It reports whether anything changed, so Load can persist the result.
func migrateSchema(data *StoreData, fromVersion int) (bool, error) {
	if fromVersion >= CurrentSchemaVersion {
		if data.SchemaVersion != CurrentSchemaVersion {
			data.SchemaVersion = CurrentSchemaVersion
			return true, nil
		}
		return false, nil
	}

	// Version 0 -> 1: the file predates schema_version itself. Nothing in the
	// stored shape changed, but the security controls must be back-fitted as
	// enabled so an upgrade cannot leave the LAN controller and download path
	// on the old, laxer defaults.
	if fromVersion < 1 {
		if !data.Settings.RemoteControllerLANOnly {
			data.Settings.RemoteControllerLANOnly = true
		}
		if !data.Settings.RemoteControllerTLS {
			data.Settings.RemoteControllerTLS = true
		}
		// Privacy Shield is a safety control; make sure an older file has it on.
		if !data.Settings.PrivacyShieldEnabled {
			data.Settings.PrivacyShieldEnabled = true
		}
	}

	// Version 1 -> 2: user-defined routing rules became a first-class list.
	//
	// Before this, three domain-only lists existed (CustomDirectDomains,
	// CustomProxyDomains, CustomBlockDomains) that the parser honoured but no
	// interface ever wrote to. They are folded into RoutingRules here and then
	// cleared, so there is exactly one place a routing override can live and
	// the new editor shows every rule the core will actually apply.
	if fromVersion < 2 {
		settings := &data.Settings
		appendLegacy := func(values []string, action string) {
			for _, value := range values {
				value = strings.TrimSpace(value)
				if value == "" {
					continue
				}
				settings.RoutingRules = append(settings.RoutingRules, RoutingRule{
					Kind:    RoutingKindDomain,
					Value:   value,
					Action:  action,
					Enabled: true,
				})
			}
		}
		appendLegacy(settings.CustomBlockDomains, RoutingActionBlock)
		appendLegacy(settings.CustomDirectDomains, RoutingActionDirect)
		appendLegacy(settings.CustomProxyDomains, RoutingActionProxy)
		settings.CustomBlockDomains = nil
		settings.CustomDirectDomains = nil
		settings.CustomProxyDomains = nil
	}

	// Normalise the rule lists on every load, not only during a version bump:
	// these arrive from a JSON blob the frontend round-trips, and a hand-edited
	// config file is a supported way to use the app.
	data.Settings.RoutingRules = NormalizeRoutingRules(data.Settings.RoutingRules)
	data.Settings.NetworkRules = NormalizeNetworkRules(data.Settings.NetworkRules)
	data.Settings.ScheduleRules = NormalizeScheduleRules(data.Settings.ScheduleRules)

	data.SchemaVersion = CurrentSchemaVersion
	return true, nil
}

func (s *Store) rebuildIndexLocked() {
	if s.idIndex == nil {
		s.idIndex = make(map[string]int, len(s.data.Profiles))
	} else {
		for k := range s.idIndex {
			delete(s.idIndex, k)
		}
	}
	for i, p := range s.data.Profiles {
		s.idIndex[p.ID] = i
	}
}

func InferCategory(proto Protocol) ProfileCategory {
	switch proto {
	case ProtocolWireGuard, ProtocolWARP:
		return CategoryWireGuardWarp
	case ProtocolOpenVPN:
		return CategoryOpenVPN
	default:
		return CategoryV2Ray
	}
}

// EffectiveCategory returns the category a profile should be treated as.
// Legacy profiles (and ones imported before categories existed) have an empty
// Category field, so callers must fall back to inferring it from the protocol
// instead of comparing the raw field — otherwise those profiles silently drop
// out of category filters such as the bulk latency test.
func (p Profile) EffectiveCategory() ProfileCategory {
	if p.Category != "" {
		return p.Category
	}
	return InferCategory(p.Protocol)
}

// sealProfilePasswords returns a copy of profiles with every plaintext password
// replaced by its sealed form.
//
// This exists because Profile.Password used to be written to config.json in
// clear. The in-memory copy deliberately keeps plaintext: the connect path, the
// OpenVPN adapter and the profile edit form all read the field directly, and
// unsealing on every use would spread the DPAPI call across the whole codebase.
// Only the bytes that reach the disk are transformed.
//
// A password that cannot be sealed is dropped rather than written in clear. That
// is recoverable — the user re-enters it — whereas leaking it into a file that
// gets backed up, synced or attached to a bug report is not.
func sealProfilePasswords(profiles []Profile) []Profile {
	if len(profiles) == 0 {
		return profiles
	}
	out := make([]Profile, len(profiles))
	copy(out, profiles)

	for i := range out {
		pw := out[i].Password
		if pw == "" || strings.HasPrefix(pw, sealedPasswordPrefix) {
			continue
		}
		sealed, err := secret.Seal(pw)
		if err != nil {
			diary.LogErr("config", "password-seal-failed", err)
			out[i].Password = ""
			continue
		}
		out[i].Password = sealedPasswordPrefix + sealed
	}
	return out
}

// openProfilePasswords decrypts sealed passwords in place, and reports whether
// any legacy plaintext password was found — which means the file on disk is
// still in the old, unencrypted layout and should be rewritten immediately.
//
// A sealed value that cannot be opened (a config copied to another machine, or
// to another Windows account) is cleared instead of being kept: the ciphertext
// is useless to the user either way, and leaving it in place makes the UI show a
// password that looks real but never works.
func openProfilePasswords(profiles []Profile) (legacyPlaintext bool) {
	for i := range profiles {
		pw := profiles[i].Password
		if pw == "" {
			continue
		}
		if !strings.HasPrefix(pw, sealedPasswordPrefix) {
			legacyPlaintext = true
			continue
		}
		plain, err := secret.Open(strings.TrimPrefix(pw, sealedPasswordPrefix))
		if err != nil {
			diary.LogErr("config", "password-open-failed", err)
			profiles[i].Password = ""
			continue
		}
		profiles[i].Password = plain
	}
	return legacyPlaintext
}

func (s *Store) saveUnlocked() error {
	s.rebuildIndexLocked()
	// Always stamp the current schema revision, so a file written by this build
	// is self-describing.
	s.data.SchemaVersion = CurrentSchemaVersion
	if err := os.MkdirAll(s.dataDir, 0700); err != nil {
		return err
	}
	// Marshal a copy whose passwords are sealed; s.data itself stays plaintext.
	persist := s.data
	persist.Profiles = sealProfilePasswords(s.data.Profiles)

	bytes, err := json.MarshalIndent(persist, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write via tmp + rename to prevent corruption on crash/power loss
	tmpFile := s.dataFile + ".tmp"
	if err := os.WriteFile(tmpFile, bytes, 0600); err != nil {
		return err
	}
	// Keep one pre-write snapshot before replacing the live file. If a future
	// release writes a root shape that turns out to be wrong, the previous good
	// file is still on disk instead of being lost to the rename.
	if prev, err := os.ReadFile(s.dataFile); err == nil && len(prev) > 0 {
		bakFile := s.dataFile + ".bak"
		// Only rotate the snapshot when the live file actually has content; a
		// failed backup must never block the real save.
		_ = os.WriteFile(bakFile, prev, 0600)
	}
	if err := os.Rename(tmpFile, s.dataFile); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}
	return nil
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveUnlocked()
}

// PreviousConfigSnapshotPath returns the path of the pre-write snapshot, or an
// empty string when no snapshot has been taken yet.
func (s *Store) PreviousConfigSnapshotPath() string {
	return s.dataFile + ".bak"
}

// RestorePreviousConfig replaces the live configuration with the snapshot taken
// before the most recent write.
//
// Every save already keeps one generation of history (see saveUnlocked), but
// until now nothing could reach it: recovering meant finding the file by hand.
// This is the missing half of that feature.
//
// The restore is itself reversible — the configuration being replaced becomes
// the new snapshot — because a rollback that cannot be undone is a trap for
// someone who rolls back to the wrong generation.
func (s *Store) RestorePreviousConfig() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bakFile := s.dataFile + ".bak"
	previous, err := os.ReadFile(bakFile)
	if err != nil {
		return fmt.Errorf("there is no previous configuration snapshot to restore yet: %w", err)
	}
	if len(previous) == 0 {
		return fmt.Errorf("the previous configuration snapshot is empty")
	}

	var restored StoreData
	if err := json.Unmarshal(previous, &restored); err != nil {
		return fmt.Errorf("the previous configuration snapshot is not readable: %w", err)
	}
	if restored.SchemaVersion > CurrentSchemaVersion {
		return fmt.Errorf(
			"the previous snapshot uses schema version %d but this build understands at most %d",
			restored.SchemaVersion, CurrentSchemaVersion)
	}

	// Validate the snapshot *before* touching anything. A snapshot that cannot
	// be migrated must leave the working configuration alone: trading a usable
	// config for a broken one is a strictly worse outcome than refusing.
	if _, err := migrateSchema(&restored, restored.SchemaVersion); err != nil {
		return fmt.Errorf("the previous configuration snapshot cannot be used: %w", err)
	}

	// Make the swap reversible before it happens.
	if current, err := os.ReadFile(s.dataFile); err == nil && len(current) > 0 {
		_ = os.WriteFile(bakFile, current, 0600)
	}

	s.data = restored
	// The snapshot on disk holds sealed passwords; the in-memory copy must hold
	// plaintext, exactly as Load produces.
	openProfilePasswords(s.data.Profiles)
	s.rebuildIndexLocked()

	return s.saveUnlocked()
}

func (s *Store) GetProfiles() []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]Profile, len(s.data.Profiles))
	copy(res, s.data.Profiles)
	return res
}

func (s *Store) GetProfilesByCategory(category ProfileCategory) []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]Profile, 0, len(s.data.Profiles))
	for _, p := range s.data.Profiles {
		cat := p.Category
		if cat == "" {
			cat = InferCategory(p.Protocol)
		}
		if category == "" || cat == category {
			res = append(res, p)
		}
	}
	return res
}

func (s *Store) GetProfile(id string) (Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if idx, exists := s.idIndex[id]; exists && idx >= 0 && idx < len(s.data.Profiles) {
		if s.data.Profiles[idx].ID == id {
			return s.data.Profiles[idx], nil
		}
	}
	for _, p := range s.data.Profiles {
		if p.ID == id {
			return p, nil
		}
	}
	return Profile{}, fmt.Errorf("profile %s not found", id)
}

func (s *Store) AddProfile(p Profile) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.Category == "" {
		p.Category = InferCategory(p.Protocol)
	}
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	s.data.Profiles = append(s.data.Profiles, p)
	return p.ID, s.saveUnlocked()
}

// AddProfileBatch adds multiple profiles in a single atomic file write (much faster than N AddProfile calls).
// Returns the IDs of all added profiles in the same order.
func (s *Store) AddProfileBatch(profiles []Profile) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	ids := make([]string, len(profiles))

	for i := range profiles {
		p := &profiles[i]
		if p.ID == "" {
			p.ID = uuid.New().String()
		} else {
			for {
				duplicate := false
				for _, existing := range s.data.Profiles {
					if existing.ID == p.ID {
						duplicate = true
						break
					}
				}
				if !duplicate {
					break
				}
				p.ID = uuid.New().String()
			}
		}
		if p.Category == "" {
			p.Category = InferCategory(p.Protocol)
		}
		if p.CreatedAt.IsZero() {
			p.CreatedAt = now
		}
		p.UpdatedAt = now
		ids[i] = p.ID
		s.data.Profiles = append(s.data.Profiles, *p)
	}

	// Single file write for all profiles
	return ids, s.saveUnlocked()
}

// ReplaceSubscriptionProfiles replaces all nodes belonging to a subscriptionURL to prevent duplicates
func (s *Store) ReplaceSubscriptionProfiles(subURL string, newProfiles []Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Keep profiles not from this subscription
	var kept []Profile
	var replaced []Profile
	for _, p := range s.data.Profiles {
		if p.SubscriptionURL != subURL || subURL == "" {
			kept = append(kept, p)
		} else {
			replaced = append(replaced, p)
		}
	}

	// Summarise the refresh before the old rows are discarded. A subscription
	// that silently loses a server the user was relying on is a real problem,
	// and it is invisible unless the before and after are compared here.
	s.recordSubscriptionChangeLocked(subURL, replaced, newProfiles)

	now := time.Now()
	usedIDs := make(map[string]struct{}, len(kept)+len(newProfiles))
	for _, p := range kept {
		if p.ID != "" {
			usedIDs[p.ID] = struct{}{}
		}
	}
	for i := range newProfiles {
		if newProfiles[i].ID == "" {
			newProfiles[i].ID = uuid.New().String()
		}
		for {
			if _, exists := usedIDs[newProfiles[i].ID]; !exists {
				break
			}
			newProfiles[i].ID = uuid.New().String()
		}
		usedIDs[newProfiles[i].ID] = struct{}{}
		if newProfiles[i].Category == "" {
			newProfiles[i].Category = InferCategory(newProfiles[i].Protocol)
		}
		newProfiles[i].CreatedAt = now
		newProfiles[i].UpdatedAt = now
		kept = append(kept, newProfiles[i])
	}

	s.data.Profiles = kept
	return s.saveUnlocked()
}

func (s *Store) UpdateProfile(p Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, existing := range s.data.Profiles {
		if existing.ID == p.ID {
			if p.Category == "" {
				p.Category = InferCategory(p.Protocol)
			}
			p.UpdatedAt = time.Now()
			s.data.Profiles[i] = p
			return s.saveUnlocked()
		}
	}
	return fmt.Errorf("profile %s not found", p.ID)
}

// UpdateProfileBatch updates multiple existing profiles in a single atomic file write
func (s *Store) UpdateProfileBatch(profiles []Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	updateMap := make(map[string]Profile, len(profiles))
	for _, p := range profiles {
		updateMap[p.ID] = p
	}

	for i, existing := range s.data.Profiles {
		if updated, ok := updateMap[existing.ID]; ok {
			if updated.Category == "" {
				updated.Category = InferCategory(updated.Protocol)
			}
			updated.UpdatedAt = now
			s.data.Profiles[i] = updated
		}
	}

	return s.saveUnlocked()
}

// SetProfileFavorite pins or unpins a profile.
//
// Pinning is deliberately a separate operation from UpdateProfile: the profile
// rows in the UI edit a copy of the struct, so routing a pin through a full
// profile update would mean the UI had to hold a complete, current copy of the
// profile just to toggle one flag — and any staleness in that copy would
// silently revert whatever else changed in the meantime.
func (s *Store) SetProfileFavorite(id string, favorite bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, ok := s.idIndex[id]
	if !ok || idx < 0 || idx >= len(s.data.Profiles) || s.data.Profiles[idx].ID != id {
		return fmt.Errorf("profile %s not found", id)
	}
	if s.data.Profiles[idx].Favorite == favorite {
		return nil // already in the requested state; do not rewrite the file
	}

	s.data.Profiles[idx].Favorite = favorite
	s.data.Profiles[idx].UpdatedAt = time.Now()
	return s.saveUnlocked()
}

// ServerFailureCooldown is how long a server that was just proven unreachable
// stays out of the automatic candidate lists.
//
// Ten minutes is the compromise the 2026-10-11 log asked for: a subscription
// host that refuses TCP was retried ~80 times in two minutes by failover, the
// quota monitor and the core itself, because nothing remembered that it had
// just failed. It is a cooldown and not a ban — a node that comes back is used
// again without the user touching anything.
const ServerFailureCooldown = 10 * time.Minute

// profileFailureReasonMaxLen bounds what is stored per failure. The reason is
// read by a human in the server list, and a core can emit a paragraph.
const profileFailureReasonMaxLen = 180

// ProfileIsCoolingDown reports whether a profile is still inside its failure
// cooldown at now.
//
// Pass window <= 0 for the default. A profile that never failed is never
// cooling down, which is what keeps an unmeasured subscription usable.
func ProfileIsCoolingDown(p Profile, now time.Time, window time.Duration) bool {
	if window <= 0 {
		window = ServerFailureCooldown
	}
	if p.LastFailureAt.IsZero() || p.ConsecutiveFailures <= 0 {
		return false
	}
	return now.Sub(p.LastFailureAt) < window
}

// RecordProfileFailure notes that a server was just proven unreachable.
func (s *Store) RecordProfileFailure(id string, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "unreachable"
	}
	// Truncate on rune boundaries: a reason can carry Persian text, and slicing a
	// UTF-8 string at an arbitrary byte index writes a broken character into the
	// store and into the report file.
	if r := []rune(reason); len(r) > profileFailureReasonMaxLen {
		reason = string(r[:profileFailureReasonMaxLen]) + "…"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx, ok := s.idIndex[id]
	if !ok || idx < 0 || idx >= len(s.data.Profiles) || s.data.Profiles[idx].ID != id {
		return fmt.Errorf("profile %s not found", id)
	}

	p := &s.data.Profiles[idx]
	// Only persist a real change. A connect attempt can fail for every one of a
	// hundred profiles in a subscription, and rewriting the store file per
	// failure would turn that into a hundred disk writes.
	if p.LastFailureReason == reason && !p.LastFailureAt.IsZero() && time.Since(p.LastFailureAt) < time.Minute {
		return nil
	}
	p.ConsecutiveFailures++
	p.LastFailureAt = time.Now()
	p.LastFailureReason = reason
	return s.saveUnlocked()
}

// ClearProfileFailure forgets a server's failure history after it has proven
// itself usable again.
func (s *Store) ClearProfileFailure(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx, ok := s.idIndex[id]
	if !ok || idx < 0 || idx >= len(s.data.Profiles) || s.data.Profiles[idx].ID != id {
		return fmt.Errorf("profile %s not found", id)
	}

	p := &s.data.Profiles[idx]
	if p.ConsecutiveFailures == 0 && p.LastFailureReason == "" && p.LastFailureAt.IsZero() {
		return nil
	}
	p.ConsecutiveFailures = 0
	p.LastFailureAt = time.Time{}
	p.LastFailureReason = ""
	return s.saveUnlocked()
}

func (s *Store) DeleteProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, p := range s.data.Profiles {
		if p.ID == id {
			s.data.Profiles = append(s.data.Profiles[:i], s.data.Profiles[i+1:]...)
			return s.saveUnlocked()
		}
	}
	return fmt.Errorf("profile %s not found", id)
}

// ClearAllProfiles removes every profile in a single atomic operation (one file write).
// Fix #9: Much more efficient than calling DeleteProfile() in a loop.
func (s *Store) ClearAllProfiles() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Profiles = []Profile{}
	return s.saveUnlocked()
}

// ClearProfilesByCategory removes all profiles matching category in one atomic write.
func (s *Store) ClearProfilesByCategory(category ProfileCategory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.data.Profiles[:0]
	for _, p := range s.data.Profiles {
		cat := p.Category
		if cat == "" {
			cat = InferCategory(p.Protocol)
		}
		if cat != category {
			kept = append(kept, p)
		}
	}
	s.data.Profiles = kept
	return s.saveUnlocked()
}

func (s *Store) DeleteProfilesByCategory(category ProfileCategory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var kept []Profile
	for _, p := range s.data.Profiles {
		cat := p.Category
		if cat == "" {
			cat = InferCategory(p.Protocol)
		}
		if cat != category {
			kept = append(kept, p)
		}
	}
	s.data.Profiles = kept
	return s.saveUnlocked()
}

func (s *Store) DeleteGroup(groupName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var kept []Profile
	for _, p := range s.data.Profiles {
		grp := p.SubscriptionName
		if grp == "" {
			grp = p.Group
		}
		if grp != groupName && p.Group != groupName {
			kept = append(kept, p)
		}
	}
	s.data.Profiles = kept
	return s.saveUnlocked()
}

func (s *Store) RenameGroup(oldName string, newName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Profiles {
		if s.data.Profiles[i].Group == oldName || s.data.Profiles[i].SubscriptionName == oldName {
			s.data.Profiles[i].Group = newName
			if s.data.Profiles[i].SubscriptionName == oldName {
				s.data.Profiles[i].SubscriptionName = newName
			}
			s.data.Profiles[i].UpdatedAt = time.Now()
		}
	}
	return s.saveUnlocked()
}

func (s *Store) GetSubscriptionURLByGroup(groupName string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.data.Profiles {
		if (p.SubscriptionName == groupName || p.Group == groupName) && p.SubscriptionURL != "" {
			return p.SubscriptionURL
		}
	}
	return ""
}

// GetSubscriptionURLs returns the distinct, non-empty subscription URLs that
// any stored profile belongs to. Used by the background quota monitor.
func (s *Store) GetSubscriptionURLs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, p := range s.data.Profiles {
		u := strings.TrimSpace(p.SubscriptionURL)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	return out
}

func (s *Store) GetSettings() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.data.Settings
	if st.Language == "" {
		st.Language = "fa"
	}
	if st.RoutingMode == "" {
		st.RoutingMode = "bypass_iran"
	}
	if st.ProxyMode == "" {
		st.ProxyMode = "tun"
	}
	// An unset or unrecognised value means the historical behaviour: the system
	// proxy is applied alongside the TUN adapter. Anything the app does not
	// understand is normalised rather than passed through to the engine, so a
	// hand-edited config file cannot silently select an unknown routing mode.
	switch st.TunSystemProxy {
	case "tun_only":
	case "dual":
	default:
		st.TunSystemProxy = "dual"
	}
	if st.RealDelayUrl == "" {
		st.RealDelayUrl = "http://cp.cloudflare.com/generate_204"
	}
	if st.DNSServer == "" {
		// The same default the store is created with and the same one the config
		// builder falls back to — three places used to disagree about this value.
		st.DNSServer = DefaultPrimaryResolver
	}
	if st.LogLevel == "" {
		st.LogLevel = "info"
	}
	if st.WarpCleanIP == "" {
		st.WarpCleanIP = "162.159.192.1"
	}
	if st.WarpPort <= 0 {
		st.WarpPort = 2408
	}
	return st
}

// SaveSettings persists a new Settings block.
//
// The incoming struct replaces the stored one wholesale, and that is deliberate.
//
// This used to run `preserveSettingsFields` over the incoming value, which
// copied any stored non-zero value back over an incoming zero. A Go bool cannot
// distinguish "the user turned this off" from "the caller never knew about this
// field", so that rule made several settings impossible to change:
// AutoFailoverEnabled, RemoteControllerLANOnly, RemoteControllerTLS and
// CustomSNIPool could be switched on but never off — the Settings page showed
// toggles that silently snapped back — and removing the last custom SNI was
// equally impossible.
//
// The data loss it guarded against does not happen in practice. Every caller
// supplies a complete Settings: the frontend spreads the object it got from
// GetSettings, and every Go handler mutates a GetSettings result. A caller that
// wants to change one field must read, mutate and write back, which is what all
// of them already do — and what this comment now requires of any new one.
func (s *Store) SaveSettings(settings Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Settings = settings
	return s.saveUnlocked()
}

func (s *Store) GetGroupsByCategory(category ProfileCategory) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	groupMap := make(map[string]bool)
	for _, p := range s.data.Profiles {
		cat := p.Category
		if cat == "" {
			cat = InferCategory(p.Protocol)
		}
		if (category == "" || cat == category) && strings.TrimSpace(p.Group) != "" {
			groupMap[p.Group] = true
		}
	}

	var groups []string
	for g := range groupMap {
		groups = append(groups, g)
	}
	return groups
}

// DeduplicateProfiles removes duplicate nodes based on server:port and raw URI
func (s *Store) DeduplicateProfiles(category ProfileCategory) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	seen := make(map[string]bool)
	var kept []Profile
	removedCount := 0

	for _, p := range s.data.Profiles {
		cat := p.Category
		if cat == "" {
			cat = InferCategory(p.Protocol)
		}
		if category != "" && cat != category {
			kept = append(kept, p)
			continue
		}

		key := ""
		if p.RawURI != "" {
			key = p.RawURI
		} else if p.Server != "" && p.Port > 0 {
			key = fmt.Sprintf("%s:%d:%s", p.Server, p.Port, p.Protocol)
		} else {
			key = p.Name
		}

		if seen[key] {
			removedCount++
			continue
		}
		seen[key] = true
		kept = append(kept, p)
	}

	if removedCount > 0 {
		s.data.Profiles = kept
		_ = s.saveUnlocked()
	}
	return removedCount
}

// RemoveDeadProfiles removes all profiles that have failed ping (latency <= 0)
func (s *Store) RemoveDeadProfiles(category ProfileCategory) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	var kept []Profile
	removedCount := 0

	for _, p := range s.data.Profiles {
		cat := p.Category
		if cat == "" {
			cat = InferCategory(p.Protocol)
		}
		if category != "" && cat != category {
			kept = append(kept, p)
			continue
		}

		if p.LatencyMs <= 0 {
			removedCount++
			continue
		}
		kept = append(kept, p)
	}

	if removedCount > 0 {
		s.data.Profiles = kept
		_ = s.saveUnlocked()
	}
	return removedCount
}

// ExportAllProfiles exports all profile links in a category as a newline-separated string
func (s *Store) ExportAllProfiles(category ProfileCategory) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var lines []string
	for _, p := range s.data.Profiles {
		cat := p.Category
		if cat == "" {
			cat = InferCategory(p.Protocol)
		}
		if category != "" && cat != category {
			continue
		}

		if p.RawURI != "" {
			lines = append(lines, p.RawURI)
		} else if p.ConfigContent != "" {
			lines = append(lines, fmt.Sprintf("# %s\n%s", p.Name, p.ConfigContent))
		}
	}
	return strings.Join(lines, "\n")
}

// RecordConnectionSession saves a completed session to history.json
func (s *Store) RecordConnectionSession(session ConnectionSession) error {
	s.mu.Lock()
	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	s.history = append([]ConnectionSession{session}, s.history...)
	if len(s.history) > 100 {
		s.history = s.history[:100]
	}
	historyCopy := append([]ConnectionSession(nil), s.history...)
	historyFile := s.historyFile
	s.mu.Unlock()

	bytes, err := json.MarshalIndent(historyCopy, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := historyFile + ".tmp"
	if err := os.WriteFile(tmpFile, bytes, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpFile, historyFile); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}
	return nil
}

// GetConnectionHistory retrieves all recorded connection sessions
func (s *Store) GetConnectionHistory() []ConnectionSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()
	result := make([]ConnectionSession, len(s.history))
	copy(result, s.history)
	return result
}

// ClearConnectionHistory deletes all session logs
func (s *Store) ClearConnectionHistory() error {
	s.mu.Lock()
	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	s.history = []ConnectionSession{}
	historyFile := s.historyFile
	s.mu.Unlock()

	tmpFile := historyFile + ".tmp"
	if err := os.WriteFile(tmpFile, []byte("[]"), 0600); err != nil {
		return err
	}
	if err := os.Rename(tmpFile, historyFile); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}
	return nil
}

// ExportFullBackupJSON exports all profiles and settings as a JSON string bundle
func (s *Store) ExportFullBackupJSON() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// A backup file is built to travel: to another machine, another Windows
	// account, a cloud drive, an attachment. A DPAPI-sealed password cannot be
	// read back in any of those places, so shipping it would produce a restore
	// that looks complete and silently has no credentials — and it would put the
	// one copy of the config most likely to leave the machine in the clear.
	// Passwords are therefore omitted outright; profiles restore intact and the
	// user re-enters the few accounts that need one.
	exportProfiles := make([]Profile, len(s.data.Profiles))
	copy(exportProfiles, s.data.Profiles)
	for i := range exportProfiles {
		exportProfiles[i].Password = ""
	}

	bundle := FullBackupBundle{
		Version:      "1.4.0",
		ExportedAt:   time.Now(),
		Profiles:     exportProfiles,
		Settings:     s.data.Settings,
		ProfileCount: len(exportProfiles),
	}

	bytes, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ImportFullBackupJSON restores profiles and settings from a JSON bundle
func (s *Store) ImportFullBackupJSON(jsonStr string, overwrite bool) (int, error) {
	var bundle FullBackupBundle
	if err := json.Unmarshal([]byte(jsonStr), &bundle); err != nil {
		return 0, fmt.Errorf("invalid backup format: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if overwrite {
		s.data.Profiles = bundle.Profiles
		s.data.Settings = bundle.Settings
		s.rebuildIndexLocked()
		_ = s.saveUnlocked()
		return len(bundle.Profiles), nil
	}

	// Merge mode
	importedCount := 0
	usedIDs := make(map[string]struct{}, len(s.data.Profiles)+len(bundle.Profiles))
	for _, existing := range s.data.Profiles {
		if existing.ID != "" {
			usedIDs[existing.ID] = struct{}{}
		}
	}
	for _, p := range bundle.Profiles {
		if p.ID == "" {
			p.ID = uuid.New().String()
		}
		if _, exists := usedIDs[p.ID]; exists {
			continue
		}
		usedIDs[p.ID] = struct{}{}
		s.data.Profiles = append(s.data.Profiles, p)
		importedCount++
	}
	s.rebuildIndexLocked()
	_ = s.saveUnlocked()
	return importedCount, nil
}

// GetHistory returns a copy of connection session history
func (s *Store) GetHistory() []ConnectionSession {
	return s.GetConnectionHistory()
}

// NewStoreWithDir creates a store initialized with a custom directory for testing or isolated environments
func NewStoreWithDir(dir string) *Store {
	return &Store{
		dataDir:     dir,
		dataFile:    filepath.Join(dir, "config.json"),
		historyFile: filepath.Join(dir, "history.json"),
		idIndex:     make(map[string]int),
		data: StoreData{
			Profiles: []Profile{},
			Settings: Settings{
				AutoConnect:             false,
				StartMinimized:          false,
				StartOnBoot:             false,
				KillSwitchEnabled:       false,
				PrivacyShieldEnabled:    true,
				RemoteControllerLANOnly: true,
				RemoteControllerTLS:     true,
				NotificationsEnabled:    true,
				DNSServer:               "1.1.1.1",
				DNSFallback:             "8.8.8.8",
				Language:                "fa",
				Theme:                   "dark",
				RoutingMode:             "bypass_iran",
				ProxyMode:               "tun",
			},
		},
	}
}
