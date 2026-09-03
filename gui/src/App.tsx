import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Shield,
  ShieldCheck,
  ShieldAlert,
  Power,
  Zap,
  Dices,
  Cloud,
  Globe,
  Server as ServerIcon,
  Terminal,
  Settings,
  RefreshCw,
  Copy,
  Check,
  Clock,
  Activity,
  Search,
  Wifi,
  Lock,
  Cpu,
  LayoutGrid,
  List,
} from "lucide-react";
import { api, ServerInfo, StatusInfo } from "./api";

const SOURCES = [
  { id: "", label: "Toutes les sources" },
  { id: "vpngate", label: "VPN Gate" },
  { id: "vpnbook", label: "VPNBook" },
  { id: "warp", label: "Cloudflare WARP" },
];

const TRANSPORTS = [
  { id: "", label: "Tous les transports" },
  { id: "tcp443", label: "TCP 443" },
  { id: "udp53", label: "UDP 53" },
  { id: "tcp80", label: "TCP 80" },
  { id: "udp25000", label: "UDP 25000" },
];

function getCountryFlag(code?: string): string {
  if (!code || code.length !== 2) return "🌐";
  const upper = code.toUpperCase();
  const c1 = upper.charCodeAt(0) - 65 + 0x1f1e6;
  const c2 = upper.charCodeAt(1) - 65 + 0x1f1e6;
  return String.fromCodePoint(c1, c2);
}

function formatScore(score: number): string {
  if (score >= 1_000_000) return `${(score / 1_000_000).toFixed(1)}M`;
  if (score >= 1_000) return `${(score / 1_000).toFixed(0)}k`;
  return String(score);
}

function parsePing(ping: string): number {
  const n = parseInt(ping, 10);
  return isNaN(n) ? 9999 : n;
}

export default function App() {
  const [activeTab, setActiveTab] = useState<"dashboard" | "servers" | "logs" | "settings">("dashboard");
  const [backend, setBackend] = useState<"checking" | "ok" | "down">("checking");
  const [status, setStatus] = useState<StatusInfo>({ state: "DISCONNECTED" });
  const [servers, setServers] = useState<ServerInfo[]>([]);
  const [error, setError] = useState<string>("");
  const [busy, setBusy] = useState(false);
  const [copiedIp, setCopiedIp] = useState(false);

  // Server Filtering & View mode
  const [search, setSearch] = useState("");
  const [source, setSource] = useState("");
  const [transport, setTransport] = useState("");
  const [sortBy, setSortBy] = useState<"ping" | "score" | "country">("ping");
  const [viewMode, setViewMode] = useState<"grid" | "table">("grid");
  const [refresh, setRefresh] = useState(false);

  // Live Timer
  const [duration, setDuration] = useState("00:00:00");
  const startTimerRef = useRef<number | null>(null);

  const connected = status.state === "CONNECTED";
  const connecting = status.state === "CONNECTING";

  // Backend Health & Status Polling
  useEffect(() => {
    const tick = async () => {
      try {
        await api.health();
        setBackend("ok");
      } catch {
        setBackend("down");
        return;
      }
      try {
        const s = await api.status();
        setStatus(s);
        if (s.state === "CONNECTED" && !startTimerRef.current) {
          startTimerRef.current = s.started_at ? new Date(s.started_at).getTime() : Date.now();
        } else if (s.state === "DISCONNECTED") {
          startTimerRef.current = null;
          setDuration("00:00:00");
        }
      } catch {
        // ignore jitter
      }
    };
    void tick();
    const id = setInterval(tick, 1500);
    return () => clearInterval(id);
  }, []);

  // Duration Clock
  useEffect(() => {
    if (!connected || !startTimerRef.current) return;
    const interval = setInterval(() => {
      const now = Date.now();
      const diff = Math.max(0, Math.floor((now - (startTimerRef.current ?? now)) / 1000));
      const h = String(Math.floor(diff / 3600)).padStart(2, "0");
      const m = String(Math.floor((diff % 3600) / 60)).padStart(2, "0");
      const s = String(diff % 60).padStart(2, "0");
      setDuration(`${h}:${m}:${s}`);
    }, 1000);
    return () => clearInterval(interval);
  }, [connected]);

  // Load Server List
  const loadServers = useCallback(async () => {
    try {
      setError("");
      const params: Record<string, string> = {};
      if (source) params.source = source;
      if (transport) params.transport = transport;
      if (refresh) params.refresh = "1";
      const list = await api.servers(params);
      setServers(list);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [source, transport, refresh]);

  useEffect(() => {
    if (backend === "ok") void loadServers();
  }, [backend, loadServers]);

  // Connect & Disconnect API
  const handleConnect = useCallback(async (body: Parameters<typeof api.connect>[0]) => {
    setBusy(true);
    setError("");
    try {
      await api.connect(body);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }, []);

  const handleDisconnect = useCallback(async () => {
    setBusy(true);
    setError("");
    try {
      await api.disconnect();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }, []);

  // Sorted and Filtered Servers
  const filteredServers = useMemo(() => {
    let result = servers;
    if (search.trim()) {
      const q = search.toLowerCase().trim();
      result = result.filter(
        (s) =>
          s.country_long.toLowerCase().includes(q) ||
          s.country_short.toLowerCase().includes(q) ||
          s.hostname.toLowerCase().includes(q) ||
          s.ip.includes(q)
      );
    }
    const cloned = [...result];
    if (sortBy === "ping") {
      cloned.sort((a, b) => parsePing(a.ping) - parsePing(b.ping));
    } else if (sortBy === "score") {
      cloned.sort((a, b) => b.score - a.score);
    } else if (sortBy === "country") {
      cloned.sort((a, b) => a.country_long.localeCompare(b.country_long));
    }
    return cloned;
  }, [servers, search, sortBy]);

  // Best server for quick connect
  const bestServer = useMemo(() => {
    if (servers.length === 0) return null;
    const sorted = [...servers].sort((a, b) => parsePing(a.ping) - parsePing(b.ping));
    return sorted[0];
  }, [servers]);

  const copyIp = (ip?: string) => {
    if (!ip) return;
    void navigator.clipboard.writeText(ip);
    setCopiedIp(true);
    setTimeout(() => setCopiedIp(false), 2000);
  };

  return (
    <div className="app-shell">
      {/* Left Sidebar */}
      <aside className="sidebar">
        <div className="sidebar-top">
          <div className="brand-section">
            <div className="brand-logo-icon">
              <Shield size={22} strokeWidth={2.5} />
            </div>
            <div className="brand-text-wrap">
              <span className="brand-name">VPNGate</span>
              <span className="brand-badge">Client Pro</span>
            </div>
          </div>

          {/* Global Status Pill */}
          <div className={`status-pill ${connected ? "connected" : connecting ? "connecting" : ""}`}>
            <span className="status-beacon" />
            <div className="status-text-group">
              <span className="status-title">
                {connected ? "Protégé" : connecting ? "Connexion…" : "Non protégé"}
              </span>
              <span className="status-subtitle">
                {connected ? (status.country || "VPN Actif") : "Trafic non chiffré"}
              </span>
            </div>
          </div>

          {/* Navigation Links */}
          <nav className="sidebar-nav">
            <button
              className={`nav-item ${activeTab === "dashboard" ? "active" : ""}`}
              onClick={() => setActiveTab("dashboard")}
            >
              <Zap size={18} />
              <span>Dashboard</span>
            </button>
            <button
              className={`nav-item ${activeTab === "servers" ? "active" : ""}`}
              onClick={() => setActiveTab("servers")}
            >
              <Globe size={18} />
              <span>Serveurs ({servers.length})</span>
            </button>
            <button
              className={`nav-item ${activeTab === "logs" ? "active" : ""}`}
              onClick={() => setActiveTab("logs")}
            >
              <Terminal size={18} />
              <span>Journaux & Logs</span>
            </button>
            <button
              className={`nav-item ${activeTab === "settings" ? "active" : ""}`}
              onClick={() => setActiveTab("settings")}
            >
              <Settings size={18} />
              <span>Sécurité & Infos</span>
            </button>
          </nav>
        </div>

        {/* Sidebar Footer */}
        <div className="sidebar-footer">
          <div className="daemon-indicator">
            <span>Daemon Core</span>
            <span>
              <span className={`daemon-dot ${backend === "ok" ? "ok" : "down"}`} />
              {backend === "ok" ? "127.0.0.1:1865" : "Inaccessible"}
            </span>
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <main className="content-area">
        {/* Top App Bar */}
        <header className="top-bar">
          <h1 className="top-bar-title">
            {activeTab === "dashboard" && "Tableau de Bord VPN"}
            {activeTab === "servers" && "Relais & Serveurs Mondiaux"}
            {activeTab === "logs" && "Diagnostic du Tunnel"}
            {activeTab === "settings" && "Paramètres & Sécurité"}
          </h1>

          <div className="top-bar-actions">
            {connected && (
              <button
                className="btn-action-danger"
                disabled={busy}
                onClick={() => void handleDisconnect()}
              >
                <Power size={16} />
                <span>Déconnecter</span>
              </button>
            )}
          </div>
        </header>

        {/* Error notification banner */}
        {error && (
          <div style={{ margin: "14px 28px 0", padding: "12px 18px", background: "rgba(244, 63, 94, 0.15)", border: "1px solid rgba(244, 63, 94, 0.3)", borderRadius: "12px", color: "#fca5a5", display: "flex", alignItems: "center", gap: "10px", fontSize: "13px" }}>
            <ShieldAlert size={18} />
            <span>{error}</span>
          </div>
        )}

        {/* Tab Content */}
        <div className="tab-content">
          {/* ========================================================
              TAB: DASHBOARD
              ======================================================== */}
          {activeTab === "dashboard" && (
            <div className="dashboard-grid">
              {/* Hero Connect Card */}
              <div className="hero-connect-card">
                <div className="power-button-wrap">
                  <button
                    className={`power-button ${connected ? "connected" : connecting ? "connecting" : ""}`}
                    disabled={busy || backend !== "ok"}
                    onClick={() => {
                      if (connected) {
                        void handleDisconnect();
                      } else {
                        void handleConnect({ random: true });
                      }
                    }}
                    title={connected ? "Cliquer pour déconnecter" : "Cliquer pour connecter"}
                  >
                    {connected ? (
                      <ShieldCheck size={42} strokeWidth={2.2} />
                    ) : connecting ? (
                      <Activity size={42} strokeWidth={2.2} />
                    ) : (
                      <Power size={42} strokeWidth={2.2} />
                    )}
                    <span style={{ fontSize: "10.5px", fontWeight: 700, letterSpacing: "0.06em", marginTop: "4px" }}>
                      {connected ? "CONNECTÉ" : connecting ? "CONNEXION…" : "CONNECTER"}
                    </span>
                  </button>
                </div>

                <div>
                  <h2 className="hero-status-heading">
                    {connected ? (
                      <span className="hero-country-badge">
                        <span>{getCountryFlag(status.country?.slice(0, 2))}</span>
                        <span>{status.country || "Relais Connecté"}</span>
                      </span>
                    ) : connecting ? (
                      "Établissement du tunnel sécurisé…"
                    ) : (
                      "Votre connexion n'est pas protégée"
                    )}
                  </h2>
                  <p style={{ color: "var(--text-muted)", fontSize: "13px", marginTop: "4px" }}>
                    {connected
                      ? "Tout votre trafic réseau est chiffré et sécurisé contre les fuites."
                      : "Sélectionnez un serveur ou utilisez la connexion rapide."}
                  </p>
                </div>

                <div className="hero-meta-row">
                  {connected && status.ip_addr && (
                    <button
                      className="btn-action-ghost"
                      onClick={() => copyIp(status.ip_addr)}
                      title="Copier l'IP publique"
                    >
                      <span>📍 IP: {status.ip_addr}</span>
                      {copiedIp ? <Check size={14} color="var(--emerald)" /> : <Copy size={14} />}
                    </button>
                  )}
                  {connected && (
                    <div className="btn-action-ghost" style={{ cursor: "default" }}>
                      <Clock size={14} color="var(--cyan)" />
                      <span>Durée: {duration}</span>
                    </div>
                  )}
                  {connected && status.hostname && (
                    <div className="btn-action-ghost" style={{ cursor: "default" }}>
                      <ServerIcon size={14} />
                      <span>{status.hostname}</span>
                    </div>
                  )}
                </div>
              </div>

              {/* Quick Actions Row */}
              <div className="quick-actions-bar">
                <button
                  className="quick-card-btn"
                  disabled={busy || backend !== "ok" || connected}
                  onClick={() => {
                    if (bestServer) {
                      void handleConnect({ hostname: bestServer.hostname, protocol: bestServer.proto });
                    } else {
                      void handleConnect({});
                    }
                  }}
                >
                  <div className="quick-card-icon">
                    <Zap size={22} />
                  </div>
                  <div className="quick-card-info">
                    <span className="quick-card-title">Plus Rapide</span>
                    <span className="quick-card-sub">
                      {bestServer ? `${bestServer.country_long} (${bestServer.ping})` : "Latence la plus basse"}
                    </span>
                  </div>
                </button>

                <button
                  className="quick-card-btn"
                  disabled={busy || backend !== "ok" || connected}
                  onClick={() => void handleConnect({ random: true })}
                >
                  <div className="quick-card-icon" style={{ color: "var(--indigo)" }}>
                    <Dices size={22} />
                  </div>
                  <div className="quick-card-info">
                    <span className="quick-card-title">Aléatoire</span>
                    <span className="quick-card-sub">Sélectionne un relais libre</span>
                  </div>
                </button>

                <button
                  className="quick-card-btn"
                  disabled={busy || backend !== "ok" || connected}
                  onClick={() => void handleConnect({ source: "warp" })}
                >
                  <div className="quick-card-icon" style={{ color: "#f97316" }}>
                    <Cloud size={22} />
                  </div>
                  <div className="quick-card-info">
                    <span className="quick-card-title">Cloudflare WARP</span>
                    <span className="quick-card-sub">WireGuard 1.1.1.1</span>
                  </div>
                </button>
              </div>

              {/* Telemetry Grid */}
              <div className="telemetry-grid">
                <div className="telemetry-card">
                  <span className="telemetry-label">
                    <Lock size={13} color="var(--cyan)" />
                    Protection Fuite IPv6
                  </span>
                  <span className="telemetry-val" style={{ color: "var(--emerald)", fontSize: "13px" }}>
                    ✓ Active (Block IPv6)
                  </span>
                </div>

                <div className="telemetry-card">
                  <span className="telemetry-label">
                    <Wifi size={13} color="var(--blue)" />
                    DNS Guard
                  </span>
                  <span className="telemetry-val" style={{ color: "var(--emerald)", fontSize: "13px" }}>
                    ✓ Forcé dans tunnel
                  </span>
                </div>

                <div className="telemetry-card">
                  <span className="telemetry-label">
                    <Cpu size={13} color="var(--indigo)" />
                    Chiffrement
                  </span>
                  <span className="telemetry-val" style={{ fontSize: "13px" }}>
                    AES-128 / ChaCha20
                  </span>
                </div>

                <div className="telemetry-card">
                  <span className="telemetry-label">
                    <Activity size={13} color="var(--amber)" />
                    Relais Disponibles
                  </span>
                  <span className="telemetry-val">
                    {servers.length} serveurs
                  </span>
                </div>
              </div>
            </div>
          )}

          {/* ========================================================
              TAB: SERVERS EXPLORER
              ======================================================== */}
          {activeTab === "servers" && (
            <div style={{ display: "flex", flexDirection: "column", gap: "16px", height: "100%" }}>
              {/* Filter Toolbar */}
              <div className="servers-toolbar">
                <div className="search-input-wrap">
                  <Search size={16} color="var(--text-muted)" />
                  <input
                    type="text"
                    placeholder="Filtrer par pays, IP ou nom de relais…"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                  />
                  {search && (
                    <span style={{ cursor: "pointer", color: "var(--text-muted)" }} onClick={() => setSearch("")}>
                      ✕
                    </span>
                  )}
                </div>

                <div className="filter-controls">
                  <select
                    className="select-pill"
                    value={source}
                    onChange={(e) => setSource(e.target.value)}
                  >
                    {SOURCES.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.label}
                      </option>
                    ))}
                  </select>

                  <select
                    className="select-pill"
                    value={transport}
                    onChange={(e) => setTransport(e.target.value)}
                  >
                    {TRANSPORTS.map((t) => (
                      <option key={t.id} value={t.id}>
                        {t.label}
                      </option>
                    ))}
                  </select>

                  <select
                    className="select-pill"
                    value={sortBy}
                    onChange={(e) => setSortBy(e.target.value as any)}
                  >
                    <option value="ping">Trier: Latence (Ping)</option>
                    <option value="score">Trier: Qualité (Score)</option>
                    <option value="country">Trier: Pays (A-Z)</option>
                  </select>

                  <button
                    className="btn-action-ghost"
                    onClick={() => setViewMode(viewMode === "grid" ? "table" : "grid")}
                    title="Changer d'affichage"
                  >
                    {viewMode === "grid" ? <List size={16} /> : <LayoutGrid size={16} />}
                  </button>

                  <label style={{ display: "flex", alignItems: "center", gap: "6px", fontSize: "12px", color: "var(--text-secondary)", cursor: "pointer" }}>
                    <input
                      type="checkbox"
                      checked={refresh}
                      onChange={(e) => setRefresh(e.target.checked)}
                      style={{ accentColor: "var(--cyan)" }}
                    />
                    <span>Re-télécharger</span>
                  </label>

                  <button
                    className="btn-action-ghost"
                    onClick={() => void loadServers()}
                    disabled={backend !== "ok" || busy}
                    title="Recharger la liste"
                  >
                    <RefreshCw size={14} />
                  </button>
                </div>
              </div>

              {/* Grid View */}
              {viewMode === "grid" ? (
                <div className="servers-grid">
                  {filteredServers.map((s) => {
                    const pingVal = parsePing(s.ping);
                    const pingColor = pingVal < 80 ? "var(--emerald)" : pingVal < 180 ? "var(--amber)" : "var(--text-muted)";
                    const isCurrent = connected && status.hostname === s.hostname;

                    return (
                      <div
                        key={s.hostname}
                        className={`server-item-card ${isCurrent ? "active-tunnel" : ""}`}
                      >
                        <div className="card-header-row">
                          <div className="country-info-wrap">
                            <span className="country-flag-lg">{getCountryFlag(s.country_short)}</span>
                            <div>
                              <div className="country-name-bold">{s.country_long}</div>
                              <div className="server-host-sub">{s.ip}</div>
                            </div>
                          </div>

                          <div className="ping-tag" style={{ color: pingColor }}>
                            <Wifi size={13} />
                            <span>{s.ping || "-"}</span>
                          </div>
                        </div>

                        <div className="card-specs-row">
                          <span className={`spec-chip ${s.proto?.toLowerCase()}`}>
                            {s.proto || "openvpn"}
                          </span>
                          <span className="spec-chip">
                            {s.transport || "tcp443"}
                          </span>
                          <span className="spec-chip">
                            ⭐ {formatScore(s.score)}
                          </span>
                          {s.source && s.source !== "vpngate" && (
                            <span className="spec-chip" style={{ color: "var(--cyan)" }}>
                              {s.source}
                            </span>
                          )}
                        </div>

                        <div className="card-footer-row">
                          <span style={{ fontSize: "11px", color: "var(--text-muted)", fontFamily: "monospace" }}>
                            {s.hostname.length > 18 ? `${s.hostname.slice(0, 16)}…` : s.hostname}
                          </span>

                          <button
                            className="btn-card-connect"
                            disabled={busy || backend !== "ok" || connected}
                            onClick={() =>
                              void handleConnect({
                                hostname: s.hostname,
                                protocol: s.proto,
                                transport: s.transport,
                                source: s.source,
                              })
                            }
                          >
                            {isCurrent ? "Actif" : "Connecter"}
                          </button>
                        </div>
                      </div>
                    );
                  })}

                  {filteredServers.length === 0 && (
                    <div style={{ gridColumn: "1 / -1", textAlign: "center", padding: "60px 20px", color: "var(--text-muted)" }}>
                      <p style={{ fontSize: "32px", marginBottom: "8px" }}>📡</p>
                      <p>Aucun serveur ne correspond à vos filtres.</p>
                    </div>
                  )}
                </div>
              ) : (
                /* Table View */
                <div style={{ flex: 1, overflow: "auto", background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)" }}>
                  <table style={{ width: "100%", borderCollapse: "collapse", textAlign: "left", fontSize: "13px" }}>
                    <thead>
                      <tr style={{ background: "rgba(11, 16, 24, 0.8)", borderBottom: "1px solid var(--border)", color: "var(--text-muted)", fontSize: "11px", textTransform: "uppercase" }}>
                        <th style={{ padding: "12px 16px" }}>Pays</th>
                        <th style={{ padding: "12px 16px" }}>Relais</th>
                        <th style={{ padding: "12px 16px" }}>Protocole</th>
                        <th style={{ padding: "12px 16px" }}>Transport</th>
                        <th style={{ padding: "12px 16px" }}>Latence</th>
                        <th style={{ padding: "12px 16px" }}>Score</th>
                        <th style={{ padding: "12px 16px", textAlign: "right" }}>Action</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredServers.map((s) => {
                        const pingVal = parsePing(s.ping);
                        const pingColor = pingVal < 80 ? "var(--emerald)" : pingVal < 180 ? "var(--amber)" : "var(--text-muted)";
                        const isCurrent = connected && status.hostname === s.hostname;

                        return (
                          <tr
                            key={s.hostname}
                            style={{
                              borderBottom: "1px solid rgba(255, 255, 255, 0.04)",
                              background: isCurrent ? "rgba(16, 185, 129, 0.08)" : undefined,
                            }}
                          >
                            <td style={{ padding: "10px 16px", fontWeight: 600 }}>
                              <span style={{ marginRight: "8px", fontSize: "16px" }}>{getCountryFlag(s.country_short)}</span>
                              {s.country_long}
                            </td>
                            <td style={{ padding: "10px 16px" }}>
                              <div>{s.hostname}</div>
                              <div style={{ fontSize: "11px", color: "var(--text-muted)", fontFamily: "monospace" }}>{s.ip}</div>
                            </td>
                            <td style={{ padding: "10px 16px" }}>
                              <span className="spec-chip">{s.proto || "openvpn"}</span>
                            </td>
                            <td style={{ padding: "10px 16px" }}>
                              <span className="spec-chip">{s.transport || "tcp443"}</span>
                            </td>
                            <td style={{ padding: "10px 16px", color: pingColor, fontFamily: "monospace", fontWeight: 700 }}>
                              {s.ping || "-"}
                            </td>
                            <td style={{ padding: "10px 16px", fontFamily: "monospace" }}>
                              {formatScore(s.score)}
                            </td>
                            <td style={{ padding: "10px 16px", textAlign: "right" }}>
                              <button
                                className="btn-card-connect"
                                disabled={busy || backend !== "ok" || connected}
                                onClick={() =>
                                  void handleConnect({
                                    hostname: s.hostname,
                                    protocol: s.proto,
                                    transport: s.transport,
                                    source: s.source,
                                  })
                                }
                              >
                                {isCurrent ? "Actif" : "Connecter"}
                              </button>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          )}

          {/* ========================================================
              TAB: LOGS & DIAGNOSTICS
              ======================================================== */}
          {activeTab === "logs" && <LogsTab backend={backend} />}

          {/* ========================================================
              TAB: SETTINGS & INFO
              ======================================================== */}
          {activeTab === "settings" && (
            <div style={{ display: "flex", flexDirection: "column", gap: "20px", maxWidth: "680px" }}>
              <div style={{ background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", padding: "22px" }}>
                <h3 style={{ fontSize: "15px", fontWeight: 700, marginBottom: "14px", display: "flex", alignItems: "center", gap: "8px" }}>
                  <Shield size={18} color="var(--cyan)" />
                  Paramètres de Sécurité & Fuites
                </h3>

                <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 0", borderBottom: "1px solid var(--border-subtle)" }}>
                    <div>
                      <div style={{ fontWeight: 600 }}>Protection contre les fuites IPv6</div>
                      <div style={{ fontSize: "12px", color: "var(--text-muted)" }}>Bloque les requêtes IPv6 pour éviter la dé-anonymisation via FAI</div>
                    </div>
                    <span style={{ color: "var(--emerald)", fontWeight: 700 }}>Actif (--block-ipv6)</span>
                  </div>

                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 0", borderBottom: "1px solid var(--border-subtle)" }}>
                    <div>
                      <div style={{ fontWeight: 600 }}>Protection Fuite DNS</div>
                      <div style={{ fontSize: "12px", color: "var(--text-muted)" }}>Redirige le résolveur DNS exclusivement vers le tunnel OpenVPN</div>
                    </div>
                    <span style={{ color: "var(--emerald)", fontWeight: 700 }}>Actif (tun0)</span>
                  </div>

                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 0" }}>
                    <div>
                      <div style={{ fontWeight: 600 }}>Daemon Partagé (/var/run/vpngate)</div>
                      <div style={{ fontSize: "12px", color: "var(--text-muted)" }}>Contrôle unifié entre CLI, TUI et l'interface Desktop</div>
                    </div>
                    <span style={{ color: "var(--cyan)", fontWeight: 700 }}>Synchronisé</span>
                  </div>
                </div>
              </div>

              <div style={{ background: "var(--bg-card)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", padding: "22px" }}>
                <h3 style={{ fontSize: "15px", fontWeight: 700, marginBottom: "8px" }}>À Propos</h3>
                <p style={{ color: "var(--text-secondary)", fontSize: "13px", lineHeight: "1.6" }}>
                  VPNGate Desktop v0.1.0 • Client sécurisé haute performance basé sur Tauri 2, Go et OpenVPN.
                </p>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

function LogsTab({ backend }: { backend: string }) {
  const [logText, setLogText] = useState("");
  const [autoScroll, setAutoScroll] = useState(true);
  const [copied, setCopied] = useState(false);
  const terminalRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    if (backend !== "ok") return;
    const fetchLogs = async () => {
      try {
        const res = await api.logs(500);
        setLogText(res.log);
      } catch {
        // ignore
      }
    };
    void fetchLogs();
    const interval = setInterval(fetchLogs, 2000);
    return () => clearInterval(interval);
  }, [backend]);

  useEffect(() => {
    if (autoScroll && terminalRef.current) {
      terminalRef.current.scrollTop = terminalRef.current.scrollHeight;
    }
  }, [logText, autoScroll]);

  const copyLogs = () => {
    void navigator.clipboard.writeText(logText);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="logs-window">
      <div className="logs-top-toolbar">
        <label style={{ display: "flex", alignItems: "center", gap: "8px", fontSize: "12.5px", cursor: "pointer", color: "var(--text-secondary)" }}>
          <input
            type="checkbox"
            checked={autoScroll}
            onChange={(e) => setAutoScroll(e.target.checked)}
            style={{ accentColor: "var(--cyan)" }}
          />
          <span>Défilement automatique en direct</span>
        </label>

        <div style={{ display: "flex", gap: "8px" }}>
          <button className="btn-action-ghost" onClick={copyLogs}>
            {copied ? <Check size={14} color="var(--emerald)" /> : <Copy size={14} />}
            <span>{copied ? "Copié !" : "Copier"}</span>
          </button>
          <button className="btn-action-ghost" onClick={() => setLogText("")}>
            <span>Effacer</span>
          </button>
        </div>
      </div>

      <pre ref={terminalRef} className="logs-scrollable-body">
        {logText
          ? logText.split("\n").map((line, idx) => {
              let cls = "";
              if (line.includes("Initialization Sequence Completed") || line.includes("connected via")) {
                cls = "log-init";
              } else if (line.includes("ERROR") || line.includes("AUTH_FAILED") || line.includes("failed")) {
                cls = "log-err";
              } else if (line.includes("WARNING")) {
                cls = "log-warn";
              }
              return (
                <div key={idx} className={cls}>
                  {line}
                </div>
              );
            })
          : "En attente des messages et événements du daemon OpenVPN…"}
      </pre>
    </div>
  );
}