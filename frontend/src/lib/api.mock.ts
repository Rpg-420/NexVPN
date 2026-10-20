import type {
  Profile,
  ConnectionStatus,
  TrafficStats,
  Settings,
  LogEntry,
  BinaryInfo,
} from '../types'

const mockProfiles: Profile[] = [
  {
    id: '1',
    name: 'Amsterdam VLESS Reality VIP',
    protocol: 'singbox',
    category: 'v2ray',
    config_content: '',
    raw_uri: 'vless://uuid-1234@ams.nexvpn.dev:443?security=reality&sni=yahoo.com&fp=chrome&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA#Amsterdam-Reality',
    server: 'ams.nexvpn.dev',
    port: 443,
    group: 'Subscriptions',
    latency_ms: 58,
    real_delay_ms: 95,
    description: 'VLESS Reality anti-censorship node',
    tags: ['nl', 'reality'],
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    usage_count: 14,
  },
  {
    id: '2',
    name: 'Cloudflare WARP (162.159.192.1)',
    protocol: 'warp',
    category: 'wireguard_warp',
    config_content: '',
    server: '162.159.192.1',
    port: 2408,
    group: 'Cloudflare WARP',
    latency_ms: 45,
    real_delay_ms: 82,
    description: 'High-speed Cloudflare WARP Clean IP',
    tags: ['warp', 'cf'],
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    usage_count: 8,
  },
]

const mockStatus: ConnectionStatus = {
  state: 'disconnected',
}

const mockStats: TrafficStats = {
  bytes_received: 0,
  bytes_sent: 0,
  speed_download: 0,
  speed_upload: 0,
  packets_received: 0,
  packets_sent: 0,
}

const mockSettings: Settings = {
  auto_connect: false,
  start_minimized: false,
  start_on_boot: false,
  kill_switch_enabled: false,
  dns_server: '1.1.1.1',
  dns_fallback: '8.8.8.8',
  language: 'fa',
  theme: 'dark',
  routing_mode: 'bypass_iran',
  proxy_mode: 'tun',
  tun_system_proxy: 'dual',
  real_delay_url: 'http://cp.cloudflare.com/generate_204',
  last_profile_id: '',
  log_level: 'info',
  max_log_entries: 1000,
  auto_reconnect: true,
  reconnect_attempts: 3,
  warp_clean_ip: '162.159.192.1',
  warp_port: 2408,
  fragment_enabled: false,
  fragment_size: '100-200',
  fragment_sleep: '10-20',
  app_routing_mode: 'all',
  selected_apps: ['telegram.exe', 'discord.exe', 'chrome.exe'],
  auto_failover: true,
  subscription_auto_update_hours: 12,
  multi_hop_warp: false,
}

const mockLogs: LogEntry[] = [
  { timestamp: new Date().toISOString(), level: 'info', message: 'NexVPN Engine v1.4.0 initialized', source: 'engine' },
]

const mockBinaries: Record<string, BinaryInfo> = {
  xray: { name: 'Xray-core (VLESS, VMess, Reality)', filename: 'xray.exe', status: 'installed', path: 'C:\\bin\\xray.exe', download_url: '' },
  singbox: { name: 'Sing-box (WARP, Hy2, TUN)', filename: 'sing-box.exe', status: 'installed', path: 'C:\\bin\\sing-box.exe', download_url: '' },
  wireguard: { name: 'Wintun Virtual Adapter Driver', filename: 'wintun.dll', status: 'installed', path: 'C:\\bin\\wintun.dll', download_url: '' },
  openvpn: { name: 'OpenVPN Core Client', filename: 'openvpn.exe', status: 'installed', path: 'C:\\bin\\openvpn.exe', download_url: '' },
}

export function getMockData<T>(method: string, args: any[]): T {
  if (method === 'GetProfiles') return mockProfiles as unknown as T
  if (method === 'GetProfilesByCategory') {
    const cat = args[0]
    if (!cat) return mockProfiles as unknown as T
    return mockProfiles.filter(p => p.category === cat) as unknown as T
  }
  if (method === 'GetConnectionStatus') return mockStatus as unknown as T
  if (method === 'GetTrafficStats') return mockStats as unknown as T
  if (method === 'GetSettings') return mockSettings as unknown as T
  if (method === 'GetLogs') return mockLogs as unknown as T
  if (method === 'CheckCoreBinaries') return mockBinaries as unknown as T
  if (method === 'GetVersion') return '1.4.0' as unknown as T
  if (method === 'GetGroups' || method === 'GetGroupsByCategory') return ['Germany', 'Subscriptions', 'VIP-WARP'] as unknown as T
  if (method === 'PingProfile') return 65 as unknown as T
  if (method === 'PingAllProfiles') return mockProfiles as unknown as T
  if (method === 'RealPingActiveProfile') return 110 as unknown as T
  if (method === 'UpdateAllSubscriptions') return 3 as unknown as T
  if (method === 'LookupExternalIP') {
    return {
      ip: '185.220.101.5',
      country: 'Germany',
      country_code: 'DE',
      city: 'Frankfurt',
      isp: 'Hetzner Online GmbH',
    } as unknown as T
  }
  if (method === 'GenerateWarpProfile') {
    const p: Profile = {
      id: 'warp-' + Date.now(),
      name: 'Cloudflare WARP (162.159.192.1)',
      protocol: 'warp',
      category: 'wireguard_warp',
      config_content: '',
      server: '162.159.192.1',
      port: 2408,
      group: 'Cloudflare WARP',
      latency_ms: 42,
      real_delay_ms: 85,
      description: 'Auto-generated high-speed Cloudflare WARP tunnel',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      usage_count: 0,
    }
    return p as unknown as T
  }
  return undefined as unknown as T
}
