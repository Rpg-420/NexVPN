import type {
  Profile,
  ConnectionStatus,
  TrafficStats,
  Settings,
  LogEntry,
  BinaryInfo,
  IPInfo,
  DoctorReport,
  RepairResult,
  InstalledApp,
  RadarReport,
  EmergencyRepairReport,
  ConnectionSession,
  DNSLeakReport,
  ServiceUnblockResult,
  GamingOptimizationStatus,
  BackupSummary,
  DiagnosticReport,
  DiagnosticRepairResult,
  SNIPoolItem,
  QuotaAlertInfo,
  DNSScanResult,
  UsageReport,
  PsiphonOptions,
  MobileRemoteInfo,
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
    const isDev = Boolean((import.meta as any)?.env?.DEV)
    const maxAttempts = isDev ? 50 : 300 // 1.5s in dev, 9s in prod
    const check = () => {
      if (isWails()) {
        resolve(true)
        return
      }
      attempts++
      if (attempts > maxAttempts) {
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

  // Only return mock data in development mode
  const isDev = Boolean((import.meta as any)?.env?.DEV)
  if (isDev) {
    console.warn(`Wails not available for ${shortName}, using mock data.`)
    const { getMockData } = await import('./api.mock')
    return getMockData<T>(shortName, args)
  }
  
  // In production, missing binding is a bug - throw to surface it (do not silently hide with empty data)
  console.error(`Wails binding not found for ${shortName} in production!`)
  throw new Error(`Wails binding not available: ${shortName}. Try rebuilding backend.`)
}

export const api = {
  getProfiles: () => callGo<Profile[]>('GetProfiles'),
  getProfileById: (id: string) => callGo<Profile>('GetProfileByID', id),
  getProfilesByCategory: (category: string) => callGo<Profile[]>('GetProfilesByCategory', category),
  addProfile: (profile: Profile) => callGo<string>('AddProfile', profile),
  updateProfile: (profile: Profile) => callGo<void>('UpdateProfile', profile),
  deleteProfile: (id: string) => callGo<void>('DeleteProfile', id),
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
  getGroupsByCategory: (category: string) => callGo<string[]>('GetGroupsByCategory', category),
  checkCoreBinaries: () => callGo<Record<string, BinaryInfo>>('CheckCoreBinaries'),
  installCoreBinary: (key: string) => callGo<void>('InstallCoreBinary', key),
  updateCoreEngine: (key: string) => callGo<string>('UpdateCoreEngine', key),
  updateGeoData: () => callGo<string>('UpdateGeoData'),

  connect: (profileId: string) => callGo<void>('Connect', profileId),
  disconnect: () => callGo<void>('Disconnect'),
  getConnectionStatus: () => callGo<ConnectionStatus>('GetConnectionStatus'),
  getTrafficStats: () => callGo<TrafficStats>('GetTrafficStats'),
  getUsageReport: () => callGo<UsageReport>('GetUsageReport'),

  getSettings: () => callGo<Settings>('GetSettings'),
  saveSettings: (settings: Settings) => callGo<void>('SaveSettings', settings),
  setProxyMode: (mode: string) => callGo<void>('SetProxyMode', mode),

  getLogs: () => callGo<LogEntry[]>('GetLogs'),
  clearLogs: () => callGo<void>('ClearLogs'),
  /** Absolute path of the directory holding config.json / history.json. */
  getAppDataPath: () => callGo<string>('GetAppDataPath'),
  /** Reveals that directory in the OS file manager. */
  openProfilesFolder: () => callGo<void>('OpenProfilesFolder'),

  /**
   * Path to the on-disk launch diary (startup.log). Unlike the in-memory log
   * buffer this survives a crash or a hang before the window is drawn, and it is
   * what the standalone NexVPN-Report tool reads when the app will not start.
   * Empty string when the diary could not be opened.
   */
  getStartupDiaryPath: () => callGo<string>('GetStartupDiaryPath'),

  getVersion: () => callGo<string>('GetVersion'),
  /** Build/runtime metadata: app_version, go_version, platform, arch. */
  getSystemInfo: () => callGo<Record<string, string>>('GetSystemInfo'),
  checkForUpdates: () => callGo<{
    has_update: boolean
    latest_version: string
    current_version: string
    release_url: string
    release_notes: string
    error?: string
  }>('CheckForUpdates'),

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
  getLocalIPAddress: () => callGo<string>('GetLocalIPAddress'),
  toggleAllowLan: (enabled: boolean) => callGo<void>('ToggleAllowLan', enabled),
  runProtocolRadar: () => callGo<RadarReport>('RunProtocolRadar'),
  runEmergencyNetworkRepair: () => callGo<EmergencyRepairReport>('RunEmergencyNetworkRepair'),
  fetchLiveCleanIPFeed: () => callGo<Array<{ cidr: string; region: string; description: string }>>('FetchLiveCleanIPFeed'),
  exportEncryptedBackup: (password?: string) => callGo<string>('ExportEncryptedBackup', password || ''),
  importEncryptedBackup: (payload: string, password?: string, overwrite?: boolean) => callGo<BackupSummary>('ImportEncryptedBackup', payload, password || '', Boolean(overwrite)),
  exportFullBackup: () => callGo<string>('ExportFullBackup'),
  importFullBackup: (jsonStr: string, overwrite: boolean) => callGo<number>('ImportFullBackup', jsonStr, overwrite),
  getConnectionHistory: () => callGo<ConnectionSession[]>('GetConnectionHistory'),
  clearConnectionHistory: () => callGo<void>('ClearConnectionHistory'),
  runDNSLeakDiagnostics: () => callGo<DNSLeakReport>('RunDNSLeakDiagnostics'),
  sendSystemNotification: (title: string, message: string) => callGo<void>('SendSystemNotification', title, message),
  createProxyChain: (hopProfileIDs: string[], chainName: string) =>
    callGo<string>('CreateProxyChain', hopProfileIDs, chainName),
  startMobileRemote: (port?: number) => callGo<string>('StartMobileRemote', port || 10815),
  stopMobileRemote: () => callGo<void>('StopMobileRemote'),
  getMobileRemoteInfo: () => callGo<MobileRemoteInfo>('GetMobileRemoteInfo'),
  getProfileShareURI: (profileId: string) => callGo<string>('GetProfileShareURI', profileId),
  openBrowserURL: (url: string) => callGo<void>('OpenBrowserURL', url),
  panicWipe: () => callGo<void>('PanicWipe'),

  // Discovery Hub Methods
  fetchTelegramProxies: (customSources?: string[]) =>
    callGo<import('../types').TelegramProxy[]>('FetchTelegramProxies', customSources || []),
  testTelegramProxies: (proxies: import('../types').TelegramProxy[]) =>
    callGo<import('../types').TelegramProxy[]>('TestTelegramProxies', proxies),
  launchTelegramProxyLink: (link: string) => callGo<void>('LaunchTelegramProxyLink', link),

  fetchFreeConfigs: (customSources?: string[]) =>
    callGo<import('../types').FreeConfigItem[]>('FetchFreeConfigs', customSources || []),
  testFreeConfigs: (configs: import('../types').FreeConfigItem[]) =>
    callGo<import('../types').FreeConfigItem[]>('TestFreeConfigs', configs),
  importFreeConfigs: (configs: import('../types').FreeConfigItem[], groupName?: string) =>
    callGo<number>('ImportFreeConfigs', configs, groupName || 'Free Community'),

  // Public OpenVPN server catalogues (VPN Gate, ...). The .ovpn bodies stay in
  // the backend — only metadata crosses the bridge, and the import resolves the
  // config by server id.
  listOpenVPNSources: () =>
    callGo<import('../types').OpenVPNSourceInfo[]>('ListOpenVPNSources'),
  fetchOpenVPNServers: (sourceId?: string, mirrorUrl?: string) =>
    callGo<import('../types').OpenVPNServer[]>('FetchOpenVPNServers', sourceId || 'vpngate', mirrorUrl || ''),
  testOpenVPNServers: (servers: import('../types').OpenVPNServer[]) =>
    callGo<import('../types').OpenVPNServer[]>('TestOpenVPNServers', servers),
  importOpenVPNServers: (servers: import('../types').OpenVPNServer[], group?: string, username?: string, password?: string) =>
    callGo<number>('ImportOpenVPNServers', servers, group || 'VPN Gate', username || '', password || ''),

  // TunnelBear. The account password is written once and never read back — the
  // backend returns accounts without it, so there is no call that would let a
  // form repopulate the field.
  listTunnelBearCountries: () =>
    callGo<import('../types').TunnelBearCountry[]>('ListTunnelBearCountries'),
  listTunnelBearAccounts: () =>
    callGo<import('../types').TunnelBearAccount[]>('ListTunnelBearAccounts'),
  saveTunnelBearAccount: (id: string, username: string, password: string) =>
    callGo<string>('SaveTunnelBearAccount', id || '', username, password),
  deleteTunnelBearAccount: (id: string) =>
    callGo<void>('DeleteTunnelBearAccount', id),
  setActiveTunnelBearAccount: (id: string) =>
    callGo<void>('SetActiveTunnelBearAccount', id || ''),
  tunnelBearSecretBackend: () => callGo<string>('TunnelBearSecretBackend'),
  testTunnelBearCountry: (countryId: string) =>
    callGo<number>('TestTunnelBearCountry', countryId),
  importTunnelBearProfile: (countryId: string) =>
    callGo<string>('ImportTunnelBearProfile', countryId),

  // --- Diagnostics, history and maintenance -------------------------------

  /** Recent launch-diary lines. The diary records per-phase connect timings. */
  getDiaryEntries: (limit?: number) =>
    callGo<import('../types').DiaryEntry[]>('GetDiaryEntries', limit ?? 800),
  getConnectionStatistics: () =>
    callGo<import('../types').ConnectionStatistics>('GetConnectionStatistics'),
  /** Probes whether IPv6 traffic escapes the tunnel. Only meaningful when connected. */
  testIPv6Leak: () => callGo<import('../types').IPv6LeakResult>('TestIPv6Leak'),
  /** Compares the running version with the latest published release. Never installs. */
  checkAppUpdate: () => callGo<import('../types').UpdateInfo>('CheckAppUpdate'),
  hasPreviousConfigSnapshot: () => callGo<boolean>('HasPreviousConfigSnapshot'),
  restorePreviousConfig: () => callGo<void>('RestorePreviousConfig'),
  /** Writes a redacted zip of logs, diary and config. Returns "" when cancelled. */
  exportDiagnosticsBundle: () => callGo<string>('ExportDiagnosticsBundle'),
  getSubscriptionChanges: () =>
    callGo<import('../types').SubscriptionChange[]>('GetSubscriptionChanges'),
  clearSubscriptionChanges: () => callGo<void>('ClearSubscriptionChanges'),
  /** Pins or unpins a profile; pinned profiles lead the list and the tray menu. */
  setProfileFavorite: (id: string, favorite: boolean) =>
    callGo<void>('SetProfileFavorite', id, favorite),
  /** SSID of the joined Wi-Fi network, or "" on Ethernet or when unreadable. */
  currentNetworkSSID: () => callGo<string>('CurrentNetworkSSID'),

  /** Stops a bulk ping sweep that is in progress. Safe to call when idle. */
  cancelPingAll: () => callGo<void>('CancelPingAll'),

  startCloudflareDiscoveryScan: (mode: string, isp?: string, customSubnets?: string[], customPorts?: number[]) =>
    callGo<void>('StartCloudflareDiscoveryScan', mode, isp || 'all', customSubnets || [], customPorts || []),
  stopCloudflareDiscoveryScan: () => callGo<void>('StopCloudflareDiscoveryScan'),
  applyCleanIPToWARPProfiles: (ip: string, port: number) =>
    callGo<number>('ApplyCleanIPToWARPProfiles', ip, port),
  applyCleanIPToV2RayProfiles: (ip: string, port: number, onlyCDN?: boolean) =>
    callGo<number>('ApplyCleanIPToV2RayProfiles', ip, port, onlyCDN !== false),

  // Fragment Scanner Methods
  startFragmentDiscoveryScan: () => callGo<void>('StartFragmentDiscoveryScan'),
  stopFragmentDiscoveryScan: () => callGo<void>('StopFragmentDiscoveryScan'),
  applyOptimalFragment: (size: string, sleep: string) =>
    callGo<void>('ApplyOptimalFragment', size, sleep),

  // Config Converter & Batch Real Delay Methods
  convertConfig: (rawInput: string, targetFormat: string) =>
    callGo<string>('ConvertConfig', rawInput, targetFormat),
  testAllProfilesRealDelay: (category: string) =>
    callGo<import('../types').BatchTestResult[]>('TestAllProfilesRealDelay', category),
  toggleAutoFailover: (enabled: boolean) =>
    callGo<void>('ToggleAutoFailover', enabled),

  // Smart ISP, Sub Info & P2P Local Sync
  detectCurrentISP: () =>
    callGo<import('../types').ISPRecommendation>('DetectCurrentISP'),
  broadcastProfilesToLAN: (category: string, deviceName: string, pin?: string) =>
    callGo<number>('BroadcastProfilesToLAN', category, deviceName, pin || ''),
  listenForNearbyProfiles: (timeoutSeconds: number, pin?: string) =>
    callGo<any>('ListenForNearbyProfiles', timeoutSeconds, pin || ''),
  generateSyncPIN: () =>
    callGo<string>('GenerateSyncPIN'),
  importNearbyProfiles: (profiles: Profile[], groupName: string) =>
    callGo<number>('ImportNearbyProfiles', profiles, groupName),
  getSubscriptionInfo: (subUrl: string) =>
    callGo<import('../types').SubscriptionUserInfo>('GetSubscriptionInfo', subUrl),
  setTrayLanguage: (lang: string) =>
    callGo<void>('SetTrayLanguage', lang),

  // Native window controls
  showWindow: () => callGo<void>('ShowWindow'),
  hideWindow: () => callGo<void>('HideWindow'),
  setTitle: (title: string) => callGo<void>('SetTitle', title),

  // Psiphon Emergency Shield Methods
  getPsiphonRegions: () =>
    callGo<string[]>('GetPsiphonRegions'),
  getPsiphonOptions: () =>
    callGo<PsiphonOptions>('GetPsiphonOptions'),
  connectPsiphonWithOptions: (region: string, opts: PsiphonOptions) =>
    callGo<void>('ConnectPsiphonWithOptions', region, opts),

  // Service Unblock Matrix & AI / Streaming Checker
  checkServiceUnblock: () =>
    callGo<ServiceUnblockResult[]>('CheckServiceUnblock'),

  // Ultra-Low Jitter Windows Gaming Optimizations
  applyGamingOptimizations: () =>
    callGo<GamingOptimizationStatus>('ApplyGamingOptimizations'),
  revertGamingOptimizations: () =>
    callGo<GamingOptimizationStatus>('RevertGamingOptimizations'),
  getGamingOptimizationStatus: () =>
    callGo<GamingOptimizationStatus>('GetGamingOptimizationStatus'),

  // 1-Click Diagnostics & Network Recovery
  runDiagnosticAudit: () =>
    callGo<DiagnosticReport>('RunDiagnosticAudit'),
  repairDiagnosticIssues: () =>
    callGo<DiagnosticRepairResult>('RepairDiagnosticIssues'),
  flushWindowsDNS: () =>
    callGo<void>('FlushWindowsDNS'),
  scanAndSelectBestDNS: (useDoH: boolean) =>
    callGo<DNSScanResult[]>('ScanAndSelectBestDNS', useDoH),

  // Dynamic SNI Fallback Pool & REALITY Optimizer
  getSNIPoolList: () =>
    callGo<SNIPoolItem[]>('GetSNIPoolList'),
  auditSNIPool: () =>
    callGo<SNIPoolItem[]>('AuditSNIPool'),
  addCustomSNI: (domain: string) =>
    callGo<void>('AddCustomSNI', domain),
  removeCustomSNI: (domain: string) =>
    callGo<void>('RemoveCustomSNI', domain),
  optimizeProfileSNI: (profileId: string) =>
    callGo<Profile>('OptimizeProfileSNI', profileId),

  // Adaptive MTU & MSS Clamping
  runAutoMTUOptimization: (targetHost?: string) =>
    callGo<number>('RunAutoMTUOptimization', targetHost || '1.1.1.1'),

  // Subscription Quota & Background Alerts
  checkSubscriptionQuotas: (subUrl: string) =>
    callGo<QuotaAlertInfo>('CheckSubscriptionQuotas', subUrl),

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
}

