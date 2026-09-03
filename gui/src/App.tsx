import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Zap,
  Globe,
  Terminal,
  Settings,
  RefreshCw,
  Copy,
  Check,
  Search,
  Wifi,
  Lock,
  Clock,
  Power,
  Dices,
  Cloud,
  ChevronRight,
} from "lucide-react";
import { api, ServerInfo, StatusInfo } from "./api";

// Custom Geometric Gateway Portal Icon (No generic shield)
function GatePortalIcon() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M4 20V9a8 8 0 0 1 16 0v11" />
      <path d="M9 20v-6a3 3 0 0 1 6 0v6" />
      <circle cx="12" cy="6.5" r="1.2" fill="currentColor" />
    </svg>
  );
}

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

  // Selected Target Relay
  const [selectedServer, setSelectedServer] = useState<ServerInfo | null>(null);

  // Master-Detail State in Servers Tab
  const [search, setSearch] = useState("");
  const [sourceFilter, setSourceFilter] = useState<string>("all");
  const [selectedCountryCode, setSelectedCountryCode] = useState<string>("");

  // Live Timer
  const [duration, setDuration] = useState("00:00:00");
  const startTimerRef = useRef<number | null>(null);

  const connected = status.state === "CONNECTED";
  const connecting = status.state === "CONNECTING";

  // Polling backend status & health
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
        // ignore
      }
    };
    void tick();
    const id = setInterval(tick, 1500);
    return () => clearInterval(id);
  }, []);

  // Duration clock
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

  // Load server list
  const loadServers = useCallback(async () => {
    try {
      setError("");
      const list = await api.servers();
      setServers(list);
      if (!selectedServer && list.length > 0) {
        const sorted = [...list].sort((a, b) => parsePing(a.ping) - parsePing(b.ping));
        setSelectedServer(sorted[0]);
        setSelectedCountryCode(sorted[0].country_short.toUpperCase());
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [selectedServer]);

  useEffect(() => {
    if (backend === "ok") void loadServers();
  }, [backend, loadServers]);

  // Connect & Disconnect
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

  // Group servers by country for Master-Detail view
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

    return Array.from(map.values()).sort((a, b) => a.country_long.localeCompare(b.country_long));
  }, [servers, sourceFilter, search]);

  // Currently active country in detail pane
  const activeCountry = useMemo(() => {
    if (!selectedCountryCode && countryGroups.length > 0) {
      return countryGroups[0];
    }
    return countryGroups.find((g) => g.country_short.toUpperCase() === selectedCountryCode) || countryGroups[0];
  }, [countryGroups, selectedCountryCode]);

  const copyIp = (ip?: string) => {
    if (!ip) return;
    void navigator.clipboard.writeText(ip);
    setCopiedIp(true);
    setTimeout(() => setCopiedIp(false), 2000);
  };

  return (
    <div className="app-shell">
      {/* Left Navigation Rail */}
      <aside className="nav-rail">
        <div>
          <div className="brand-row">
            <div className="brand-portal-icon">
              <GatePortalIcon />
            </div>
            <div className="brand-meta">
              <span className="brand-name">vpngate</span>
              <span className="brand-tag">sécurisé & libre</span>
            </div>
          </div>

          <nav className="nav-menu">
            <button
              className={`nav-link ${activeTab === "connect" ? "active" : ""}`}
              onClick={() => setActiveTab("connect")}
            >
              <Zap size={15} />
              <span>Connexion</span>
            </button>
            <button
              className={`nav-link ${activeTab === "servers" ? "active" : ""}`}
              onClick={() => setActiveTab("servers")}
            >
              <Globe size={15} />
              <span>Emplacements</span>
            </button>
            <button
              className={`nav-link ${activeTab === "logs" ? "active" : ""}`}
              onClick={() => setActiveTab("logs")}
            >
              <Terminal size={15} />
              <span>Journal</span>
            </button>
            <button
              className={`nav-link ${activeTab === "settings" ? "active" : ""}`}
              onClick={() => setActiveTab("settings")}
            >
              <Settings size={15} />
              <span>Paramètres</span>
            </button>
          </nav>
        </div>

        <div className="rail-footer">
          <span style={{ display: "flex", alignItems: "center" }}>
            <span
              className={`status-dot-mini ${
                connected ? "active" : connecting ? "pending" : "idle"
              }`}
            />
            {connected ? "Tunnel actif" : connecting ? "Négociation" : "Inactif"}
          </span>
          <span>{servers.length} relais</span>
        </div>
      </aside>

      {/* Main Stage */}
      <main className="app-stage">
        <header className="stage-header">
          <span className="stage-title">
            {activeTab === "connect" && "Tableau de bord"}
            {activeTab === "servers" && "Sélection d'emplacement"}
            {activeTab === "logs" && "Journal d'activité du tunnel"}
            {activeTab === "settings" && "Paramètres et sécurité"}
          </span>

          <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
            <button
              className="btn-clean-ghost"
              onClick={() => void loadServers()}
              disabled={busy}
              title="Recharger la liste des serveurs"
            >
              <RefreshCw size={12} />
              <span>Actualiser</span>
            </button>
          </div>
        </header>

        {error && (
          <div
            style={{
              margin: "16px 24px 0",
              padding: "10px 14px",
              backgroundColor: "rgba(239, 68, 68, 0.1)",
              border: "1px solid rgba(239, 68, 68, 0.25)",
              borderRadius: "6px",
              color: "#fca5a5",
              fontSize: "12.5px",
            }}
          >
            {error}
          </div>
        )}

        <div className="stage-content">
          {/* ========================================================
              TAB: CONNECTION
              ======================================================== */}
          {activeTab === "connect" && (
            <div className="connection-panel">
              {/* Main Status & Connect Box */}
              <div className="status-banner-card">
                <div className="banner-top-row">
                  <div>
                    <div style={{ fontSize: "16px", fontWeight: 700, color: "#fff" }}>
                      {connected
                        ? `Connecté à ${status.country || "Relais distant"}`
                        : connecting
                        ? "Sécurisation en cours..."
                        : "Non connecté"}
                    </div>
                    <div style={{ fontSize: "12px", color: "var(--text-tertiary)", marginTop: "2px" }}>
                      {connected
                        ? "Tout votre trafic passe par le tunnel chiffré"
                        : "Votre trafic internet n'est pas protégé"}
                    </div>
                  </div>

                  <div
                    className={`status-badge-chip ${
                      connected ? "connected" : connecting ? "connecting" : ""
                    }`}
                  >
                    <span
                      className={`status-dot-mini ${
                        connected ? "active" : connecting ? "pending" : "idle"
                      }`}
                    />
                    <span>{connected ? "Protégé" : connecting ? "Connexion" : "Déconnecté"}</span>
                  </div>
                </div>

                {/* Selected Relay Card */}
                <div
                  className="target-relay-card"
                  onClick={() => setActiveTab("servers")}
                  title="Changer d'emplacement"
                >
                  <div className="relay-country-group">
                    <span className="country-flag-display">
                      {getCountryFlag(
                        connected
                          ? status.country?.slice(0, 2)
                          : selectedServer?.country_short
                      )}
                    </span>
                    <div className="relay-location-details">
                      <span className="relay-country-heading">
                        {connected
                          ? status.country || "Japon"
                          : selectedServer
                          ? selectedServer.country_long
                          : "Choisir un pays"}
                      </span>
                      <span className="relay-server-subtext">
                        {connected
                          ? status.hostname || status.ip_addr
                          : selectedServer
                          ? `${selectedServer.hostname} • ${selectedServer.ip}`
                          : "Cliquer pour parcourir les emplacements"}
                      </span>
                    </div>
                  </div>

                  <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                    {!connected && selectedServer && (
                      <div className="relay-latency-pill">
                        <Wifi size={12} />
                        <span>{selectedServer.ping}</span>
                      </div>
                    )}
                    <span style={{ fontSize: "12px", color: "var(--text-secondary)" }}>
                      Modifier
                    </span>
                    <ChevronRight size={14} color="var(--text-muted)" />
                  </div>
                </div>

                {/* Connect / Disconnect Action */}
                <button
                  className={`btn-connect-solid ${
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
                  <Power size={15} />
                  <span>
                    {connected
                      ? "Déconnecter"
                      : connecting
                      ? "Connexion en cours..."
                      : "Se connecter"}
                  </span>
                </button>
              </div>

              {/* Metrics Grid */}
              <div className="metrics-row">
                <div className="metric-box">
                  <span className="metric-label">Adresse IP publique</span>
                  <div className="metric-value">
                    <span>{connected ? status.ip_addr || "—" : "IP opérateur"}</span>
                    {connected && status.ip_addr && (
                      <button
                        className="btn-clean-ghost"
                        onClick={() => copyIp(status.ip_addr)}
                        title="Copier l'IP"
                        style={{ padding: "3px 6px" }}
                      >
                        {copiedIp ? <Check size={12} color="var(--accent-green)" /> : <Copy size={12} />}
                      </button>
                    )}
                  </div>
                </div>

                <div className="metric-box">
                  <span className="metric-label">Durée de session</span>
                  <div className="metric-value">
                    <span style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                      <Clock size={13} color="var(--text-tertiary)" />
                      {connected ? duration : "00:00:00"}
                    </span>
                  </div>
                </div>

                <div className="metric-box">
                  <span className="metric-label">Protocole & Port</span>
                  <div className="metric-value">
                    <span>
                      {connected
                        ? status.protocol || "OpenVPN (tun0)"
                        : selectedServer
                        ? `${selectedServer.proto.toUpperCase()} ${selectedServer.transport || ""}`
                        : "OpenVPN"}
                    </span>
                  </div>
                </div>

                <div className="metric-box">
                  <span className="metric-label">Fuites DNS & IPv6</span>
                  <div className="metric-value">
                    <span style={{ color: "var(--accent-green)", display: "flex", alignItems: "center", gap: "6px" }}>
                      <Lock size={12} />
                      Bloquées
                    </span>
                  </div>
                </div>
              </div>

              {/* Quick Options */}
              <div className="shortcuts-grid">
                <button
                  className="shortcut-btn"
                  disabled={busy || connected}
                  onClick={() => {
                    if (servers.length > 0) {
                      const sorted = [...servers].sort((a, b) => parsePing(a.ping) - parsePing(b.ping));
                      void handleConnect(sorted[0]);
                    }
                  }}
                >
                  <Zap size={15} color="var(--accent-green)" />
                  <div>
                    <div style={{ color: "#fff" }}>Plus rapide</div>
                    <div style={{ fontSize: "10.5px", color: "var(--text-tertiary)" }}>Ping le plus bas</div>
                  </div>
                </button>

                <button
                  className="shortcut-btn"
                  disabled={busy || connected}
                  onClick={() => void handleConnect(undefined, { random: true })}
                >
                  <Dices size={15} color="var(--accent-blue)" />
                  <div>
                    <div style={{ color: "#fff" }}>Aléatoire</div>
                    <div style={{ fontSize: "10.5px", color: "var(--text-tertiary)" }}>N'importe où</div>
                  </div>
                </button>

                <button
                  className="shortcut-btn"
                  disabled={busy || connected}
                  onClick={() => void handleConnect(undefined, { source: "warp" })}
                >
                  <Cloud size={15} color="var(--accent-amber)" />
                  <div>
                    <div style={{ color: "#fff" }}>Cloudflare WARP</div>
                    <div style={{ fontSize: "10.5px", color: "var(--text-tertiary)" }}>WireGuard 1.1.1.1</div>
                  </div>
                </button>
              </div>
            </div>
          )}

          {/* ========================================================
              TAB: SERVERS (MASTER-DETAIL PANE)
              ======================================================== */}
          {activeTab === "servers" && (
            <div className="master-detail-container">
              {/* Left Pane: Country List */}
              <div className="master-countries-pane">
                <div className="pane-search-header">
                  <div className="clean-search-input">
                    <Search size={14} color="var(--text-tertiary)" />
                    <input
                      type="text"
                      placeholder="Filtrer par pays, IP..."
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                    />
                    {search && (
                      <span
                        style={{ cursor: "pointer", color: "var(--text-tertiary)" }}
                        onClick={() => setSearch("")}
                      >
                        ✕
                      </span>
                    )}
                  </div>

                  <div className="pane-chips-row">
                    <button
                      className={`pane-chip-filter ${sourceFilter === "all" ? "active" : ""}`}
                      onClick={() => setSourceFilter("all")}
                    >
                      Tous
                    </button>
                    <button
                      className={`pane-chip-filter ${sourceFilter === "vpngate" ? "active" : ""}`}
                      onClick={() => setSourceFilter("vpngate")}
                    >
                      VPN Gate
                    </button>
                    <button
                      className={`pane-chip-filter ${sourceFilter === "vpnbook" ? "active" : ""}`}
                      onClick={() => setSourceFilter("vpnbook")}
                    >
                      VPNBook
                    </button>
                    <button
                      className={`pane-chip-filter ${sourceFilter === "warp" ? "active" : ""}`}
                      onClick={() => setSourceFilter("warp")}
                    >
                      WARP
                    </button>
                  </div>
                </div>

                <div className="countries-scroll-list">
                  {countryGroups.map((group) => {
                    const isSelected = activeCountry?.country_short === group.country_short;
                    return (
                      <div
                        key={group.country_short}
                        className={`country-item-row ${isSelected ? "selected" : ""}`}
                        onClick={() => setSelectedCountryCode(group.country_short.toUpperCase())}
                      >
                        <div className="country-left-meta">
                          <span style={{ fontSize: "20px", lineHeight: 1 }}>
                            {getCountryFlag(group.country_short)}
                          </span>
                          <div>
                            <span className="country-name-txt">{group.country_long}</span>
                            <span className="country-servers-count">({group.servers.length})</span>
                          </div>
                        </div>

                        <div style={{ display: "flex", alignItems: "center", gap: "6px", fontSize: "11px", color: "var(--accent-green)", fontFamily: "monospace" }}>
                          <span>{group.bestPing < 9000 ? `${group.bestPing}ms` : "—"}</span>
                          <ChevronRight size={13} color="var(--text-muted)" />
                        </div>
                      </div>
                    );
                  })}

                  {countryGroups.length === 0 && (
                    <div style={{ padding: "20px", textAlign: "center", color: "var(--text-tertiary)" }}>
                      Aucun emplacement trouvé.
                    </div>
                  )}
                </div>
              </div>

              {/* Right Pane: Relays in Selected Country */}
              <div className="detail-relays-pane">
                {activeCountry ? (
                  <>
                    <div className="detail-pane-header">
                      <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
                        <span style={{ fontSize: "24px", lineHeight: 1 }}>
                          {getCountryFlag(activeCountry.country_short)}
                        </span>
                        <div>
                          <div style={{ fontSize: "15px", fontWeight: 700, color: "#fff" }}>
                            {activeCountry.country_long}
                          </div>
                          <div style={{ fontSize: "11.5px", color: "var(--text-tertiary)" }}>
                            {activeCountry.servers.length} relais disponibles
                          </div>
                        </div>
                      </div>

                      <button
                        className="btn-select-relay"
                        disabled={busy || connected}
                        onClick={() => {
                          const best = [...activeCountry.servers].sort(
                            (a, b) => parsePing(a.ping) - parsePing(b.ping)
                          )[0];
                          setSelectedServer(best);
                          setActiveTab("connect");
                        }}
                      >
                        Choisir le plus rapide
                      </button>
                    </div>

                    <div className="relays-scroll-list">
                      {activeCountry.servers.map((s) => {
                        const isCurrentTarget = selectedServer?.hostname === s.hostname;
                        return (
                          <div key={s.hostname} className="relay-card-row">
                            <div className="relay-info-cluster">
                              <span className="relay-hostname-bold">{s.hostname}</span>
                              <span className="relay-ip-sub">{s.ip}</span>
                            </div>

                            <div className="relay-tags-cluster">
                              <span className="clean-spec-tag">{s.proto}</span>
                              {s.transport && <span className="clean-spec-tag">{s.transport}</span>}
                              <span
                                style={{
                                  fontSize: "11.5px",
                                  fontFamily: "JetBrains Mono, monospace",
                                  color: "var(--accent-green)",
                                  minWidth: "48px",
                                  textAlign: "right",
                                }}
                              >
                                {s.ping}
                              </span>

                              <button
                                className="btn-select-relay"
                                disabled={busy || connected}
                                onClick={() => {
                                  setSelectedServer(s);
                                  setActiveTab("connect");
                                }}
                              >
                                {isCurrentTarget ? "Sélectionné" : "Choisir"}
                              </button>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </>
                ) : (
                  <div style={{ padding: "40px", textAlign: "center", color: "var(--text-tertiary)" }}>
                    Sélectionnez un pays à gauche pour voir ses relais.
                  </div>
                )}
              </div>
            </div>
          )}

          {/* ========================================================
              TAB: LOGS
              ======================================================== */}
          {activeTab === "logs" && <TerminalLogsView backend={backend} />}

          {/* ========================================================
              TAB: SETTINGS
              ======================================================== */}
          {activeTab === "settings" && (
            <div style={{ maxWidth: "560px", margin: "0 auto", width: "100%", display: "flex", flexDirection: "column", gap: "14px" }}>
              <div style={{ backgroundColor: "var(--bg-surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", padding: "18px 20px" }}>
                <h3 style={{ fontSize: "13.5px", fontWeight: 600, color: "#fff", marginBottom: "12px" }}>
                  Sécurité et isolation réseau
                </h3>

                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 0", borderBottom: "1px solid var(--border)" }}>
                  <div>
                    <div style={{ fontSize: "12.5px", fontWeight: 500, color: "var(--text-primary)" }}>
                      Blocage des fuites IPv6
                    </div>
                    <div style={{ fontSize: "11px", color: "var(--text-tertiary)" }}>
                      Désactive le trafic IPv6 non chiffré sur l'interface
                    </div>
                  </div>
                  <span style={{ fontSize: "11.5px", color: "var(--accent-green)", fontWeight: 600 }}>
                    Actif
                  </span>
                </div>

                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 0", borderBottom: "1px solid var(--border)" }}>
                  <div>
                    <div style={{ fontSize: "12.5px", fontWeight: 500, color: "var(--text-primary)" }}>
                      DNS Tunnel Exclusif
                    </div>
                    <div style={{ fontSize: "11px", color: "var(--text-tertiary)" }}>
                      Empêche votre FAI d'observer vos résolutions de noms
                    </div>
                  </div>
                  <span style={{ fontSize: "11.5px", color: "var(--accent-green)", fontWeight: 600 }}>
                    Actif
                  </span>
                </div>

                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 0" }}>
                  <div>
                    <div style={{ fontSize: "12.5px", fontWeight: 500, color: "var(--text-primary)" }}>
                      Daemon de contrôle unifié
                    </div>
                    <div style={{ fontSize: "11px", color: "var(--text-tertiary)" }}>
                      Partage automatique de l'état avec la CLI et la TUI
                    </div>
                  </div>
                  <span style={{ fontSize: "11.5px", color: "var(--accent-blue)", fontWeight: 600 }}>
                    /var/run/vpngate
                  </span>
                </div>
              </div>

              <div style={{ backgroundColor: "var(--bg-surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", padding: "18px 20px" }}>
                <h3 style={{ fontSize: "13.5px", fontWeight: 600, color: "#fff", marginBottom: "4px" }}>
                  À propos
                </h3>
                <p style={{ fontSize: "12px", color: "var(--text-secondary)", lineHeight: "1.6" }}>
                  vpngate desktop • Client OpenVPN natif épuré pour Linux et macOS.
                </p>
              </div>
            </div>
          )}
        </div>
      </main>
    </div>
  );
}

function TerminalLogsView({ backend }: { backend: string }) {
  const [logText, setLogText] = useState("");
  const [copied, setCopied] = useState(false);
  const logBoxRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    if (backend !== "ok") return;
    const fetchLogs = async () => {
      try {
        const res = await api.logs(250);
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
    <div className="clean-terminal-box">
      <div className="terminal-bar">
        <span>daemon.log</span>
        <div style={{ display: "flex", gap: "6px" }}>
          <button className="btn-clean-ghost" onClick={copy}>
            {copied ? <Check size={11} color="var(--accent-green)" /> : <Copy size={11} />}
            <span>{copied ? "Copié" : "Copier"}</span>
          </button>
          <button className="btn-clean-ghost" onClick={() => setLogText("")}>
            Effacer
          </button>
        </div>
      </div>

      <pre ref={logBoxRef} className="terminal-stream">
        {logText || "En attente des messages du daemon..."}
      </pre>
    </div>
  );
}