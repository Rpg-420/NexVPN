export type ProfileCategory = 'v2ray' | 'wireguard_warp' | 'openvpn' | 'aether' | 'psiphon'

export type Protocol = 'wireguard' | 'xray' | 'singbox' | 'openvpn' | 'hysteria2' | 'warp' | 'aether' | 'psiphon'

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
