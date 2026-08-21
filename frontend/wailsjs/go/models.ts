export namespace config {
	
	export class ConnectionSession {
	    id: string;
	    profile_id: string;
	    profile_name: string;
	    protocol: string;
	    server: string;
	    // Go type: time
	    connected_at: any;
	    // Go type: time
	    disconnected_at: any;
	    duration_sec: number;
	    bytes_received: number;
	    bytes_sent: number;
	    quality_score: number;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.profile_id = source["profile_id"];
	        this.profile_name = source["profile_name"];
	        this.protocol = source["protocol"];
	        this.server = source["server"];
	        this.connected_at = this.convertValues(source["connected_at"], null);
	        this.disconnected_at = this.convertValues(source["disconnected_at"], null);
	        this.duration_sec = source["duration_sec"];
	        this.bytes_received = source["bytes_received"];
	        this.bytes_sent = source["bytes_sent"];
	        this.quality_score = source["quality_score"];
	        this.reason = source["reason"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Profile {
	    id: string;
	    name: string;
	    protocol: string;
	    category: string;
	    config_content: string;
	    raw_uri?: string;
	    server?: string;
	    port?: number;
	    group?: string;
	    country?: string;
	    country_code?: string;
	    latency_ms?: number;
	    real_delay_ms?: number;
	    subscription_url?: string;
	    subscription_name?: string;
	    description?: string;
	    tags?: string[];
	    username?: string;
	    password?: string;
	    warp_endpoint?: string;
	    warp_clean_ip?: string;
	    warp_reserved?: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	    // Go type: time
	    last_used?: any;
	    usage_count: number;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.protocol = source["protocol"];
	        this.category = source["category"];
	        this.config_content = source["config_content"];
	        this.raw_uri = source["raw_uri"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.group = source["group"];
	        this.country = source["country"];
	        this.country_code = source["country_code"];
	        this.latency_ms = source["latency_ms"];
	        this.real_delay_ms = source["real_delay_ms"];
	        this.subscription_url = source["subscription_url"];
	        this.subscription_name = source["subscription_name"];
	        this.description = source["description"];
	        this.tags = source["tags"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.warp_endpoint = source["warp_endpoint"];
	        this.warp_clean_ip = source["warp_clean_ip"];
	        this.warp_reserved = source["warp_reserved"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.last_used = this.convertValues(source["last_used"], null);
	        this.usage_count = source["usage_count"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Settings {
	    auto_connect: boolean;
	    start_minimized: boolean;
	    start_on_boot: boolean;
	    kill_switch_enabled: boolean;
	    dns_server: string;
	    dns_fallback: string;
	    language: string;
	    theme: string;
	    routing_mode: string;
	    proxy_mode: string;
	    real_delay_url: string;
	    last_profile_id?: string;
	    log_level: string;
	    max_log_entries: number;
	    auto_reconnect: boolean;
	    reconnect_attempts: number;
	    warp_clean_ip?: string;
	    warp_port?: number;
	    warp_license_key?: string;
	    warp_account_type?: string;
	    fragment_enabled: boolean;
	    fragment_size: string;
	    fragment_sleep: string;
	    app_routing_mode: string;
	    selected_apps: string[];
	    auto_failover: boolean;
	    auto_failover_enabled: boolean;
	    subscription_auto_update_hours: number;
	    multi_hop_warp: boolean;
	    ip_strategy: string;
	    block_ads: boolean;
	    bypass_lan: boolean;
	    custom_direct_domains: string[];
	    custom_proxy_domains: string[];
	    custom_block_domains: string[];
	    amnezia_jc: number;
	    amnezia_jmin: number;
	    amnezia_jmax: number;
	    amnezia_s1: number;
	    amnezia_s2: number;
	    amnezia_h1: number;
	    amnezia_h2: number;
	    amnezia_h3: number;
	    amnezia_h4: number;
	    show_mini_widget: boolean;
	    v2ray_core: string;
	    allow_lan: boolean;
	    game_mode_enabled: boolean;
	    sound_effects_enabled: boolean;
	    monthly_quota_gb: number;
	    quota_alert_percentage: number;
	    stealth_mode_enabled: boolean;
	    privacy_shield_enabled: boolean;
	    notifications_enabled: boolean;
	    custom_doh_url?: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.auto_connect = source["auto_connect"];
	        this.start_minimized = source["start_minimized"];
	        this.start_on_boot = source["start_on_boot"];
	        this.kill_switch_enabled = source["kill_switch_enabled"];
	        this.dns_server = source["dns_server"];
	        this.dns_fallback = source["dns_fallback"];
	        this.language = source["language"];
	        this.theme = source["theme"];
	        this.routing_mode = source["routing_mode"];
	        this.proxy_mode = source["proxy_mode"];
	        this.real_delay_url = source["real_delay_url"];
	        this.last_profile_id = source["last_profile_id"];
	        this.log_level = source["log_level"];
	        this.max_log_entries = source["max_log_entries"];
	        this.auto_reconnect = source["auto_reconnect"];
	        this.reconnect_attempts = source["reconnect_attempts"];
	        this.warp_clean_ip = source["warp_clean_ip"];
	        this.warp_port = source["warp_port"];
	        this.warp_license_key = source["warp_license_key"];
	        this.warp_account_type = source["warp_account_type"];
	        this.fragment_enabled = source["fragment_enabled"];
	        this.fragment_size = source["fragment_size"];
	        this.fragment_sleep = source["fragment_sleep"];
	        this.app_routing_mode = source["app_routing_mode"];
	        this.selected_apps = source["selected_apps"];
	        this.auto_failover = source["auto_failover"];
	        this.auto_failover_enabled = source["auto_failover_enabled"];
	        this.subscription_auto_update_hours = source["subscription_auto_update_hours"];
	        this.multi_hop_warp = source["multi_hop_warp"];
	        this.ip_strategy = source["ip_strategy"];
	        this.block_ads = source["block_ads"];
	        this.bypass_lan = source["bypass_lan"];
	        this.custom_direct_domains = source["custom_direct_domains"];
	        this.custom_proxy_domains = source["custom_proxy_domains"];
	        this.custom_block_domains = source["custom_block_domains"];
	        this.amnezia_jc = source["amnezia_jc"];
	        this.amnezia_jmin = source["amnezia_jmin"];
	        this.amnezia_jmax = source["amnezia_jmax"];
	        this.amnezia_s1 = source["amnezia_s1"];
	        this.amnezia_s2 = source["amnezia_s2"];
	        this.amnezia_h1 = source["amnezia_h1"];
	        this.amnezia_h2 = source["amnezia_h2"];
	        this.amnezia_h3 = source["amnezia_h3"];
	        this.amnezia_h4 = source["amnezia_h4"];
	        this.show_mini_widget = source["show_mini_widget"];
	        this.v2ray_core = source["v2ray_core"];
	        this.allow_lan = source["allow_lan"];
	        this.game_mode_enabled = source["game_mode_enabled"];
	        this.sound_effects_enabled = source["sound_effects_enabled"];
	        this.monthly_quota_gb = source["monthly_quota_gb"];
	        this.quota_alert_percentage = source["quota_alert_percentage"];
	        this.stealth_mode_enabled = source["stealth_mode_enabled"];
	        this.privacy_shield_enabled = source["privacy_shield_enabled"];
	        this.notifications_enabled = source["notifications_enabled"];
	        this.custom_doh_url = source["custom_doh_url"];
	    }
	}

}

export namespace core {
	
	export class BatchTestResult {
	    profile_id: string;
	    profile_name: string;
	    latency_ms: number;
	    success: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BatchTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile_id = source["profile_id"];
	        this.profile_name = source["profile_name"];
	        this.latency_ms = source["latency_ms"];
	        this.success = source["success"];
	    }
	}
	export class ConnectionStatus {
	    state: string;
	    profile_id?: string;
	    profile_name?: string;
	    protocol?: string;
	    // Go type: time
	    connected_at?: any;
	    server?: string;
	    error_msg?: string;
	    duration?: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.profile_id = source["profile_id"];
	        this.profile_name = source["profile_name"];
	        this.protocol = source["protocol"];
	        this.connected_at = this.convertValues(source["connected_at"], null);
	        this.server = source["server"];
	        this.error_msg = source["error_msg"];
	        this.duration = source["duration"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DNSQueryProbe {
	    name: string;
	    target: string;
	    resolved_ip: string;
	    status: string;
	    latency_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new DNSQueryProbe(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.target = source["target"];
	        this.resolved_ip = source["resolved_ip"];
	        this.status = source["status"];
	        this.latency_ms = source["latency_ms"];
	    }
	}
	export class DNSLeakReport {
	    is_leaking: boolean;
	    outbound_ip: string;
	    outbound_isp: string;
	    outbound_country: string;
	    dns_resolver_ip: string;
	    dns_resolver_isp: string;
	    dns_resolver_country: string;
	    risk_level: string;
	    probes: DNSQueryProbe[];
	    summary: string;
	    // Go type: time
	    checked_at: any;
	
	    static createFrom(source: any = {}) {
	        return new DNSLeakReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_leaking = source["is_leaking"];
	        this.outbound_ip = source["outbound_ip"];
	        this.outbound_isp = source["outbound_isp"];
	        this.outbound_country = source["outbound_country"];
	        this.dns_resolver_ip = source["dns_resolver_ip"];
	        this.dns_resolver_isp = source["dns_resolver_isp"];
	        this.dns_resolver_country = source["dns_resolver_country"];
	        this.risk_level = source["risk_level"];
	        this.probes = this.convertValues(source["probes"], DNSQueryProbe);
	        this.summary = source["summary"];
	        this.checked_at = this.convertValues(source["checked_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class LogEntry {
	    // Go type: time
	    timestamp: any;
	    level: string;
	    message: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.level = source["level"];
	        this.message = source["message"];
	        this.source = source["source"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TrafficStats {
	    bytes_received: number;
	    bytes_sent: number;
	    speed_download: number;
	    speed_upload: number;
	    packets_received: number;
	    packets_sent: number;
	
	    static createFrom(source: any = {}) {
	        return new TrafficStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bytes_received = source["bytes_received"];
	        this.bytes_sent = source["bytes_sent"];
	        this.speed_download = source["speed_download"];
	        this.speed_upload = source["speed_upload"];
	        this.packets_received = source["packets_received"];
	        this.packets_sent = source["packets_sent"];
	    }
	}

}

export namespace discovery {
	
	export class FreeConfigItem {
	    id: string;
	    name: string;
	    protocol: string;
	    server: string;
	    port: number;
	    raw_uri: string;
	    latency_ms: number;
	    is_alive: boolean;
	    country?: string;
	    country_code?: string;
	    source_name?: string;
	    network?: string;
	    security?: string;
	
	    static createFrom(source: any = {}) {
	        return new FreeConfigItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.protocol = source["protocol"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.raw_uri = source["raw_uri"];
	        this.latency_ms = source["latency_ms"];
	        this.is_alive = source["is_alive"];
	        this.country = source["country"];
	        this.country_code = source["country_code"];
	        this.source_name = source["source_name"];
	        this.network = source["network"];
	        this.security = source["security"];
	    }
	}
	export class ISPRecommendation {
	    isp_name: string;
	    isp_name_en: string;
	    isp_category: string;
	    ip: string;
	    country: string;
	    country_code: string;
	    asn: string;
	    recommended_protocol: string;
	    recommended_fragment: string;
	    recommended_dns: string;
	    status_score: number;
	    advice: string;
	    advice_en: string;
	
	    static createFrom(source: any = {}) {
	        return new ISPRecommendation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isp_name = source["isp_name"];
	        this.isp_name_en = source["isp_name_en"];
	        this.isp_category = source["isp_category"];
	        this.ip = source["ip"];
	        this.country = source["country"];
	        this.country_code = source["country_code"];
	        this.asn = source["asn"];
	        this.recommended_protocol = source["recommended_protocol"];
	        this.recommended_fragment = source["recommended_fragment"];
	        this.recommended_dns = source["recommended_dns"];
	        this.status_score = source["status_score"];
	        this.advice = source["advice"];
	        this.advice_en = source["advice_en"];
	    }
	}
	export class SyncPayload {
	    device_name: string;
	    timestamp: number;
	    profiles: config.Profile[];
	
	    static createFrom(source: any = {}) {
	        return new SyncPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device_name = source["device_name"];
	        this.timestamp = source["timestamp"];
	        this.profiles = this.convertValues(source["profiles"], config.Profile);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TelegramProxy {
	    id: string;
	    type: string;
	    server: string;
	    port: number;
	    secret?: string;
	    username?: string;
	    password?: string;
	    latency_ms: number;
	    is_alive: boolean;
	    country?: string;
	    country_code?: string;
	    link: string;
	    source?: string;
	
	    static createFrom(source: any = {}) {
	        return new TelegramProxy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.secret = source["secret"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.latency_ms = source["latency_ms"];
	        this.is_alive = source["is_alive"];
	        this.country = source["country"];
	        this.country_code = source["country_code"];
	        this.link = source["link"];
	        this.source = source["source"];
	    }
	}

}

export namespace main {
	
	export class BatchIPRequest {
	    ip: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new BatchIPRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.port = source["port"];
	    }
	}
	export class CleanIPRange {
	    cidr: string;
	    region: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new CleanIPRange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cidr = source["cidr"];
	        this.region = source["region"];
	        this.description = source["description"];
	    }
	}
	export class RepairStep {
	    name: string;
	    command: string;
	    status: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new RepairStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.command = source["command"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class EmergencyRepairReport {
	    overall_status: string;
	    steps: RepairStep[];
	    timestamp: string;
	
	    static createFrom(source: any = {}) {
	        return new EmergencyRepairReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.overall_status = source["overall_status"];
	        this.steps = this.convertValues(source["steps"], RepairStep);
	        this.timestamp = source["timestamp"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MobileRemoteInfo {
	    running: boolean;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new MobileRemoteInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.url = source["url"];
	    }
	}
	export class RadarProtocolResult {
	    name: string;
	    protocol: string;
	    status: string;
	    latency_ms: number;
	    packet_loss: number;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new RadarProtocolResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.protocol = source["protocol"];
	        this.status = source["status"];
	        this.latency_ms = source["latency_ms"];
	        this.packet_loss = source["packet_loss"];
	        this.description = source["description"];
	    }
	}
	export class RadarReport {
	    isp_name: string;
	    ip_address: string;
	    censorship_level: string;
	    best_protocol: string;
	    recommended_action: string;
	    protocols: RadarProtocolResult[];
	    timestamp: string;
	
	    static createFrom(source: any = {}) {
	        return new RadarReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isp_name = source["isp_name"];
	        this.ip_address = source["ip_address"];
	        this.censorship_level = source["censorship_level"];
	        this.best_protocol = source["best_protocol"];
	        this.recommended_action = source["recommended_action"];
	        this.protocols = this.convertValues(source["protocols"], RadarProtocolResult);
	        this.timestamp = source["timestamp"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class UpdateCheckResult {
	    has_update: boolean;
	    latest_version: string;
	    current_version: string;
	    release_url: string;
	    release_notes: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.has_update = source["has_update"];
	        this.latest_version = source["latest_version"];
	        this.current_version = source["current_version"];
	        this.release_url = source["release_url"];
	        this.release_notes = source["release_notes"];
	        this.error = source["error"];
	    }
	}

}

export namespace parser {
	
	export class DiagnosticItem {
	    id: string;
	    title: string;
	    description: string;
	    status: string;
	    latency_ms: number;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.latency_ms = source["latency_ms"];
	        this.detail = source["detail"];
	    }
	}
	export class DoctorReport {
	    overall_status: string;
	    summary: string;
	    items: DiagnosticItem[];
	    timestamp: number;
	
	    static createFrom(source: any = {}) {
	        return new DoctorReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.overall_status = source["overall_status"];
	        this.summary = source["summary"];
	        this.items = this.convertValues(source["items"], DiagnosticItem);
	        this.timestamp = source["timestamp"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class IPInfo {
	    ip: string;
	    country: string;
	    country_code: string;
	    city: string;
	    isp: string;
	
	    static createFrom(source: any = {}) {
	        return new IPInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.country = source["country"];
	        this.country_code = source["country_code"];
	        this.city = source["city"];
	        this.isp = source["isp"];
	    }
	}
	export class RepairResult {
	    success: boolean;
	    message: string;
	    actions: string[];
	
	    static createFrom(source: any = {}) {
	        return new RepairResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.actions = source["actions"];
	    }
	}
	export class SubscriptionUserInfo {
	    upload_bytes: number;
	    download_bytes: number;
	    total_bytes: number;
	    used_bytes: number;
	    expire_date: number;
	    remaining_days: number;
	    percent_used: number;
	
	    static createFrom(source: any = {}) {
	        return new SubscriptionUserInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.upload_bytes = source["upload_bytes"];
	        this.download_bytes = source["download_bytes"];
	        this.total_bytes = source["total_bytes"];
	        this.used_bytes = source["used_bytes"];
	        this.expire_date = source["expire_date"];
	        this.remaining_days = source["remaining_days"];
	        this.percent_used = source["percent_used"];
	    }
	}
	export class SubscriptionFetchResult {
	    content: string;
	    user_info?: SubscriptionUserInfo;
	
	    static createFrom(source: any = {}) {
	        return new SubscriptionFetchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.content = source["content"];
	        this.user_info = this.convertValues(source["user_info"], SubscriptionUserInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace sysproxy {
	
	export class InstalledApp {
	    id: string;
	    name: string;
	    exe_name: string;
	    category: string;
	    is_running: boolean;
	    icon_name: string;
	
	    static createFrom(source: any = {}) {
	        return new InstalledApp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.exe_name = source["exe_name"];
	        this.category = source["category"];
	        this.is_running = source["is_running"];
	        this.icon_name = source["icon_name"];
	    }
	}

}

