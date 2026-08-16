import type {
  Profile,
  ConnectionStatus,
  TrafficStats,
  Settings,
  LogEntry,
  BinaryInfo,
  IPInfo,
  CleanIPResult,
  ScanProgress,
  SpeedTestProgress,
  DoctorReport,
  RepairResult,
  InstalledApp,
  SubscriptionFetchResult,
  RadarReport,
  EmergencyRepairReport,
} from '../types'

const isWails = () => {
  return (
    typeof window !== 'undefined' &&
    typeof (window as any).go !== 'undefined' &&
    typeof (window as any).go.main !== 'undefined' &&
    typeof (window as any).go.main.App !== 'undefined'
  )
}

let wailsReadyPromise: Promise<boolean> | null = null

function waitForWails(): Promise<boolean> {
  if (isWails()) return Promise.resolve(true)
  if (wailsReadyPromise) return wailsReadyPromise

  wailsReadyPromise = new Promise<boolean>(resolve => {
    let attempts = 0
    const check = () => {
      if (isWails()) {
        resolve(true)
        return
      }
      attempts++
      if (attempts > 50) {
        resolve(false)
        return
      }
      setTimeout(check, 30)
    }
    check()
  })

  return wailsReadyPromise
}

async function callGo<T>(methodName: string, ...args: any[]): Promise<T> {
  const shortName = methodName.replace('main.App.', '')
  const ready = await waitForWails()
  if (ready && isWails()) {
    const fn = (window as any).go.main.App[shortName]
    if (typeof fn === 'function') {
      try {
        return await fn(...args)
      } catch (err) {
        console.error(`Wails call error [${shortName}]:`, err)
        throw err
      }
    }
  }

  return getMockData<T>(shortName, args)
}

function getMockData<T>(method: string, args: any[]): T {
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
  if (method === 'GetVersion') return '1.3.1' as unknown as T
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
  { timestamp: new Date().toISOString(), level: 'info', message: 'NexVPN Engine v1.3.1 initialized', source: 'engine' },
]

const mockBinaries: Record<string, BinaryInfo> = {
  xray: { name: 'Xray-core (VLESS, VMess, Reality)', filename: 'xray.exe', status: 'installed', path: 'C:\\bin\\xray.exe', download_url: '' },
  singbox: { name: 'Sing-box (WARP, Hy2, TUN)', filename: 'sing-box.exe', status: 'installed', path: 'C:\\bin\\sing-box.exe', download_url: '' },
  wireguard: { name: 'Wintun Virtual Adapter Driver', filename: 'wintun.dll', status: 'installed', path: 'C:\\bin\\wintun.dll', download_url: '' },
  openvpn: { name: 'OpenVPN Core Client', filename: 'openvpn.exe', status: 'installed', path: 'C:\\bin\\openvpn.exe', download_url: '' },
}

export const api = {
  getProfiles: () => callGo<Profile[]>('GetProfiles'),
  getProfilesByCategory: (category: string) => callGo<Profile[]>('GetProfilesByCategory', category),
  addProfile: (profile: Profile) => callGo<string>('AddProfile', profile),
  updateProfile: (profile: Profile) => callGo<void>('UpdateProfile', profile),
  deleteProfile: (id: string) => callGo<void>('DeleteProfile', id),
  deleteAllProfiles: () => callGo<void>('DeleteAllProfiles'),
  deleteProfilesByCategory: (category: string) => callGo<void>('DeleteProfilesByCategory', category),
  
  importProfile: (name: string, protocol: string, content: string, group?: string, category?: string) =>
    callGo<string>('ImportProfile', name, protocol, content, group || '', category || ''),
  importURI: (uri: string, group?: string) =>
    callGo<string>('ImportURI', uri, group || ''),
  importSubscription: (url: string, group?: string) =>
    callGo<number>('ImportSubscription', url, group || ''),
  updateAllSubscriptions: () => callGo<number>('UpdateAllSubscriptions'),
  updateSubscriptionGroup: (groupName: string) => callGo<number>('UpdateSubscriptionGroup', groupName),
  deleteGroup: (groupName: string) => callGo<void>('DeleteGroup', groupName),
  renameGroup: (oldName: string, newName: string) => callGo<void>('RenameGroup', oldName, newName),

  generateWarpProfile: (cleanIP?: string, port?: number) =>
    callGo<Profile>('GenerateWarpProfile', cleanIP || '', port || 0),
  generateWarpOnWarpProfile: (cleanIP?: string, port?: number, egressIP?: string, egressPort?: number) =>
    callGo<Profile>('GenerateWarpOnWarpProfile', cleanIP || '', port || 0, egressIP || '', egressPort || 0),
  deduplicateProfiles: (category?: string) => callGo<number>('DeduplicateProfiles', category || ''),
  removeDeadProfiles: (category?: string) => callGo<number>('RemoveDeadProfiles', category || ''),
  exportAllProfiles: (category?: string) => callGo<string>('ExportAllProfiles', category || ''),
  openTelegramProxy: () => callGo<void>('OpenTelegramProxy'),
  importOpenVPNProfile: (name: string, ovpnContent: string, username?: string, password?: string, group?: string) =>
    callGo<string>('ImportOpenVPNProfile', name, ovpnContent, username || '', password || '', group || ''),
  importWireGuardProfile: (name: string, confContent: string, group?: string) =>
    callGo<string>('ImportWireGuardProfile', name, confContent, group || ''),

  pingProfile: (id: string) => callGo<number>('PingProfile', id),
  pingAllProfiles: (category?: string) => callGo<Profile[]>('PingAllProfiles', category || ''),
  realPingActiveProfile: () => callGo<number>('RealPingActiveProfile'),
  lookupExternalIP: () => callGo<IPInfo>('LookupExternalIP'),
  clearStaleProxy: () => callGo<void>('ClearStaleProxy'),
  getGroups: () => callGo<string[]>('GetGroups'),
  getGroupsByCategory: (category: string) => callGo<string[]>('GetGroupsByCategory', category),
  checkCoreBinaries: () => callGo<Record<string, BinaryInfo>>('CheckCoreBinaries'),
  installCoreBinary: (key: string) => callGo<void>('InstallCoreBinary', key),
  updateCoreEngine: (key: string) => callGo<string>('UpdateCoreEngine', key),
  updateGeoData: () => callGo<string>('UpdateGeoData'),

  connect: (profileId: string) => callGo<void>('Connect', profileId),
  disconnect: () => callGo<void>('Disconnect'),
  getConnectionStatus: () => callGo<ConnectionStatus>('GetConnectionStatus'),
  getTrafficStats: () => callGo<TrafficStats>('GetTrafficStats'),

  getSettings: () => callGo<Settings>('GetSettings'),
  saveSettings: (settings: Settings) => callGo<void>('SaveSettings', settings),
  setProxyMode: (mode: string) => callGo<void>('SetProxyMode', mode),

  getLogs: () => callGo<LogEntry[]>('GetLogs'),
  clearLogs: () => callGo<void>('ClearLogs'),

  getVersion: () => callGo<string>('GetVersion'),

  // Clean IP Scanner & SpeedTest
  startCleanIPScan: (subnets?: string[], ports?: number[]) => callGo<void>('StartCleanIPScan', subnets || [], ports || []),
  stopCleanIPScan: () => callGo<void>('StopCleanIPScan'),
  batchGenerateWarpProfiles: (targets: Array<{ ip: string; port: number }>, useAmnezia: boolean) =>
    callGo<Profile[]>('BatchGenerateWarpProfiles', targets, useAmnezia),
  startRealSpeedTest: () => callGo<void>('StartRealSpeedTest'),
  stopRealSpeedTest: () => callGo<void>('StopRealSpeedTest'),
  applyWarpLicenseKey: (key: string) => callGo<void>('ApplyWarpLicenseKey', key),

  // Smart Engine & Diagnostics
  connectFastest: (category?: string) => callGo<Profile>('ConnectFastest', category || 'all'),
  runNetworkDiagnostics: () => callGo<DoctorReport>('RunNetworkDiagnostics'),
  fixNetworkIssues: () => callGo<RepairResult>('FixNetworkIssues'),
  getInstalledApplications: () => callGo<InstalledApp[]>('GetInstalledApplications'),
  saveAppRouting: (mode: string, apps: string[]) => callGo<void>('SaveAppRouting', mode, apps),
  fetchSubscriptionInfo: (url: string) => callGo<SubscriptionFetchResult>('FetchSubscriptionInfo', url),
  getLocalIPAddress: () => callGo<string>('GetLocalIPAddress'),
  toggleAllowLan: (enabled: boolean) => callGo<void>('ToggleAllowLan', enabled),
  runProtocolRadar: () => callGo<RadarReport>('RunProtocolRadar'),
  runEmergencyNetworkRepair: () => callGo<EmergencyRepairReport>('RunEmergencyNetworkRepair'),
  fetchLiveCleanIPFeed: () => callGo<Array<{ cidr: string; region: string; description: string }>>('FetchLiveCleanIPFeed'),
  exportEncryptedBackup: (password: string) => callGo<string>('ExportEncryptedBackup', password),
  importEncryptedBackup: (payload: string, password: string) => callGo<void>('ImportEncryptedBackup', payload, password),
  exportFullBackup: () => callGo<string>('ExportFullBackup'),
  importFullBackup: (jsonStr: string, overwrite: boolean) => callGo<number>('ImportFullBackup', jsonStr, overwrite),
  getConnectionHistory: () => callGo<any[]>('GetConnectionHistory'),
  clearConnectionHistory: () => callGo<void>('ClearConnectionHistory'),
  testAllProfilesBatch: (category?: string) => callGo<any[]>('TestAllProfilesBatch', category || 'all'),
  runDNSLeakDiagnostics: () => callGo<any>('RunDNSLeakDiagnostics'),
  sendSystemNotification: (title: string, message: string) => callGo<void>('SendSystemNotification', title, message),
  createProxyChain: (hopProfileIDs: string[], chainName: string) =>
    callGo<string>('CreateProxyChain', hopProfileIDs, chainName),
  startMobileRemote: (port?: number) => callGo<string>('StartMobileRemote', port || 10815),
  stopMobileRemote: () => callGo<void>('StopMobileRemote'),
  getMobileRemoteInfo: () => callGo<{ running: boolean; url: string }>('GetMobileRemoteInfo'),
  openBrowserURL: (url: string) => callGo<void>('OpenBrowserURL', url),
  panicWipe: () => callGo<void>('PanicWipe'),
  updateGeoDatabase: () => callGo<Record<string, string>>('UpdateGeoDatabase'),
  generateShareableSubscription: (category?: string) =>
    callGo<string>('GenerateShareableSubscription', category || 'all'),

  // Events listener helper
  onEvent: (eventName: string, callback: (...data: any[]) => void): (() => void) => {
    if (typeof window !== 'undefined' && (window as any).runtime && (window as any).runtime.EventsOn) {
      const cancelFn = (window as any).runtime.EventsOn(eventName, callback)
      return () => {
        if (typeof cancelFn === 'function') {
          cancelFn()
        } else if ((window as any).runtime.EventsOff) {
          (window as any).runtime.EventsOff(eventName)
        }
      }
    }
    return () => {}
  },
  offEvent: (eventName: string, ...additionalArgs: any[]) => {
    if (typeof window !== 'undefined' && (window as any).runtime && (window as any).runtime.EventsOff) {
      (window as any).runtime.EventsOff(eventName, ...additionalArgs)
    }
  },
}

