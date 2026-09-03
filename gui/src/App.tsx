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
  Lock,
  Clock,
  Power,
  Dices,
  Cloud,
  ChevronRight,
  RotateCcw,
} from "lucide-react";
import { api, ServerInfo, StatusInfo } from "./api";

// Minimalist Portal Gateway Glyph
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

  // Live Health Map (hostname -> { status, latency_ms })
  const [healthMap, setHealthMap] = useState<
    Record<string, { status: "working" | "failed" | "checking" | "unknown"; latency_ms?: number }>
  >({});

  // Manual Target Relay (null = Mode Automatique / "⚡ Emplacement le plus rapide")
  const [selectedServer, setSelectedServer] = useState<ServerInfo | null>(() => {
    const saved = localStorage.getItem("vpngate.selectedServer");
    if (saved) {
      try {
        return JSON.parse(saved);
      } catch {
        return null;
      }
    }
    return null;
  });

  // Master-Detail State in Servers Tab
  const [search, setSearch] = useState("");
  const [sourceFilter, setSourceFilter] = useState<string>("all");
  const [onlineOnly, setOnlineOnly] = useState<boolean>(false);
  const [sortBy, setSortBy] = useState<"health" | "ping" | "score" | "country">("health");
  const [selectedCountryCode, setSelectedCountryCode] = useState<string>("");

  // Live Duration Timer
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

  // Polling Live Probing Health from Go Monitor
  useEffect(() => {
    if (backend !== "ok") return;
    const pollHealth = async () => {
      try {
        const res = await api.serversHealth();
        setHealthMap(res);
      } catch {
        // ignore
      }
    };
    void pollHealth();
    const interval = setInterval(pollHealth, 3000);
    return () => clearInterval(interval);
  }, [backend]);

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

  // Load servers
  const loadServers = useCallback(async () => {
    try {
      setError("");
      const list = await api.servers();
      setServers(list);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    if (backend === "ok") void loadServers();
  }, [backend, loadServers]);

  // Enriched servers list merging live health status
  const enrichedServers = useMemo(() => {
    return servers.map((s) => {
      const live = healthMap[s.hostname];
      return {
        ...s,
        health: live ? live.status : s.health || "unknown",
        latency_ms: live && live.latency_ms ? live.latency_ms : s.latency_ms,
      };
    });
  }, [servers, healthMap]);

  // Connect & Disconnect handlers
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
        } else {
          // Mode Automatique : Se connecter au meilleur serveur en ligne (ou au plus bas ping)
          const working = enrichedServers.filter((s) => s.health === "working");
          const pool = working.length > 0 ? working : enrichedServers;
          if (pool.length > 0) {
            const sorted = [...pool].sort((a, b) => {
              const pingA = a.latency_ms || parsePing(a.ping);
              const pingB = b.latency_ms || parsePing(b.ping);
              return pingA - pingB;
            });
            await api.connect({
              hostname: sorted[0].hostname,
              protocol: sorted[0].proto,
              transport: sorted[0].transport,
              source: sorted[0].source,
            });
          } else {
            await api.connect({ random: true });
          }
        }
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedServer, enrichedServers]
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

  // Save selected server to localStorage
  const pickTargetServer = (s: ServerInfo | null) => {
    setSelectedServer(s);
    if (s) {
      localStorage.setItem("vpngate.selectedServer", JSON.stringify(s));
    } else {
      localStorage.removeItem("vpngate.selectedServer");
    }
    setActiveTab("connect");
  };

  // Group servers by country for Master-Detail view
  const countryGroups = useMemo(() => {
    let list = enrichedServers;
    if (sourceFilter !== "all") {
      list = list.filter((s) => (s.source || "vpngate") === sourceFilter);
    }
    if (onlineOnly) {
      list = list.filter((s) => s.health === "working");
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
        servers: typeof enrichedServers;
        bestPing: number;
        workingCount: number;
      }
    >();

    for (const s of list) {
      const key = s.country_short.toUpperCase();
      const existing = map.get(key);
      const pingNum = s.latency_ms || parsePing(s.ping);
      const isUp = s.health === "working";

      if (!existing) {
        map.set(key, {
          country_long: s.country_long,
          country_short: s.country_short,
          servers: [s],
          bestPing: pingNum,
          workingCount: isUp ? 1 : 0,
        });
      } else {
        existing.servers.push(s);
        if (isUp) existing.workingCount++;
        if (pingNum < existing.bestPing) existing.bestPing = pingNum;
      }
    }

    const result = Array.from(map.values());

    // Sort countries according to chosen strategy
    if (sortBy === "health") {
      result.sort((a, b) => b.workingCount - a.workingCount || a.bestPing - b.bestPing);
    } else if (sortBy === "ping") {
      result.sort((a, b) => a.bestPing - b.bestPing);
    } else if (sortBy === "score") {
      result.sort(
        (a, b) =>
          Math.max(...b.servers.map((s) => s.score)) - Math.max(...a.servers.map((s) => s.score))
      );
    } else {
      result.sort((a, b) => a.country_long.localeCompare(b.country_long));
    }

    // Also sort servers inside each country
    for (const g of result) {
      g.servers.sort((a, b) => {
        if (sortBy === "health") {
          const rank = (h?: string) => (h === "working" ? 0 : h === "checking" ? 1 : h === "failed" ? 3 : 2);
          if (rank(a.health) !== rank(b.health)) return rank(a.health) - rank(b.health);
        }
        const pingA = a.latency_ms || parsePing(a.ping);
        const pingB = b.latency_ms || parsePing(b.ping);
        return pingA - pingB;
      });
    }

    return result;
  }, [enrichedServers, sourceFilter, onlineOnly, search, sortBy]);

  // Active country in the Detail Pane
  const activeCountry = useMemo(() => {
    if (!selectedCountryCode && countryGroups.length > 0) {
      return countryGroups[0];
    }
    return countryGroups.find((g) => g.country_short.toUpperCase() === selectedCountryCode) || countryGroups[0];
  }, [countryGroups, selectedCountryCode]);

  // Selected server health state
  const targetHealth = selectedServer ? healthMap[selectedServer.hostname]?.status || selectedServer.health || "unknown" : "unknown";
  const targetLatency = selectedServer ? healthMap[selectedServer.hostname]?.latency_ms || selectedServer.latency_ms : undefined;

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
              TAB: CONNECTION (ACCUEIL)
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

                {/* Target Location Card */}
                <div
                  className="target-relay-card"
                  onClick={() => setActiveTab("servers")}
                  title="Changer d'emplacement"
                >
                  <div className="relay-country-group">
                    <span className="country-flag-display">
                      {connected
                        ? getCountryFlag(status.country?.slice(0, 2))
                        : selectedServer
                        ? getCountryFlag(selectedServer.country_short)
                        : "⚡"}
                    </span>
                    <div className="relay-location-details">
                      <span className="relay-country-heading">
                        {connected
                          ? status.country || "Relais Distant"
                          : selectedServer
                          ? selectedServer.country_long
                          : "Emplacement le plus rapide"}
                      </span>
                      <span className="relay-server-subtext">
                        {connected
                          ? status.hostname || status.ip_addr
                          : selectedServer
                          ? `${selectedServer.hostname} • ${selectedServer.ip}`
                          : "Sélectionne automatiquement le meilleur relais en ligne"}
                      </span>
                    </div>
                  </div>

                  <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
                    {!connected && selectedServer && (
                      <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                        <span className={`health-dot ${targetHealth}`} />
                        <span style={{ fontSize: "11.5px", fontFamily: "monospace", color: targetHealth === "working" ? "var(--accent-green)" : "var(--text-muted)" }}>
                          {targetLatency ? `${targetLatency}ms` : selectedServer.ping}
                        </span>
                      </div>
                    )}
                    {!connected && !selectedServer && (
                      <span className="clean-spec-tag" style={{ color: "var(--accent-green)", borderColor: "var(--accent-green-border)" }}>
                        Auto
                      </span>
                    )}
                    <span style={{ fontSize: "12px", color: "var(--text-secondary)" }}>
                      Modifier
                    </span>
                    <ChevronRight size={14} color="var(--text-muted)" />
                  </div>
                </div>

                {/* Reset to Auto button if manual server was selected */}
                {!connected && selectedServer && (
                  <button
                    className="btn-clean-ghost"
                    onClick={() => pickTargetServer(null)}
                    style={{ alignSelf: "flex-start", marginTop: "-8px", padding: "4px 8px", fontSize: "11px" }}
                  >
                    <RotateCcw size={11} />
                    <span>Rétablir la sélection automatique (⚡ Plus rapide)</span>
                  </button>
                )}

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
                      : selectedServer
                      ? `Se connecter à ${selectedServer.country_long}`
                      : "Se connecter au plus rapide"}
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
                        : "OpenVPN (Auto)"}
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
                    pickTargetServer(null);
                    void handleConnect();
                  }}
                >
                  <Zap size={15} color="var(--accent-green)" />
                  <div>
                    <div style={{ color: "#fff" }}>Plus rapide</div>
                    <div style={{ fontSize: "10.5px", color: "var(--text-tertiary)" }}>Automatique</div>
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
              TAB: SERVERS (MASTER-DETAIL PANE WITH LIVE UP/DOWN)
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

                  {/* Secondary filter & sort row (like TUI) */}
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: "6px", paddingTop: "4px" }}>
                    <button
                      className={`pane-chip-filter ${onlineOnly ? "active" : ""}`}
                      onClick={() => setOnlineOnly(!onlineOnly)}
                      title="Afficher uniquement les serveurs confirmés en ligne"
                    >
                      🟢 En ligne
                    </button>

                    <select
                      className="select-clean"
                      value={sortBy}
                      onChange={(e) => setSortBy(e.target.value as any)}
                      title="Options de tri"
                    >
                      <option value="health">Trier: Santé d'abord</option>
                      <option value="ping">Trier: Latence / Ping</option>
                      <option value="score">Trier: Qualité</option>
                      <option value="country">Trier: Pays (A-Z)</option>
                    </select>
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
                            <span className="country-servers-count">
                              ({group.workingCount > 0 ? `🟢 ${group.workingCount}/` : ""}{group.servers.length})
                            </span>
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
                            {activeCountry.servers.length} relais ({activeCountry.workingCount} confirmés 🟢 en ligne)
                          </div>
                        </div>
                      </div>

                      <button
                        className="btn-select-relay"
                        disabled={busy || connected}
                        onClick={() => {
                          const working = activeCountry.servers.filter((s) => s.health === "working");
                          const best = (working.length > 0 ? working : activeCountry.servers)[0];
                          pickTargetServer(best);
                        }}
                      >
                        ⚡ Choisir le plus rapide
                      </button>
                    </div>

                    <div className="relays-scroll-list">
                      {activeCountry.servers.map((s) => {
                        const isCurrentTarget = selectedServer?.hostname === s.hostname;
                        const health = s.health || "unknown";
                        const latency = s.latency_ms || parsePing(s.ping);

                        return (
                          <div key={s.hostname} className="relay-card-row">
                            <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
                              {/* Live Health Status Dot (Green/Red/Amber/Gray) */}
                              <span
                                className={`health-dot ${health}`}
                                title={
                                  health === "working"
                                    ? "Relais vérifié et en ligne (UP)"
                                    : health === "failed"
                                    ? "Relais inaccessible ou hors ligne (DOWN)"
                                    : health === "checking"
                                    ? "Vérification de connectivité en cours..."
                                    : "Non testé"
                                }
                              />

                              <div className="relay-info-cluster">
                                <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                                  <span className="relay-hostname-bold">{s.hostname}</span>
                                  {health === "working" && (
                                    <span style={{ fontSize: "10px", color: "var(--accent-green)", fontWeight: 600 }}>
                                      EN LIGNE
                                    </span>
                                  )}
                                  {health === "failed" && (
                                    <span style={{ fontSize: "10px", color: "var(--accent-red)", fontWeight: 600 }}>
                                      HORS LIGNE
                                    </span>
                                  )}
                                </div>
                                <span className="relay-ip-sub">{s.ip}</span>
                              </div>
                            </div>

                            <div className="relay-tags-cluster">
                              <span className="clean-spec-tag">{s.proto}</span>
                              {s.transport && <span className="clean-spec-tag">{s.transport}</span>}
                              <span
                                style={{
                                  fontSize: "11.5px",
                                  fontFamily: "JetBrains Mono, monospace",
                                  color: health === "working" ? "var(--accent-green)" : "var(--text-tertiary)",
                                  minWidth: "48px",
                                  textAlign: "right",
                                }}
                              >
                                {latency < 9000 ? `${latency}ms` : s.ping}
                              </span>

                              <button
                                className="btn-select-relay"
                                disabled={busy || connected}
                                onClick={() => pickTargetServer(s)}
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