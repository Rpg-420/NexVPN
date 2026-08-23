package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Protocol string

const (
	ProtocolWireGuard Protocol = "wireguard"
	ProtocolXray      Protocol = "xray"
	ProtocolSingBox   Protocol = "singbox"
	ProtocolOpenVPN   Protocol = "openvpn"
	ProtocolHysteria2 Protocol = "hysteria2"
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

	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	LastUsed   time.Time `json:"last_used,omitempty"`
	UsageCount int       `json:"usage_count"`
}

type Settings struct {
	AutoConnect                 bool     `json:"auto_connect"`
	StartMinimized              bool     `json:"start_minimized"`
	StartOnBoot                 bool     `json:"start_on_boot"`
	KillSwitchEnabled           bool     `json:"kill_switch_enabled"`
	DNSServer                   string   `json:"dns_server"`
	DNSFallback                 string   `json:"dns_fallback"`
	Language                    string   `json:"language"`
	Theme                       string   `json:"theme"`
	RoutingMode                 string   `json:"routing_mode"` // "global", "bypass_iran", "direct_iran_block_ads"
	ProxyMode                   string   `json:"proxy_mode"`   // "tun", "system_proxy"
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
	NotificationsEnabled        bool     `json:"notifications_enabled"`  // Native Windows Toast notifications
	CustomDoHUrl                string   `json:"custom_doh_url,omitempty"`
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

type StoreData struct {
	Profiles []Profile `json:"profiles"`
	Settings Settings  `json:"settings"`
}

type Store struct {
	mu          sync.RWMutex
	dataDir     string
	dataFile    string
	historyFile string
	data        StoreData
	idIndex     map[string]int
}

func NewStore() *Store {
	home, _ := os.UserConfigDir()
	if home == "" {
		home = os.Getenv("APPDATA")
	}
	dir := filepath.Join(home, "NexVPN")
	return &Store{
		dataDir:     dir,
		dataFile:    filepath.Join(dir, "config.json"),
		historyFile: filepath.Join(dir, "history.json"),
		idIndex:     make(map[string]int),
		data: StoreData{
			Profiles: []Profile{},
			Settings: Settings{
				AutoConnect:                 false,
				StartMinimized:              false,
				StartOnBoot:                 false,
				KillSwitchEnabled:           false,
				PrivacyShieldEnabled:        true,
				NotificationsEnabled:        true,
				DNSServer:                   "https://1.1.1.1/dns-query",
				DNSFallback:                 "8.8.8.8",
				Language:                    "fa",
				Theme:                       "dark",
				RoutingMode:                 "bypass_iran",
				ProxyMode:                   "tun", // Default to high-performance TUN mode
				RealDelayUrl:                "http://cp.cloudflare.com/generate_204",
				LogLevel:                    "info",
				MaxLogEntries:               1000,
				AutoReconnect:               true,
				ReconnectAttempts:           3,
				WarpCleanIP:                 "162.159.192.1",
				WarpPort:                    2408,
				WarpAccountType:             "free",
				FragmentEnabled:             false,
				FragmentSize:                "100-200",
				FragmentSleep:               "10-20",
				AppRoutingMode:              "all",
				SelectedApps:                []string{"telegram.exe", "discord.exe", "chrome.exe", "spotify.exe"},
				AutoFailover:                true,
				SubscriptionAutoUpdateHours: 12,
				MultiHopWarp:                false,
				IPStrategy:                  "prefer_ipv4",
				BlockAds:                    false,
				BypassLAN:                   true,
				CustomDirectDomains:         []string{},
				CustomProxyDomains:          []string{},
				CustomBlockDomains:          []string{},
				AmneziaJc:                   4,
				AmneziaJmin:                 40,
				AmneziaJmax:                 70,
				AmneziaS1:                   15,
				AmneziaS2:                   20,
				AmneziaH1:                   1,
				AmneziaH2:                   2,
				AmneziaH3:                   3,
				AmneziaH4:                   4,
				ShowMiniWidget:              false,
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

	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
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

	// Migrate categories for legacy profiles
	for i := range s.data.Profiles {
		p := &s.data.Profiles[i]
		if p.Category == "" {
			p.Category = InferCategory(p.Protocol)
		}
	}
	s.rebuildIndexLocked()

	return nil
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

func (s *Store) saveUnlocked() error {
	s.rebuildIndexLocked()
	if err := os.MkdirAll(s.dataDir, 0755); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.dataFile, bytes, 0644)
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	for _, p := range s.data.Profiles {
		if p.SubscriptionURL != subURL || subURL == "" {
			kept = append(kept, p)
		}
	}

	now := time.Now()
	for i := range newProfiles {
		if newProfiles[i].ID == "" {
			newProfiles[i].ID = uuid.New().String()
		}
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
	if st.RealDelayUrl == "" {
		st.RealDelayUrl = "http://cp.cloudflare.com/generate_204"
	}
	if st.DNSServer == "" {
		st.DNSServer = "1.1.1.1"
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
	defer s.mu.Unlock()

	if s.historyFile == "" {
		s.historyFile = filepath.Join(s.dataDir, "history.json")
	}

	var history []ConnectionSession
	if data, err := os.ReadFile(s.historyFile); err == nil {
		_ = json.Unmarshal(data, &history)
	}

	// Prepend newest session, cap at 100 entries
	history = append([]ConnectionSession{session}, history...)
	if len(history) > 100 {
		history = history[:100]
	}

	bytes, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.historyFile, bytes, 0600)
}

// GetConnectionHistory retrieves all recorded connection sessions
func (s *Store) GetConnectionHistory() []ConnectionSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.historyFile == "" {
		s.historyFile = filepath.Join(s.dataDir, "history.json")
	}

	var history []ConnectionSession
	if data, err := os.ReadFile(s.historyFile); err == nil {
		_ = json.Unmarshal(data, &history)
	}
	return history
}

// ClearConnectionHistory deletes all session logs
func (s *Store) ClearConnectionHistory() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.historyFile == "" {
		s.historyFile = filepath.Join(s.dataDir, "history.json")
	}
	return os.WriteFile(s.historyFile, []byte("[]"), 0600)
}

// ExportFullBackupJSON exports all profiles and settings as a JSON string bundle
func (s *Store) ExportFullBackupJSON() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bundle := FullBackupBundle{
		Version:      "1.3.5",
		ExportedAt:   time.Now(),
		Profiles:     s.data.Profiles,
		Settings:     s.data.Settings,
		ProfileCount: len(s.data.Profiles),
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
	for _, p := range bundle.Profiles {
		if p.ID == "" {
			p.ID = uuid.New().String()
		}
		if _, exists := s.idIndex[p.ID]; !exists {
			s.data.Profiles = append(s.data.Profiles, p)
			importedCount++
		}
	}
	s.rebuildIndexLocked()
	_ = s.saveUnlocked()
	return importedCount, nil
}
