export type ProfileCategory = 'v2ray' | 'wireguard_warp' | 'openvpn' | 'aether' | 'psiphon'

export type Protocol = 'wireguard' | 'xray' | 'singbox' | 'openvpn' | 'hysteria2' | 'tuic' | 'warp' | 'aether' | 'psiphon'

export interface Profile {
  id: string
  name: string
  protocol: Protocol
  category: ProfileCategory
  config_content: string
  raw_uri?: string
  server?: string
  port?: number
  group?: string
  country?: string
  country_code?: string
  latency_ms?: number
  real_delay_ms?: number
  subscription_url?: string
  subscription_name?: string
  description?: string
  tags?: string[]

  // OpenVPN credentials
  username?: string
  password?: string

  // Cloudflare WARP parameters
  warp_endpoint?: string
  warp_clean_ip?: string
  warp_reserved?: string

  /** Pinned to the top of its list and into the tray's server submenu. */
  favorite?: boolean
  /**
   * Per-session traffic alert threshold in MB. 0 means no alert.
   *
   * It is an alert rather than a hard cut: the cores expose no portable way to
   * enforce a byte ceiling, so the app reports the threshold being crossed
   * instead of pretending it can stop it.
   */
  session_data_cap_mb?: number

  created_at: string
  updated_at: string
  last_used?: string
  usage_count: number
}

export type ConnectionState =
  | 'disconnected'
  | 'connecting'
  | 'connected'
  | 'disconnecting'
  | 'error'

export interface ConnectionStatus {
  state: ConnectionState
  profile_id?: string
  profile_name?: string
  protocol?: Protocol
  connected_at?: string
  server?: string
  error_msg?: string
  duration?: number
}

export interface TrafficStats {
  bytes_received: number
  bytes_sent: number
  speed_download: number
  speed_upload: number
  packets_received: number
  packets_sent: number
}

export interface IPInfo {
  ip: string
  country: string
  country_code: string
  city: string
  isp: string
}

export interface Settings {
  auto_connect: boolean
  start_minimized: boolean
  start_on_boot: boolean
  kill_switch_enabled: boolean
  dns_server: string
  dns_fallback: string
  language: 'fa' | 'en' | string
  theme: string
  routing_mode: 'global' | 'bypass_iran' | 'direct_iran_block_ads' | string
  proxy_mode: 'system_proxy' | 'tun' | string
  // 'dual' applies the Windows system proxy alongside the TUN adapter (the
  // default); 'tun_only' leaves the adapter alone and does not touch the proxy.
  tun_system_proxy: 'dual' | 'tun_only' | string
  real_delay_url: string
  last_profile_id?: string
  log_level: 'debug' | 'info' | 'warning' | 'error'
  max_log_entries: number
  auto_reconnect: boolean
  reconnect_attempts: number
  warp_clean_ip?: string
  warp_port?: number
  warp_license_key?: string
  warp_account_type?: 'free' | 'plus' | string
  fragment_enabled?: boolean
  fragment_size?: string
  fragment_sleep?: string
  app_routing_mode?: 'all' | 'proxy_selected' | 'bypass_selected' | string
  selected_apps?: string[]
  auto_failover?: boolean
  // NOTE: must be mirrored in TS or SaveSettings() silently wipes it (Go writes it in ToggleAutoFailover)
  auto_failover_enabled?: boolean
  subscription_auto_update_hours?: number
  multi_hop_warp?: boolean
  ip_strategy?: 'prefer_ipv4' | 'prefer_ipv6' | 'ipv4_only' | 'ipv6_only' | string
  block_ads?: boolean
  bypass_lan?: boolean
  custom_direct_domains?: string[]
  custom_proxy_domains?: string[]
  custom_block_domains?: string[]
  amnezia_jc?: number
  amnezia_jmin?: number
  amnezia_jmax?: number
  amnezia_s1?: number
  amnezia_s2?: number
  amnezia_h1?: number
  amnezia_h2?: number
  amnezia_h3?: number
  amnezia_h4?: number
  show_mini_widget?: boolean
  v2ray_core?: 'auto' | 'xray' | 'singbox' | string
  allow_lan?: boolean
  game_mode_enabled?: boolean
  sound_effects_enabled?: boolean
  monthly_quota_gb?: number
  quota_alert_percentage?: number
  stealth_mode_enabled?: boolean
  privacy_shield_enabled?: boolean
  notifications_enabled?: boolean
  custom_doh_url?: string
  smart_routing_enabled?: boolean
  ai_routing_mode?: string
  streaming_routing_mode?: string
  domestic_routing_mode?: string
  auto_mtu_enabled?: boolean
  custom_mtu?: number
  quota_webhook_url?: string
  // NOTE: must be mirrored in TS or SaveSettings() silently wipes it (Go writes it in AddCustomSNI/RemoveCustomSNI)
  custom_sni_pool?: string[]
  // LAN controller hardening. Both default to true in Go; the backend refuses
  // to let a partial payload turn them off (see preserveSettingsFields).
  remote_controller_lan_only?: boolean
  remote_controller_tls?: boolean
  // Opt-in to installing a proxy core with no verifiable SHA-256 digest.
  allow_unverified_core_downloads?: boolean
  // WebView2 rendering fallbacks. Read once at startup, so both need a restart.
  webview_disable_gpu?: boolean
  webview_disable_code_integrity?: boolean
  // SSID-driven automatic connect / disconnect. First matching rule wins.
  network_rules?: NetworkRule[]
  // User routing overrides layered on top of routing_mode.
  routing_rules?: RoutingRule[]
  // Scheduled connect / disconnect rules, evaluated once a minute.
  schedule_rules?: ScheduleRule[]
  // Whether the app may look for a newer release. Sends no identifiers.
  app_update_check_enabled?: boolean
}

/** Action taken when the joined Wi-Fi network matches an SSID pattern. */
export type NetworkRuleAction = 'connect' | 'direct' | 'skip'

/** One SSID automation rule. Mirrors config.NetworkRule in Go. */
export interface NetworkRule {
  id: string
  /** SSID to match. Case-insensitive; `*` and `?` are wildcards. */
  pattern: string
  action: NetworkRuleAction
  enabled: boolean
}

/** What a routing override does with the matched traffic. */
export type RoutingRuleAction = 'proxy' | 'direct' | 'block'

/** How the value of a routing rule is interpreted. */
export type RoutingRuleKind = 'domain' | 'ip' | 'cidr'

/** One user routing override. Mirrors config.RoutingRule in Go. */
export interface RoutingRule {
  id: string
  kind: RoutingRuleKind
  value: string
  action: RoutingRuleAction
  enabled: boolean
}

/** One scheduled connect or disconnect. Mirrors config.ScheduleRule in Go. */
export interface ScheduleRule {
  id: string
  action: 'connect' | 'disconnect'
  /** "HH:MM" in 24-hour local time. */
  time: string
  /** Weekdays, 0 = Sunday … 6 = Saturday. Empty means every day. */
  days: number[]
  profile_id?: string
  enabled: boolean
  /** "YYYY-MM-DDTHH:MM" of the slot that already fired. */
  last_run?: string
}

/** One parsed launch-diary line. Mirrors diary.Entry in Go. */
export interface DiaryEntry {
  time: string
  category: string
  event: string
  fields: Record<string, string>
}

/** One day of aggregated session data. Mirrors config.DailyUsage in Go. */
export interface DailyUsage {
  date: string
  sessions: number
  duration_sec: number
  bytes_rx: number
  bytes_tx: number
}

/** One server's share of the recorded history. Mirrors config.ServerUsage in Go. */
export interface ServerUsage {
  profile_name: string
  protocol: string
  sessions: number
  duration_sec: number
  bytes: number
}

/** Aggregated connection history. Mirrors ConnectionStatistics in Go. */
export interface ConnectionStatistics {
  total_sessions: number
  total_duration_sec: number
  total_bytes_rx: number
  total_bytes_tx: number
  average_duration_sec: number
  window_days: number
  uptime_percent: number
  daily: DailyUsage[]
  top_servers: ServerUsage[]
}

/** Result of the IPv6 leak probe. Mirrors IPv6LeakResult in Go. */
export interface IPv6LeakResult {
  tunnel_active: boolean
  ipv6_reachable: boolean
  leaking: boolean
  observed_ip: string
  detail: string
}

/** Result of an update check. Mirrors UpdateInfo in Go. */
export interface UpdateInfo {
  current_version: string
  latest_version: string
  update_available: boolean
  release_url: string
  release_notes: string
  published_at: string
  checked_at: string
  detail: string
}

/** What one subscription refresh changed. Mirrors config.SubscriptionChange in Go. */
export interface SubscriptionChange {
  url: string
  group: string
  added: number
  removed: number
  renamed: number
  unchanged: number
  added_names: string[]
  removed_names: string[]
  renamed_names: string[]
  at: string
}

/** Advanced Psiphon engine options. Mirrors core.PsiphonOptions in Go. */
export interface PsiphonOptions {
  tunnel_protocol: string
  socks_port: number
  http_port: number
  disable_split: boolean
}

/** One selectable region returned by GetPsiphonRegions. */
export interface PsiphonRegion {
  code: string
  label: string
}

/** Status of the LAN mobile remote controller. Mirrors MobileRemoteInfo in Go. */
export interface MobileRemoteInfo {
  running: boolean
  url: string
  auth_url: string
  pin: string
  idle_timeout_sec: number
  /** True when the controller is served over HTTPS (self-signed cert). */
  tls_active: boolean
  /** True when non-RFC1918 peers are refused at the listener. */
  lan_only: boolean
}

/**
 * Aggregated traffic usage for the quota window. Mirrors UsageReport in Go.
 * `bytes_from_history` is the sum of recorded sessions; the live session is
 * reported separately because a session is only written to history on
 * disconnect.
 */
export interface UsageReport {
  window_start: string
  window_end: string
  /** "calendar_month" or "rolling_30d". */
  window_kind: string
  bytes_from_history: number
  bytes_current_session: number
  total_bytes: number
  session_count: number
  /** 0 means no quota is configured. */
  quota_bytes: number
  used_percent: number
  history_available: boolean
}

export interface SNIPoolItem {
  domain: string
  provider: string
  latency_ms: number
  status: 'working' | 'blocked' | 'untested' | string
}

/**
 * Subscription usage/quota metadata. Mirrors parser.SubscriptionUserInfo in Go.
 * `expire_date` is a Unix timestamp in seconds (0 == no expiry).
 */
export interface SubscriptionUserInfo {
  upload_bytes: number
  download_bytes: number
  total_bytes: number
  used_bytes: number
  expire_date: number
  remaining_days: number
  percent_used: number
}

/**
 * Full subscription fetch result: the raw body plus optional usage metadata.
 * Mirrors parser.SubscriptionFetchResult in Go.
 */
export interface SubscriptionFetchResult {
  content: string
  user_info?: SubscriptionUserInfo
}

/**
 * Result of a live ISP/ASN detection. Mirrors discovery.ISPRecommendation in Go. * `detected` is false when the lookup could not complete; the remaining advice
 * fields then hold generic (not measured) guidance.
 */
export interface ISPRecommendation {
  isp_name: string
  isp_name_en: string
  /** "mci" | "mtn" | "rtl" | "fixed" | "other" */
  isp_category: string
  ip: string
  country: string
  country_code: string
  asn: string
  recommended_protocol: string
  recommended_fragment: string
  fragment_size: string
  fragment_sleep: string
  recommended_dns: string
  status_score: number
  detection_ms: number
  detected: boolean
  advice: string
  advice_en: string
}


export interface QuotaAlertInfo {
  subscription_url: string
  subscription_name: string
  total_bytes: number
  used_bytes: number
  remaining_bytes: number
  used_percent: number
  expires_at?: string
  days_remaining: number
  alert_triggered: boolean
  alert_reason: string
}

export interface HealthStatus {
  quality_score: number
  quality_level: 'excellent' | 'good' | 'fair' | 'poor' | 'unhealthy' | string
  latency_ms: number
  jitter_ms: number
  packet_loss_pct: number
  consecutive_fails: number
  is_alive: boolean
  last_checked?: string
}

export interface ConnectionSession {
  id: string
  profile_id: string
  profile_name: string
  protocol: string
  server: string
  connected_at: string
  disconnected_at: string
  duration_sec: number
  bytes_received: number
  bytes_sent: number
  quality_score: number
  reason: string
}

export interface DNSQueryProbe {
  name: string
  target: string
  resolved_ip?: string
  status: 'ok' | 'poisoned' | 'failed' | string
  latency_ms: number
}

export interface DNSLeakReport {
  is_leaking: boolean
  outbound_ip: string
  outbound_isp: string
  outbound_country: string
  dns_resolver_ip: string
  dns_resolver_country: string
  risk_level: 'safe' | 'medium' | 'high' | string
  summary: string
  probes: DNSQueryProbe[]
  checked_at?: string
}

export interface BatchTestResult {
  profile_id: string
  profile_name: string
  latency_ms: number
  success: boolean
}

export interface RepairStep {
  name: string
  command: string
  status: 'pending' | 'running' | 'success' | 'failed'
  message: string
}

export interface EmergencyRepairReport {
  overall_status: 'success' | 'partial' | 'failed'
  steps: RepairStep[]
  timestamp: string
}

export interface RadarProtocolResult {
  name: string
  protocol: string
  status: 'excellent' | 'good' | 'throttled' | 'blocked'
  latency_ms: number
  packet_loss: number
  description: string
}

export interface RadarReport {
  isp_name: string
  ip_address: string
  censorship_level: string
  best_protocol: string
  recommended_action: string
  protocols: RadarProtocolResult[]
  timestamp: string
}

export interface BinaryInfo {
  name: string
  filename: string
  status: 'installed' | 'missing'
  path: string
  version?: string
  download_url: string
}

export type LogLevel = 'debug' | 'info' | 'warning' | 'error'

export interface LogEntry {
  timestamp: string
  level: LogLevel
  message: string
  source: string
}

export interface CleanIPResult {
  ip: string
  port: number
  latency_ms: number
  loss_rate: number
  location?: string
}

export interface ScanProgress {
  total: number
  scanned: number
  found: number
  current_ip: string
  results: CleanIPResult[]
}

export interface SpeedTestProgress {
  phase: 'ping' | 'download' | 'upload' | 'complete' | 'error'
  download_mbps: number
  upload_mbps: number
  ping_ms: number
  jitter_ms: number
  progress: number
  error_msg?: string
}

export interface DiagnosticItem {
  id: string
  title: string
  description: string
  status: 'ok' | 'warning' | 'error' | 'running'
  latency_ms?: number
  detail: string
}

export interface DoctorReport {
  overall_status: 'healthy' | 'degraded' | 'critical'
  summary: string
  items: DiagnosticItem[]
  timestamp: number
}

export interface RepairResult {
  success: boolean
  message: string
  actions: string[]
}

export interface InstalledApp {
  id: string
  name: string
  exe_name: string
  category: 'browser' | 'social' | 'game' | 'media' | 'tool' | 'iranian'
  is_running: boolean
  icon_name: string
}

export interface SubscriptionUserInfo {
  upload_bytes: number
  download_bytes: number
  total_bytes: number
  used_bytes: number
  expire_date: number
  remaining_days: number
  percent_used: number
}

export interface SubscriptionFetchResult {
  content: string
  user_info?: SubscriptionUserInfo
}

export interface TelegramProxy {
  id: string
  type: 'mtproto' | 'socks5'
  server: string
  port: number
  secret?: string
  username?: string
  password?: string
  latency_ms: number
  is_alive: boolean
  country?: string
  country_code?: string
  link: string
  source?: string
}

export interface TelegramTestProgress {
  tested_count: number
  total_count: number
  current: string
  results?: TelegramProxy[]
}

export interface FreeConfigItem {
  id: string
  name: string
  protocol: 'vless' | 'vmess' | 'trojan' | 'shadowsocks' | 'hysteria2' | 'tuic' | 'wireguard' | string
  server: string
  port: number
  raw_uri: string
  latency_ms: number
  is_alive: boolean
  country?: string
  country_code?: string
  source_name?: string
  network?: string
  security?: string
}

export interface FreeConfigTestProgress {
  tested_count: number
  total_count: number
  current: string
  results?: FreeConfigItem[]
}

// --- Public OpenVPN server catalogues -------------------------------------

export interface OpenVPNSourceInfo {
  id: string
  name: string
  description: string
  url: string
  operator: string
  license: string
  needs_credentials: boolean
  default_port: number
  supports_probe: boolean
}

export interface OpenVPNServer {
  id: string
  source_id: string
  host_name: string
  ip: string
  port: number
  country: string
  country_code: string
  operator: string
  score: number
  ping_ms: number
  speed_bps: number
  sessions: number
  latency_ms: number
  is_alive: boolean
}

export interface OpenVPNServerTestProgress {
  tested_count: number
  total_count: number
  current: string
  results?: OpenVPNServer[]
}

// Streamed by PingAllProfiles while it sweeps a category, so the UI can show a
// counter instead of an apparently hung button.
export interface ProfilePingProgress {
  tested_count: number
  total_count: number
  current: string
  latency_ms: number
  alive: boolean
}

// --- TunnelBear ------------------------------------------------------------

export interface TunnelBearCountry {
  id: string
  /** English label. */
  name: string
  /** Persian label; the catalogue carries both so the two cannot drift. */
  name_fa: string
  /** ISO 3166-1 alpha-2, for the flag. */
  code: string
  host: string
}

/** A saved TunnelBear login. The password is never part of this shape. */
export interface TunnelBearAccount {
  id: string
  username: string
  /** What the core last reported: unknown, ok, or failed. */
  auth: 'unknown' | 'ok' | 'failed' | string
  last_error?: string
  last_connected?: string
  active: boolean
}

export interface CloudflareTarget {
  ip: string
  port: number
  latency_ms: number
  loss_rate: number
  location?: string
  mode: 'warp' | 'cdn' | string
  is_clean: boolean
}

export interface CloudflareScanProgress {
  total: number
  scanned: number
  found: number
  current_ip: string
  mode: string
  results: CloudflareTarget[]
}

export interface FragmentCandidate {
  id: string
  size_range: string
  sleep_range: string
  min_size: number
  max_size: number
  min_sleep_ms: number
  max_sleep_ms: number
  success_rate: number
  avg_latency: number
  status: 'optimal' | 'passed' | 'failed' | string
  score: number
  recommended: boolean
}

export interface FragmentScanProgress {
  total: number
  scanned: number
  current: string
  best_setting?: FragmentCandidate
  results: FragmentCandidate[]
}

export interface ServiceUnblockResult {
  id: string
  name: string
  category: 'ai' | 'streaming' | 'social' | 'gaming' | 'dev' | string
  status: 'unlocked' | 'blocked' | 'restricted' | 'error' | string
  latency_ms: number
  http_status: number
  region?: string
  message: string
}

export interface GamingOptimizationStatus {
  active: boolean
  tcp_ack_frequency_applied?: boolean
  tcp_nodelay_applied?: boolean
  network_throttling_tuned?: boolean
  system_responsiveness?: boolean
  optimized_interfaces?: string[]
  message: string
}

export interface BackupSummary {
  profiles_imported: number
  settings_restored: boolean
  created_at: string
  is_encrypted: boolean
}

export interface DiagnosticReport {
  healthy: boolean
  issues_found: string[]
  wintun_status: string
  proxy_status: string
  port_status: Record<number, string>
  binaries_status: Record<string, string>
  dns_status: string
  platform: string
  timestamp: string
}

export interface DiagnosticRepairResult {
  success: boolean
  actions_taken: string[]
  post_repair_health: DiagnosticReport
  message: string
}



export interface DNSScanResult {
  server_name: string
  address: string
  is_doh: boolean
  latency: number
  is_success: boolean
  raw_value: string
}
