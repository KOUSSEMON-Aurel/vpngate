import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Shield,
  Zap,
  Globe,
  Terminal,
  Settings,
  RefreshCw,
  Copy,
  Check,
  ChevronDown,
  ChevronRight,
  ChevronRight as ArrowRight,
  Search,
  Wifi,
  Lock,
  Clock,
  Power,
  Dices,
  Cloud,
} from "lucide-react";
import { api, ServerInfo, StatusInfo } from "./api";

function getCountryFlag(code?: string): string {
  if (!code || code.length !== 2) return "🌐";
  const upper = code.toUpperCase();
  const c1 = upper.charCodeAt(0) - 65 + 0x1f1e6;
  const c2 = upper.charCodeAt(1) - 65 + 0x1f1e6;
  return String.fromCodePoint(c1, c2);
}

function parsePing(ping: string): number {
  const n = parseInt(ping, 10);
  return isNaN(n) ? 9999 : n;
}

export default function App() {
  const [activeTab, setActiveTab] = useState<"connect" | "servers" | "logs" | "settings">("connect");
  const [backend, setBackend] = useState<"checking" | "ok" | "down">("checking");
  const [status, setStatus] = useState<StatusInfo>({ state: "DISCONNECTED" });
  const [servers, setServers] = useState<ServerInfo[]>([]);
  const [error, setError] = useState<string>("");
  const [busy, setBusy] = useState(false);
  const [copiedIp, setCopiedIp] = useState(false);

  // Selected server to connect
  const [selectedServer, setSelectedServer] = useState<ServerInfo | null>(null);

  // Servers Tab State
  const [search, setSearch] = useState("");
  const [sourceFilter, setSourceFilter] = useState<string>("all");
  const [expandedCountry, setExpandedCountry] = useState<string | null>(null);

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
        // ignore network hiccups
      }
    };
    void tick();
    const id = setInterval(tick, 1500);
    return () => clearInterval(id);
  }, []);

  // Duration Timer
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

  // Load Servers
  const loadServers = useCallback(async () => {
    try {
      setError("");
      const list = await api.servers();
      setServers(list);
      if (!selectedServer && list.length > 0) {
        // Default to the fastest server
        const sorted = [...list].sort((a, b) => parsePing(a.ping) - parsePing(b.ping));
        setSelectedServer(sorted[0]);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [selectedServer]);

  useEffect(() => {
    if (backend === "ok") void loadServers();
  }, [backend, loadServers]);

  // Handle Connect & Disconnect
  const handleConnect = useCallback(
    async (target?: ServerInfo, options: { random?: boolean; source?: string } = {}) => {
      setBusy(true);
      setError("");
      try {
        if (options.random) {
          await api.connect({ random: true });
        } else if (options.source) {
          await api.connect({ source: options.source });
        } else if (target) {
          await api.connect({
            hostname: target.hostname,
            protocol: target.proto,
            transport: target.transport,
            source: target.source,
          });
        } else if (selectedServer) {
          await api.connect({
            hostname: selectedServer.hostname,
            protocol: selectedServer.proto,
            transport: selectedServer.transport,
            source: selectedServer.source,
          });
        }
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedServer]
  );

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

  // Group servers by country
  const countryGroups = useMemo(() => {
    let list = servers;
    if (sourceFilter !== "all") {
      list = list.filter((s) => (s.source || "vpngate") === sourceFilter);
    }
    if (search.trim()) {
      const q = search.toLowerCase().trim();
      list = list.filter(
        (s) =>
          s.country_long.toLowerCase().includes(q) ||
          s.country_short.toLowerCase().includes(q) ||
          s.hostname.toLowerCase().includes(q) ||
          s.ip.includes(q)
      );
    }

    const map = new Map<
      string,
      {
        country_long: string;
        country_short: string;
        servers: ServerInfo[];
        bestPing: number;
      }
    >();

    for (const s of list) {
      const key = s.country_short.toUpperCase();
      const existing = map.get(key);
      const pingNum = parsePing(s.ping);
      if (!existing) {
        map.set(key, {
          country_long: s.country_long,
          country_short: s.country_short,
          servers: [s],
          bestPing: pingNum,
        });
      } else {
        existing.servers.push(s);
        if (pingNum < existing.bestPing) existing.bestPing = pingNum;
      }
    }

    // Sort countries by server count or lowest ping
    return Array.from(map.values()).sort((a, b) => a.country_long.localeCompare(b.country_long));
  }, [servers, sourceFilter, search]);

  const copyIp = (ip?: string) => {
    if (!ip) return;
    void navigator.clipboard.writeText(ip);
    setCopiedIp(true);
    setTimeout(() => setCopiedIp(false), 2000);
  };

  return (
    <div className="app-container">
      {/* Sidebar Navigation */}
      <aside className="app-sidebar">
        <div>
          <div className="brand-header">
            <div className="brand-icon-box">
              <Shield size={18} strokeWidth={2.5} />
            </div>
            <div>
              <div className="brand-title">vpngate</div>
              <div className="brand-edition">client sécurisé</div>
            </div>
          </div>

          <nav className="nav-group">
            <button
              className={`nav-button ${activeTab === "connect" ? "active" : ""}`}
              onClick={() => setActiveTab("connect")}
            >
              <Zap size={16} />
              <span>Connexion</span>
            </button>
            <button
              className={`nav-button ${activeTab === "servers" ? "active" : ""}`}
              onClick={() => setActiveTab("servers")}
            >
              <Globe size={16} />
              <span>Emplacements</span>
            </button>
            <button
              className={`nav-button ${activeTab === "logs" ? "active" : ""}`}
              onClick={() => setActiveTab("logs")}
            >
              <Terminal size={16} />
              <span>Journal</span>
            </button>
            <button
              className={`nav-button ${activeTab === "settings" ? "active" : ""}`}
              onClick={() => setActiveTab("settings")}
            >
              <Settings size={16} />
              <span>Paramètres</span>
            </button>
          </nav>
        </div>

        <div className="sidebar-footer-info">
          <span style={{ display: "flex", alignItems: "center" }}>
            <span
              className={`status-bullet ${
                connected ? "green" : connecting ? "amber" : "gray"
              }`}
            />
            {connected ? "Tunnel actif" : connecting ? "Négociation..." : "Inactif"}
          </span>
          <span>{servers.length} relais</span>
        </div>
      </aside>

      {/* Main Area */}
      <main className="app-content">
        <header className="top-navbar">
          <div className="page-heading">
            {activeTab === "connect" && "Connexion VPN"}
            {activeTab === "servers" && "Sélection de l'emplacement"}
            {activeTab === "logs" && "Journal d'activité"}
            {activeTab === "settings" && "Paramètres de sécurité"}
          </div>

          <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
            <button
              className="btn-icon-subtle"
              onClick={() => void loadServers()}
              title="Actualiser la liste"
              disabled={busy}
            >
              <RefreshCw size={13} />
              <span>Actualiser</span>
            </button>
          </div>
        </header>

        {error && (
          <div
            style={{
              margin: "16px 28px 0",
              padding: "10px 16px",
              backgroundColor: "rgba(239, 68, 68, 0.12)",
              border: "1px solid rgba(239, 68, 68, 0.25)",
              borderRadius: "8px",
              color: "#fca5a5",
              fontSize: "12.5px",
            }}
          >
            {error}
          </div>
        )}

        <div className="page-body">
          {/* ========================================================
              TAB: CONNECTION
              ======================================================== */}
          {activeTab === "connect" && (
            <div className="connection-view">
              {/* Main Status & Connect Card */}
              <div className="connection-hero-card">
                <div className="hero-status-row">
                  <div>
                    <div style={{ fontSize: "18px", fontWeight: 700, color: "#fff" }}>
                      {connected
                        ? `Connecté à ${status.country || "Relais distant"}`
                        : connecting
                        ? "Connexion en cours..."
                        : "Non connecté"}
                    </div>
                    <div style={{ fontSize: "12.5px", color: "var(--text-muted)", marginTop: "2px" }}>
                      {connected
                        ? "Tout votre trafic est chiffré via le tunnel OpenVPN"
                        : "Votre trafic internet n'est pas protégé"}
                    </div>
                  </div>

                  <div
                    className={`status-badge-clean ${
                      connected ? "connected" : connecting ? "connecting" : ""
                    }`}
                  >
                    <span
                      className={`status-bullet ${
                        connected ? "green" : connecting ? "amber" : "gray"
                      }`}
                    />
                    <span>{connected ? "Protégé" : connecting ? "Sécurisation" : "Déconnecté"}</span>
                  </div>
                </div>

                {/* Target Location Card */}
                <div
                  className="location-selector-box"
                  onClick={() => setActiveTab("servers")}
                  title="Cliquer pour changer d'emplacement"
                >
                  <div className="location-left-info">
                    <span className="location-flag-badge">
                      {getCountryFlag(
                        connected
                          ? status.country?.slice(0, 2)
                          : selectedServer?.country_short
                      )}
                    </span>
                    <div className="location-text-group">
                      <span className="location-country-name">
                        {connected
                          ? status.country || "Japon"
                          : selectedServer
                          ? selectedServer.country_long
                          : "Choisir un pays"}
                      </span>
                      <span className="location-server-name">
                        {connected
                          ? status.hostname || status.ip_addr
                          : selectedServer
                          ? `${selectedServer.hostname} • ${selectedServer.ip}`
                          : "Aucun relais sélectionné"}
                      </span>
                    </div>
                  </div>

                  <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                    {!connected && selectedServer && (
                      <div className="location-latency-tag">
                        <Wifi size={13} color="var(--accent-green)" />
                        <span>{selectedServer.ping}</span>
                      </div>
                    )}
                    <span style={{ fontSize: "12px", color: "var(--text-secondary)" }}>
                      Modifier
                    </span>
                    <ArrowRight size={14} color="var(--text-muted)" />
                  </div>
                </div>

                {/* Action Button */}
                <button
                  className={`btn-primary-connect ${
                    connected ? "connected" : connecting ? "connecting" : ""
                  }`}
                  disabled={busy || backend !== "ok"}
                  onClick={() => {
                    if (connected) {
                      void handleDisconnect();
                    } else {
                      void handleConnect();
                    }
                  }}
                >
                  <Power size={16} />
                  <span>
                    {connected
                      ? "Déconnecter"
                      : connecting
                      ? "Connexion en cours..."
                      : "Se connecter"}
                  </span>
                </button>
              </div>

              {/* Stats Grid */}
              <div className="details-grid">
                <div className="details-tile">
                  <span className="details-tile-label">Adresse IP assignée</span>
                  <div className="details-tile-value">
                    <span>{connected ? status.ip_addr || "—" : "IP opérateur"}</span>
                    {connected && status.ip_addr && (
                      <button
                        className="btn-icon-subtle"
                        onClick={() => copyIp(status.ip_addr)}
                        title="Copier l'IP"
                      >
                        {copiedIp ? <Check size={12} color="var(--accent-green)" /> : <Copy size={12} />}
                      </button>
                    )}
                  </div>
                </div>

                <div className="details-tile">
                  <span className="details-tile-label">Temps de connexion</span>
                  <div className="details-tile-value">
                    <span style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                      <Clock size={13} color="var(--accent-blue)" />
                      {connected ? duration : "00:00:00"}
                    </span>
                  </div>
                </div>

                <div className="details-tile">
                  <span className="details-tile-label">Protocole</span>
                  <div className="details-tile-value">
                    <span>
                      {connected
                        ? status.protocol || "OpenVPN (tun0)"
                        : selectedServer
                        ? `${selectedServer.proto.toUpperCase()} ${selectedServer.transport || ""}`
                        : "OpenVPN"}
                    </span>
                  </div>
                </div>

                <div className="details-tile">
                  <span className="details-tile-label">Protection fuites</span>
                  <div className="details-tile-value">
                    <span style={{ color: "var(--accent-green)", display: "flex", alignItems: "center", gap: "6px" }}>
                      <Lock size={13} />
                      IPv6 & DNS isolés
                    </span>
                  </div>
                </div>
              </div>

              {/* Quick Options Strip */}
              <div className="shortcuts-strip">
                <button
                  className="shortcut-tile-btn"
                  disabled={busy || connected}
                  onClick={() => {
                    if (servers.length > 0) {
                      const sorted = [...servers].sort((a, b) => parsePing(a.ping) - parsePing(b.ping));
                      void handleConnect(sorted[0]);
                    }
                  }}
                >
                  <Zap size={18} color="var(--accent-green)" />
                  <div>
                    <div className="shortcut-tile-title">Plus rapide</div>
                    <div className="shortcut-tile-desc">Ping le plus bas</div>
                  </div>
                </button>

                <button
                  className="shortcut-tile-btn"
                  disabled={busy || connected}
                  onClick={() => void handleConnect(undefined, { random: true })}
                >
                  <Dices size={18} color="var(--accent-blue)" />
                  <div>
                    <div className="shortcut-tile-title">Aléatoire</div>
                    <div className="shortcut-tile-desc">N'importe quel pays</div>
                  </div>
                </button>

                <button
                  className="shortcut-tile-btn"
                  disabled={busy || connected}
                  onClick={() => void handleConnect(undefined, { source: "warp" })}
                >
                  <Cloud size={18} color="var(--accent-amber)" />
                  <div>
                    <div className="shortcut-tile-title">Cloudflare WARP</div>
                    <div className="shortcut-tile-desc">WireGuard 1.1.1.1</div>
                  </div>
                </button>
              </div>
            </div>
          )}

          {/* ========================================================
              TAB: SERVERS (GROUPED ACCORDION)
              ======================================================== */}
          {activeTab === "servers" && (
            <div className="servers-view-container">
              {/* Search & Filter Bar */}
              <div className="servers-search-bar">
                <Search size={16} color="var(--text-muted)" />
                <input
                  type="text"
                  placeholder="Rechercher un pays, une IP ou un serveur..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
                {search && (
                  <button
                    className="btn-icon-subtle"
                    onClick={() => setSearch("")}
                    style={{ border: "none" }}
                  >
                    ✕
                  </button>
                )}
              </div>

              <div className="chips-filter-row">
                <button
                  className={`filter-chip ${sourceFilter === "all" ? "active" : ""}`}
                  onClick={() => setSourceFilter("all")}
                >
                  Tous ({servers.length})
                </button>
                <button
                  className={`filter-chip ${sourceFilter === "vpngate" ? "active" : ""}`}
                  onClick={() => setSourceFilter("vpngate")}
                >
                  VPN Gate
                </button>
                <button
                  className={`filter-chip ${sourceFilter === "vpnbook" ? "active" : ""}`}
                  onClick={() => setSourceFilter("vpnbook")}
                >
                  VPNBook
                </button>
                <button
                  className={`filter-chip ${sourceFilter === "warp" ? "active" : ""}`}
                  onClick={() => setSourceFilter("warp")}
                >
                  Cloudflare WARP
                </button>
              </div>

              {/* Accordion Country List */}
              <div className="countries-list">
                {countryGroups.map((group) => {
                  const isExpanded = expandedCountry === group.country_short;
                  const isCurrentCountry =
                    connected && status.country?.includes(group.country_long);

                  return (
                    <div key={group.country_short} className="country-accordion-card">
                      <div
                        className="country-summary-row"
                        onClick={() =>
                          setExpandedCountry(isExpanded ? null : group.country_short)
                        }
                      >
                        <div className="country-meta-left">
                          <span className="country-flag-icon">
                            {getCountryFlag(group.country_short)}
                          </span>
                          <div>
                            <span className="country-title-text">{group.country_long}</span>
                            <span className="country-count-tag">
                              {group.servers.length} relais
                            </span>
                          </div>
                        </div>

                        <div className="country-summary-actions">
                          <div style={{ display: "flex", alignItems: "center", gap: "6px", fontSize: "12px", color: "var(--text-secondary)", fontFamily: "monospace" }}>
                            <Wifi size={12} color="var(--accent-green)" />
                            <span>{group.bestPing < 9000 ? `${group.bestPing} ms` : "—"}</span>
                          </div>

                          <button
                            className="btn-fast-connect"
                            disabled={busy || connected}
                            onClick={(e) => {
                              e.stopPropagation();
                              const best = [...group.servers].sort(
                                (a, b) => parsePing(a.ping) - parsePing(b.ping)
                              )[0];
                              setSelectedServer(best);
                              setActiveTab("connect");
                            }}
                          >
                            {isCurrentCountry ? "Actif" : "Choisir"}
                          </button>

                          {isExpanded ? (
                            <ChevronDown size={16} color="var(--text-muted)" />
                          ) : (
                            <ChevronRight size={16} color="var(--text-muted)" />
                          )}
                        </div>
                      </div>

                      {/* Expanded Sublist of Relays */}
                      {isExpanded && (
                        <div className="country-relays-sublist">
                          {group.servers.map((s) => (
                            <div key={s.hostname} className="relay-subrow">
                              <div className="relay-name-col">
                                <span className="relay-hostname-text">{s.hostname}</span>
                                <span className="relay-ip-text">{s.ip}</span>
                              </div>

                              <div className="relay-specs-col">
                                <span className="spec-pill-compact">{s.proto}</span>
                                {s.transport && (
                                  <span className="spec-pill-compact">{s.transport}</span>
                                )}
                                <span style={{ fontSize: "11.5px", fontFamily: "monospace", color: "var(--text-secondary)", minWidth: "50px", textAlign: "right" }}>
                                  {s.ping}
                                </span>
                                <button
                                  className="btn-fast-connect"
                                  disabled={busy || connected}
                                  onClick={() => {
                                    setSelectedServer(s);
                                    setActiveTab("connect");
                                  }}
                                >
                                  Sélectionner
                                </button>
                              </div>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  );
                })}

                {countryGroups.length === 0 && (
                  <div style={{ textAlign: "center", padding: "40px 20px", color: "var(--text-muted)" }}>
                    Aucun emplacement ne correspond à votre recherche.
                  </div>
                )}
              </div>
            </div>
          )}

          {/* ========================================================
              TAB: LOGS
              ======================================================== */}
          {activeTab === "logs" && <TerminalLogs backend={backend} />}

          {/* ========================================================
              TAB: SETTINGS
              ======================================================== */}
          {activeTab === "settings" && (
            <div style={{ maxWidth: "600px", margin: "0 auto", display: "flex", flexDirection: "column", gap: "16px" }}>
              <div style={{ backgroundColor: "var(--bg-card)", border: "1px solid var(--border-card)", borderRadius: "var(--radius-lg)", padding: "20px" }}>
                <h3 style={{ fontSize: "14px", fontWeight: 600, color: "#fff", marginBottom: "14px" }}>
                  Sécurité réseau
                </h3>

                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 0", borderBottom: "1px solid var(--border-subtle)" }}>
                  <div>
                    <div style={{ fontSize: "13px", fontWeight: 500, color: "var(--text-main)" }}>
                      Blocage du trafic IPv6
                    </div>
                    <div style={{ fontSize: "11.5px", color: "var(--text-muted)" }}>
                      Empêche les fuites d'adresse via IPv6 pendant la navigation
                    </div>
                  </div>
                  <span style={{ fontSize: "12px", color: "var(--accent-green)", fontWeight: 600 }}>
                    Actif
                  </span>
                </div>

                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 0", borderBottom: "1px solid var(--border-subtle)" }}>
                  <div>
                    <div style={{ fontSize: "13px", fontWeight: 500, color: "var(--text-main)" }}>
                      Routage DNS exclusif
                    </div>
                    <div style={{ fontSize: "11.5px", color: "var(--text-muted)" }}>
                      Force toutes les requêtes DNS à passer par le relais VPN
                    </div>
                  </div>
                  <span style={{ fontSize: "12px", color: "var(--accent-green)", fontWeight: 600 }}>
                    Actif
                  </span>
                </div>

                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 0" }}>
                  <div>
                    <div style={{ fontSize: "13px", fontWeight: 500, color: "var(--text-main)" }}>
                      Daemon de fond unifié
                    </div>
                    <div style={{ fontSize: "11.5px", color: "var(--text-muted)" }}>
                      Partagé avec les commandes CLI et TUI dans /var/run/vpngate
                    </div>
                  </div>
                  <span style={{ fontSize: "12px", color: "var(--accent-blue)", fontWeight: 600 }}>
                    Connecté
                  </span>
                </div>
              </div>

              <div style={{ backgroundColor: "var(--bg-card)", border: "1px solid var(--border-card)", borderRadius: "var(--radius-lg)", padding: "20px" }}>
                <h3 style={{ fontSize: "14px", fontWeight: 600, color: "#fff", marginBottom: "6px" }}>
                  À propos
                </h3>
                <p style={{ fontSize: "12.5px", color: "var(--text-secondary)", lineHeight: "1.6" }}>
                  VPNGate Client v1.0.0 — Interface native légère conçue pour Linux et macOS.
                </p>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

function TerminalLogs({ backend }: { backend: string }) {
  const [logText, setLogText] = useState("");
  const [copied, setCopied] = useState(false);
  const logBoxRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    if (backend !== "ok") return;
    const fetchLogs = async () => {
      try {
        const res = await api.logs(300);
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
    if (logBoxRef.current) {
      logBoxRef.current.scrollTop = logBoxRef.current.scrollHeight;
    }
  }, [logText]);

  const copy = () => {
    void navigator.clipboard.writeText(logText);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="terminal-shell">
      <div className="terminal-header-bar">
        <span>openvpn.log</span>
        <div style={{ display: "flex", gap: "8px" }}>
          <button className="btn-icon-subtle" onClick={copy}>
            {copied ? <Check size={12} color="var(--accent-green)" /> : <Copy size={12} />}
            <span>{copied ? "Copié" : "Copier"}</span>
          </button>
          <button className="btn-icon-subtle" onClick={() => setLogText("")}>
            Effacer
          </button>
        </div>
      </div>

      <pre ref={logBoxRef} className="terminal-body">
        {logText || "En attente des messages du daemon..."}
      </pre>
    </div>
  );
}