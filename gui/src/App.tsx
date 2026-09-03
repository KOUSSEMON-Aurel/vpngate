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
  ShieldAlert,
} from "lucide-react";
import { api, ServerInfo, StatusInfo } from "./api";
import { WorldMap } from "./WorldMap";

// Minimalist Gateway Vector Icon
function GatePortalIcon() {
  return (
    <svg
      width="15"
      height="15"
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
  const [publicIp, setPublicIp] = useState<string>("");

  // Live Health Map (hostname -> { status, latency_ms })
  const [healthMap, setHealthMap] = useState<
    Record<string, { status: "working" | "failed" | "checking" | "unknown"; latency_ms?: number }>
  >({});

  // Manual Target Relay (null = Mode Automatique / "Emplacement le plus rapide")
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

  // Fetch real public IP when disconnected
  useEffect(() => {
    if (backend !== "ok") return;
    const fetchIp = async () => {
      try {
        const res = await api.ip();
        if (res.ip) setPublicIp(res.ip);
      } catch {
        try {
          const res = await fetch("https://api.ipify.org?format=json");
          const data = await res.json();
          if (data.ip) setPublicIp(data.ip);
        } catch {
          // ignore
        }
      }
    };
    void fetchIp();
    const id = setInterval(fetchIp, 12000);
    return () => clearInterval(id);
  }, [backend, connected]);

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
          // Automatic Mode: Connect to best online server (or lowest ping)
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
            <div className="brand-glyph-box">
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
              <Zap size={14} />
              <span>Connexion</span>
            </button>
            <button
              className={`nav-link ${activeTab === "servers" ? "active" : ""}`}
              onClick={() => setActiveTab("servers")}
            >
              <Globe size={14} />
              <span>Emplacements</span>
            </button>
            <button
              className={`nav-link ${activeTab === "logs" ? "active" : ""}`}
              onClick={() => setActiveTab("logs")}
            >
              <Terminal size={14} />
              <span>Journal</span>
            </button>
            <button
              className={`nav-link ${activeTab === "settings" ? "active" : ""}`}
              onClick={() => setActiveTab("settings")}
            >
              <Settings size={14} />
              <span>Paramètres</span>
            </button>
          </nav>
        </div>

        <div className="rail-footer">
          <span className="status-indicator-tag">
            <span
              style={{
                width: "6px",
                height: "6px",
                borderRadius: "50%",
                backgroundColor: connected ? "var(--accent-green)" : connecting ? "var(--accent-amber)" : "var(--text-muted)",
              }}
            />
            {connected ? "Tunnel actif" : connecting ? "Négociation" : "Inactif"}
          </span>
          <span>{servers.length} relais</span>
        </div>
      </aside>

      {/* Main Stage Canvas */}
      <main className="app-stage">
        <header className="stage-header">
          <span className="stage-title">
            {activeTab === "connect" && "Connexion"}
            {activeTab === "servers" && "Emplacements réseau"}
            {activeTab === "logs" && "Journal système"}
            {activeTab === "settings" && "Paramètres"}
          </span>

          <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
            <button
              className="btn-clean-ghost"
              onClick={() => void loadServers()}
              disabled={busy}
              title="Recharger la liste des serveurs"
            >
              <RefreshCw size={11} />
              <span>Actualiser</span>
            </button>
          </div>
        </header>

        {error && (
          <div
            style={{
              margin: "16px 28px 0",
              padding: "9px 12px",
              backgroundColor: "rgba(239, 68, 68, 0.1)",
              border: "1px solid rgba(239, 68, 68, 0.25)",
              borderRadius: "6px",
              color: "#fca5a5",
              fontSize: "12px",
            }}
          >
            {error}
          </div>
        )}

        <div className="stage-content">
          {/* ========================================================
              TAB: CONNECTION (ACCUEIL SANS CADRE LOURD)
              ======================================================== */}
          {activeTab === "connect" && (
            <div className="connection-flow">
              {/* Hero Status Typography */}
              <div className="hero-state-display">
                <div
                  className={`hero-pill-badge ${
                    connected ? "connected" : connecting ? "connecting" : ""
                  }`}
                >
                  <span
                    style={{
                      width: "6px",
                      height: "6px",
                      borderRadius: "50%",
                      backgroundColor: connected ? "var(--accent-green)" : connecting ? "var(--accent-amber)" : "var(--text-muted)",
                    }}
                  />
                  <span>{connected ? "Protégé" : connecting ? "Connexion" : "Non protégé"}</span>
                </div>

                <h1 className="hero-title-h1">
                  {connected
                    ? `Connecté à ${status.country || "Relais distant"}`
                    : connecting
                    ? "Sécurisation en cours..."
                    : "Connexion sécurisée"}
                </h1>

                <p className="hero-subtitle-p">
                  {connected
                    ? "Votre adresse IP publique est masquée et votre trafic est chiffré."
                    : "Votre trafic internet n'est pas protégé sur votre réseau local."}
                </p>
              </div>

              {/* Interactive World Map */}
              <WorldMap
                servers={enrichedServers}
                selectedCountry={selectedServer?.country_short}
                connectedCountry={status.country?.slice(0, 2)}
                isConnected={connected}
                onSelectCountry={(code) => {
                  const countryServers = enrichedServers.filter(
                    (s) => s.country_short.toUpperCase() === code.toUpperCase()
                  );
                  if (countryServers.length > 0) {
                    const working = countryServers.filter((s) => s.health === "working");
                    const best = (working.length > 0 ? working : countryServers)[0];
                    pickTargetServer(best);
                  }
                }}
              />

              {/* Interactive Location Selector Strip */}
              <div
                className="location-pick-strip"
                onClick={() => setActiveTab("servers")}
                title="Changer d'emplacement"
              >
                <div className="location-cluster-left">
                  <span className="iso-pill">
                    {connected
                      ? status.country?.slice(0, 2).toUpperCase() || "VPN"
                      : selectedServer
                      ? selectedServer.country_short.toUpperCase()
                      : "AUTO"}
                  </span>
                  <div className="location-text-stack">
                    <span className="location-name-bold">
                      {connected
                        ? status.country || "Relais Distant"
                        : selectedServer
                        ? selectedServer.country_long
                        : "Emplacement le plus rapide"}
                    </span>
                    <span className="location-sub-server">
                      {connected
                        ? status.hostname || status.ip_addr
                        : selectedServer
                        ? `${selectedServer.hostname} • ${selectedServer.ip}`
                        : "Sélection automatique du meilleur relais en ligne"}
                    </span>
                  </div>
                </div>

                <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
                  {!connected && selectedServer && (
                    <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                      <span className={`badge-status ${targetHealth}`}>
                        {targetHealth === "working" ? "En ligne" : targetHealth === "failed" ? "Hors ligne" : "Test..."}
                      </span>
                      <span className="location-latency-text">
                        {targetLatency ? `${targetLatency}ms` : selectedServer.ping}
                      </span>
                    </div>
                  )}
                  {!connected && !selectedServer && (
                    <span className="clean-spec-tag">Auto</span>
                  )}
                  <span style={{ fontSize: "11.5px", color: "var(--text-secondary)" }}>
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
                  style={{ alignSelf: "flex-start", marginTop: "-16px" }}
                >
                  <RotateCcw size={11} />
                  <span>Rétablir l'emplacement le plus rapide (Auto)</span>
                </button>
              )}

              {/* Primary Connect Button */}
              <button
                className={`btn-solid-action ${
                  connected ? "disconnect-mode" : connecting ? "connecting-mode" : ""
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
                    : "Se connecter maintenant"}
                </span>
              </button>

              {/* Clean Telemetry List */}
              <div className="telemetry-clean-list">
                <div className="telemetry-item-row">
                  <span className="label">Adresse IP</span>
                  <div className="val">
                    {connected ? (
                      <>
                        <span style={{ color: "var(--accent-green)", fontWeight: 600 }}>{status.ip_addr || "—"}</span>
                        <span className="clean-spec-tag" style={{ color: "var(--accent-green)", borderColor: "var(--accent-green-border)" }}>
                          Protégée (VPN)
                        </span>
                        {status.ip_addr && (
                          <button
                            className="btn-clean-ghost"
                            onClick={() => copyIp(status.ip_addr)}
                            title="Copier l'IP"
                            style={{ padding: "2px 6px" }}
                          >
                            {copiedIp ? <Check size={11} color="var(--accent-green)" /> : <Copy size={11} />}
                          </button>
                        )}
                      </>
                    ) : (
                      <>
                        <span>{publicIp || "Détection..."}</span>
                        {publicIp && (
                          <span className="clean-spec-tag" style={{ color: "var(--accent-amber)", borderColor: "rgba(245, 158, 11, 0.3)" }}>
                            Non masquée
                          </span>
                        )}
                        {publicIp && (
                          <button
                            className="btn-clean-ghost"
                            onClick={() => copyIp(publicIp)}
                            title="Copier l'IP réelle"
                            style={{ padding: "2px 6px" }}
                          >
                            {copiedIp ? <Check size={11} color="var(--accent-green)" /> : <Copy size={11} />}
                          </button>
                        )}
                      </>
                    )}
                  </div>
                </div>

                <div className="telemetry-item-row">
                  <span className="label">Durée de session</span>
                  <span className="val">
                    {connected ? (
                      <>
                        <Clock size={12} color="var(--accent-green)" />
                        <span style={{ color: "#fff", fontWeight: 600 }}>{duration}</span>
                      </>
                    ) : (
                      <span style={{ color: "var(--text-tertiary)" }}>Non connecté (Chronomètre arrêté)</span>
                    )}
                  </span>
                </div>

                <div className="telemetry-item-row">
                  <span className="label">Protocole</span>
                  <span className="val">
                    {connected
                      ? status.protocol || "OpenVPN (tun0 • Chiffré)"
                      : selectedServer
                      ? `${selectedServer.proto.toUpperCase()} ${selectedServer.transport || ""}`
                      : "OpenVPN (Sélection automatique)"}
                  </span>
                </div>

                <div className="telemetry-item-row">
                  <span className="label">Protection réseau</span>
                  <span className="val">
                    {connected ? (
                      <span style={{ color: "var(--accent-green)", display: "flex", alignItems: "center", gap: "5px" }}>
                        <Lock size={12} />
                        Actif • Fuites IPv6 & DNS isolées
                      </span>
                    ) : (
                      <span style={{ color: "var(--accent-amber)", display: "flex", alignItems: "center", gap: "5px" }}>
                        <ShieldAlert size={12} />
                        Inactif • Trafic réseau non chiffré
                      </span>
                    )}
                  </span>
                </div>
              </div>

              {/* Quick Presets */}
              <div className="quick-preset-bar">
                <button
                  className="preset-chip"
                  disabled={busy || connected}
                  onClick={() => {
                    pickTargetServer(null);
                    void handleConnect();
                  }}
                >
                  <Zap size={13} color="var(--accent-green)" />
                  <span>Plus rapide</span>
                </button>

                <button
                  className="preset-chip"
                  disabled={busy || connected}
                  onClick={() => void handleConnect(undefined, { random: true })}
                >
                  <Dices size={13} color="var(--accent-blue)" />
                  <span>Aléatoire</span>
                </button>

                <button
                  className="preset-chip"
                  disabled={busy || connected}
                  onClick={() => void handleConnect(undefined, { source: "warp" })}
                >
                  <Cloud size={13} color="var(--accent-amber)" />
                  <span>Cloudflare WARP</span>
                </button>
              </div>
            </div>
          )}

          {/* ========================================================
              TAB: SERVERS (MASTER-DETAIL WITH TEXT BADGES)
              ======================================================== */}
          {activeTab === "servers" && (
            <div className="master-detail-container">
              {/* Left: Countries List */}
              <div className="master-countries-pane">
                <div className="pane-search-header">
                  <div className="clean-search-input">
                    <Search size={13} color="var(--text-tertiary)" />
                    <input
                      type="text"
                      placeholder="Rechercher pays, IP..."
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                    />
                    {search && (
                      <span
                        style={{ cursor: "pointer", color: "var(--text-tertiary)", fontSize: "11px" }}
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

                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: "6px" }}>
                    <button
                      className={`pane-chip-filter ${onlineOnly ? "active" : ""}`}
                      onClick={() => setOnlineOnly(!onlineOnly)}
                    >
                      En ligne ({enrichedServers.filter((s) => s.health === "working").length})
                    </button>

                    <select
                      className="select-clean"
                      value={sortBy}
                      onChange={(e) => setSortBy(e.target.value as any)}
                    >
                      <option value="health">Trier: Santé d'abord</option>
                      <option value="ping">Trier: Latence / Ping</option>
                      <option value="score">Trier: Score</option>
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
                          <span className="iso-pill">{group.country_short.toUpperCase()}</span>
                          <div>
                            <span className="country-name-txt">{group.country_long}</span>
                            <span className="country-servers-count">
                              ({group.workingCount > 0 ? `${group.workingCount}/` : ""}{group.servers.length})
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

              {/* Right: Relays List */}
              <div className="detail-relays-pane">
                {activeCountry ? (
                  <>
                    <div className="detail-pane-header">
                      <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
                        <span className="iso-pill" style={{ height: "26px", fontSize: "12px" }}>
                          {activeCountry.country_short.toUpperCase()}
                        </span>
                        <div>
                          <div style={{ fontSize: "14.5px", fontWeight: 700, color: "#fff" }}>
                            {activeCountry.country_long}
                          </div>
                          <div style={{ fontSize: "11px", color: "var(--text-tertiary)" }}>
                            {activeCountry.servers.length} relais ({activeCountry.workingCount} en ligne)
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
                        Choisir le plus rapide
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
                              {/* Clean Status Badge (Text based) */}
                              <span className={`badge-status ${health}`}>
                                {health === "working"
                                  ? "En ligne"
                                  : health === "failed"
                                  ? "Hors ligne"
                                  : health === "checking"
                                  ? "Test..."
                                  : "Non testé"}
                              </span>

                              <div className="relay-info-cluster">
                                <span className="relay-hostname-bold">{s.hostname}</span>
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
            <div style={{ maxWidth: "520px", margin: "0 auto", width: "100%", display: "flex", flexDirection: "column", gap: "14px" }}>
              <div style={{ backgroundColor: "var(--bg-surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-md)", padding: "16px 18px" }}>
                <h3 style={{ fontSize: "13px", fontWeight: 600, color: "#fff", marginBottom: "12px" }}>
                  Sécurité réseau
                </h3>

                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "10px 0", borderBottom: "1px solid var(--border)" }}>
                  <div>
                    <div style={{ fontSize: "12.5px", fontWeight: 500, color: "var(--text-primary)" }}>
                      Blocage du trafic IPv6
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

              <div style={{ backgroundColor: "var(--bg-surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-md)", padding: "16px 18px" }}>
                <h3 style={{ fontSize: "13px", fontWeight: 600, color: "#fff", marginBottom: "4px" }}>
                  À propos
                </h3>
                <p style={{ fontSize: "11.5px", color: "var(--text-secondary)", lineHeight: "1.6" }}>
                  vpngate desktop • Client OpenVPN natif pour Linux et macOS.
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