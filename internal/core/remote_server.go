package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"nexvpn/internal/config"
)

type RemoteServer struct {
	mu         sync.RWMutex
	httpServer *http.Server
	port       int
	running    bool
	engine     *Engine
	store      *config.Store
}

var globalRemoteServer *RemoteServer

func GetRemoteServer(engine *Engine, store *config.Store) *RemoteServer {
	if globalRemoteServer == nil {
		globalRemoteServer = &RemoteServer{
			port:   10815,
			engine: engine,
			store:  store,
		}
	}
	return globalRemoteServer
}

func (rs *RemoteServer) Start(port int) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.running {
		return nil
	}
	if port <= 0 {
		port = 10815
	}
	rs.port = port

	mux := http.NewServeMux()

	// API Endpoints
	mux.HandleFunc("/api/status", rs.handleStatus)
	mux.HandleFunc("/api/connect", rs.handleConnect)
	mux.HandleFunc("/api/disconnect", rs.handleDisconnect)

	// Responsive Web Mobile UI
	mux.HandleFunc("/", rs.handleWebUI)

	server := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", port),
		Handler: mux,
	}

	rs.httpServer = server
	rs.running = true

	go func() {
		_ = server.ListenAndServe()
	}()

	return nil
}

func (rs *RemoteServer) Stop() {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = rs.httpServer.Shutdown(ctx)
		rs.httpServer = nil
	}
	rs.running = false
}

func (rs *RemoteServer) IsRunning() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.running
}

func (rs *RemoteServer) GetURL() string {
	lanIP := getLocalLANIP()
	return fmt.Sprintf("http://%s:%d", lanIP, rs.port)
}

func getLocalLANIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ipStr := ipNet.IP.String()
				if strings.HasPrefix(ipStr, "192.168.") || strings.HasPrefix(ipStr, "10.") || strings.HasPrefix(ipStr, "172.") {
					return ipStr
				}
			}
		}
	}
	return "127.0.0.1"
}

func (rs *RemoteServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	status := rs.engine.GetStatus()
	stats := rs.engine.GetStats()
	rawProfiles := rs.store.GetProfiles()

	// Sanitize profiles to prevent credential and private key leakage over LAN
	type RemoteProfileSummary struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Protocol  string `json:"protocol"`
		Category  string `json:"category"`
		Group     string `json:"group"`
		LatencyMs int    `json:"latency_ms"`
	}

	sanitizedProfiles := make([]RemoteProfileSummary, 0, len(rawProfiles))
	for _, p := range rawProfiles {
		sanitizedProfiles = append(sanitizedProfiles, RemoteProfileSummary{
			ID:        p.ID,
			Name:      p.Name,
			Protocol:  string(p.Protocol),
			Category:  string(p.Category),
			Group:     p.Group,
			LatencyMs: p.LatencyMs,
		})
	}

	resp := map[string]interface{}{
		"status":   status,
		"stats":    stats,
		"profiles": sanitizedProfiles,
		"time":     time.Now().Unix(),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (rs *RemoteServer) handleConnect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ProfileID string `json:"profile_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProfileID == "" {
		http.Error(w, "Missing profile_id", http.StatusBadRequest)
		return
	}

	profile, err := rs.store.GetProfile(req.ProfileID)
	if err != nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	settings := rs.store.GetSettings()
	err = rs.engine.Connect(profile, settings)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "profile": profile.Name})
}

func (rs *RemoteServer) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	err := rs.engine.Disconnect()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func (rs *RemoteServer) handleWebUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
<title>NexVPN Mobile Controller</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
  body { background: #0a0e17; color: #e2e8f0; min-height: 100vh; display: flex; flex-direction: column; padding: 20px; }
  .card { background: #131b2e; border: 1px solid #1e293b; border-radius: 24px; padding: 24px; margin-bottom: 16px; text-align: center; box-shadow: 0 10px 30px rgba(0,0,0,0.5); }
  .title { font-size: 20px; font-weight: 800; color: #38bdf8; margin-bottom: 4px; display: flex; align-items: center; justify-content: center; gap: 8px; }
  .subtitle { font-size: 12px; color: #94a3b8; margin-bottom: 24px; }
  .orb { width: 140px; height: 140px; border-radius: 50%; margin: 0 auto 20px; display: flex; align-items: center; justify-content: center; font-size: 16px; font-weight: bold; border: 3px solid #334155; transition: all 0.3s; cursor: pointer; }
  .orb.connected { background: radial-gradient(circle, #059669, #022c22); border-color: #10b981; box-shadow: 0 0 40px rgba(16, 185, 129, 0.4); color: white; }
  .orb.disconnected { background: radial-gradient(circle, #1e293b, #0f172a); border-color: #475569; color: #94a3b8; }
  .orb.connecting { background: radial-gradient(circle, #d97706, #451a03); border-color: #f59e0b; color: white; animation: pulse 1.5s infinite; }
  @keyframes pulse { 0%, 100% { transform: scale(1); } 50% { transform: scale(1.05); } }
  .stats-grid { display: grid; grid-template-cols: 1fr 1fr; gap: 10px; margin-top: 16px; }
  .stat-box { background: #0f172a; padding: 12px; border-radius: 16px; border: 1px solid #1e293b; }
  .stat-label { font-size: 11px; color: #64748b; font-weight: 600; text-transform: uppercase; }
  .stat-val { font-size: 15px; font-weight: 800; color: #f8fafc; margin-top: 4px; font-family: monospace; }
  select { width: 100%; background: #0f172a; color: white; border: 1px solid #334155; padding: 14px; border-radius: 16px; font-size: 13px; font-weight: 600; margin-top: 16px; outline: none; }
  .btn { width: 100%; padding: 14px; border-radius: 16px; border: none; font-weight: bold; font-size: 14px; cursor: pointer; transition: 0.2s; margin-top: 12px; }
  .btn-primary { background: #0284c7; color: white; }
  .btn-primary:active { background: #0369a1; }
</style>
</head>
<body>
  <div class="card">
    <div class="title">⚡ NexVPN Remote</div>
    <div class="subtitle">Real-Time Mobile Controller</div>
    
    <div id="orb" class="orb disconnected" onclick="toggleConnect()">
      <span id="orb-text">CONNECT</span>
    </div>

    <div id="active-profile" style="font-size: 14px; font-weight: 700; color: #e2e8f0; margin-bottom: 4px;">Disconnected</div>
    <div id="active-server" style="font-size: 11px; color: #64748b; font-family: monospace;">-</div>

    <div class="stats-grid">
      <div class="stat-box">
        <div class="stat-label">⬇ Download</div>
        <div id="stat-down" class="stat-val">0 B</div>
      </div>
      <div class="stat-box">
        <div class="stat-label">⬆ Upload</div>
        <div id="stat-up" class="stat-val">0 B</div>
      </div>
      <div class="stat-box">
        <div class="stat-label">⚡ Speed</div>
        <div id="stat-speed" class="stat-val">0 KB/s</div>
      </div>
      <div class="stat-box">
        <div class="stat-label">⏱ Duration</div>
        <div id="stat-dur" class="stat-val">00:00</div>
      </div>
    </div>
  </div>

  <div class="card" style="padding: 16px;">
    <div style="font-size: 12px; font-weight: 700; color: #94a3b8; text-align: left;">SWITCH SERVER</div>
    <select id="server-select"></select>
    <button class="btn btn-primary" onclick="switchServer()">Connect Selected</button>
  </div>

<script>
let currentStatus = 'disconnected';
let profiles = [];

function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

function formatDuration(sec) {
  if (!sec) return '00:00';
  const m = Math.floor(sec / 60);
  const s = sec % 60;
  return String(m).padStart(2, '0') + ':' + String(s).padStart(2, '0');
}

async function updateState() {
  try {
    const res = await fetch('/api/status');
    const data = await res.json();
    currentStatus = data.status.state;
    
    const orb = document.getElementById('orb');
    const orbText = document.getElementById('orb-text');
    orb.className = 'orb ' + currentStatus;
    orbText.innerText = currentStatus.toUpperCase();

    document.getElementById('active-profile').innerText = data.status.profile_name || 'Disconnected';
    document.getElementById('active-server').innerText = data.status.server || '-';
    
    document.getElementById('stat-down').innerText = formatBytes(data.stats.bytes_received);
    document.getElementById('stat-up').innerText = formatBytes(data.stats.bytes_sent);
    document.getElementById('stat-speed').innerText = formatBytes(data.stats.speed_download) + '/s';
    document.getElementById('stat-dur').innerText = formatDuration(data.status.duration);

    if (profiles.length !== data.profiles.length) {
      profiles = data.profiles;
      const select = document.getElementById('server-select');
      select.innerHTML = '';
      profiles.forEach(p => {
        const opt = document.createElement('option');
        opt.value = p.id;
        opt.innerText = p.name + (p.latency_ms ? ' (' + p.latency_ms + 'ms)' : '');
        select.appendChild(opt);
      });
    }
  } catch (err) {
    console.error(err);
  }
}

async function toggleConnect() {
  if (currentStatus === 'connected') {
    await fetch('/api/disconnect', { method: 'POST' });
  } else {
    const select = document.getElementById('server-select');
    if (select.value) {
      await fetch('/api/connect', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ profile_id: select.value })
      });
    }
  }
  updateState();
}

async function switchServer() {
  const select = document.getElementById('server-select');
  if (select.value) {
    await fetch('/api/connect', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ profile_id: select.value })
    });
    updateState();
  }
}

setInterval(updateState, 1500);
updateState();
</script>
</body>
</html>`
	_, _ = w.Write([]byte(html))
}
