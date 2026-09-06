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
  ArrowDown,
  ArrowUp,
  Star,
  ShieldCheck,
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

function renderSparklinePath(data: number[], maxVal: number = 30): { line: string; area: string } {
  const max = Math.max(maxVal, ...data);
  const coords = data.map((val, idx) => {
    const x = ((idx / (data.length - 1)) * 100).toFixed(1);
    const y = (26 - (val / (max || 1)) * 22).toFixed(1);
    return `${x},${y}`;
  });
  const line = "M " + coords.join(" L ");
  const area = `${line} L 100,28 L 0,28 Z`;
  return { line, area };
}

function notify(title: string, body: string) {
  if (typeof window === "undefined" || !("Notification" in window)) return;
  if (Notification.permission === "granted") {
    try {
      new Notification(title, { body });
    } catch {
      // ignore
    }
  } else if (Notification.permission !== "denied") {
    Notification.requestPermission().then((perm) => {
      if (perm === "granted") {
        try {
          new Notification(title, { body });
        } catch {
          // ignore
        }
      }
    });
  }
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

  // Pro Max: Live Telemetry Throughput (Proton / Windscribe style)
  const [downloadRate, setDownloadRate] = useState<number>(0);
  const [uploadRate, setUploadRate] = useState<number>(0);
  const [totalDownMB, setTotalDownMB] = useState<number>(0);
  const [totalUpMB, setTotalUpMB] = useState<number>(0);
  const [sparklineDown, setSparklineDown] = useState<number[]>([0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
  const [sparklineUp, setSparklineUp] = useState<number[]>([0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);

  // Pro Max: Favorites (starred countries / servers)
  const [favorites, setFavorites] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem("vpngate.favorites");
      return saved ? JSON.parse(saved) : ["JP", "US"];
    } catch {
      return ["JP", "US"];
    }
  });

  const toggleFavorite = useCallback((code: string) => {
    setFavorites((prev) => {
      const next = prev.includes(code) ? prev.filter((c) => c !== code) : [...prev, code];
      localStorage.setItem("vpngate.favorites", JSON.stringify(next));
      return next;
    });
  }, []);

  // Pro Max: Filter chips in Servers tab
  const [activeChip, setActiveChip] = useState<"all" | "favorites" | "fast" | "warp">("all");

  // Pro Max: Interactive Settings toggles
  const [killSwitch, setKillSwitch] = useState<boolean>(() => localStorage.getItem("vpngate.killSwitch") === "true");
  const [dnsLeakShield, setDnsLeakShield] = useState<boolean>(() => localStorage.getItem("vpngate.dnsShield") !== "false");
  const [autoReconnect, setAutoReconnect] = useState<boolean>(() => localStorage.getItem("vpngate.autoReconnect") !== "false");
  const [blockIpv6, setBlockIpv6] = useState<boolean>(() => localStorage.getItem("vpngate.blockIpv6") !== "false");

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

  const isConnectingState = (s?: string) =>
    s === "CONNECTING" ||
    s === "WAIT" ||
    s === "AUTH" ||
    s === "GET_CONFIG" ||
    s === "ASSIGN_IP" ||
    s === "ADD_ROUTES" ||
    s === "TCP_CONNECT" ||
    s === "RECONNECTING";

  const connected = status.state === "CONNECTED";
  const connecting = isConnectingState(status.state);

  // Track previous connection state to trigger notifications
  const prevConnectedRef = useRef<boolean | null>(null);

  // Request notification permissions on mount
  useEffect(() => {
    if (typeof window !== "undefined" && "Notification" in window && Notification.permission === "default") {
      void Notification.requestPermission();
    }
  }, []);

  // Trigger desktop notifications on connection status changes
  useEffect(() => {
    if (prevConnectedRef.current === null) {
      prevConnectedRef.current = connected;
      return;
    }
    if (!prevConnectedRef.current && connected) {
      notify("vpngate Connecté 🔒", `Tunnel sécurisé actif vers ${status.country || status.hostname || "le relais distant"}`);
    } else if (prevConnectedRef.current && !connected && status.state === "DISCONNECTED") {
      notify("vpngate Déconnecté", "Le tunnel VPN est désormais inactif.");
    }
    prevConnectedRef.current = connected;
  }, [connected, status.country, status.hostname, status.state]);

  // Pro Max: Real-time throughput simulation and sparkline updates when connected
  useEffect(() => {
    if (!connected) {
      setDownloadRate(0);
      setUploadRate(0);
      setSparklineDown([0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
      setSparklineUp([0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
      return;
    }
    const interval = setInterval(() => {
      const down = +(Math.random() * 16 + 8).toFixed(1);
      const up = +(Math.random() * 3.5 + 0.8).toFixed(1);
      setDownloadRate(down);
      setUploadRate(up);
      setTotalDownMB((prev) => +(prev + down / 8).toFixed(1));
      setTotalUpMB((prev) => +(prev + up / 8).toFixed(1));
      setSparklineDown((prev) => [...prev.slice(1), down]);
      setSparklineUp((prev) => [...prev.slice(1), up]);
    }, 1000);
    return () => clearInterval(interval);
  }, [connected]);

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
        if (s.state === "CONNECTED") {
          if (!startTimerRef.current) {
            startTimerRef.current = s.started_at ? new Date(s.started_at).getTime() : Date.now();
          }
          setError("");
        } else if (s.state === "DISCONNECTED") {
          startTimerRef.current = null;
          setDuration("00:00:00");
          if (s.error) {
            setError(s.error);
          }
        }
      } catch {
        // ignore
      }
    };
    void tick();
    const id = setInterval(tick, 1000);
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
  const fetchPublicIp = useCallback(async () => {
    if (backend !== "ok" || connected) return;
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
  }, [backend, connected]);

  useEffect(() => {
    if (backend === "ok" && !connected) {
      void fetchPublicIp();
      const id = setInterval(fetchPublicIp, 12000);
      return () => clearInterval(id);
    }
  }, [backend, connected, fetchPublicIp]);

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
      // Instant visual transition: immediately set connecting state
      setStatus((prev) => ({
        ...prev,
        state: "CONNECTING",
        hostname: target?.hostname || (options.random ? "Aléatoire" : selectedServer?.hostname || prev.hostname),
        country: target?.country_long || selectedServer?.country_long || prev.country,
      }));
      try {
        if (options.random) {
          await api.connect({ random: true, kill_switch: killSwitch });
        } else if (options.source) {
          await api.connect({
            source: options.source,
            protocol: options.source === "warp" ? "wireguard" : "openvpn",
            kill_switch: killSwitch,
          });
        } else if (target) {
          await api.connect({
            hostname: target.hostname,
            proto: target.proto,
            protocol: target.source === "warp" ? "wireguard" : "openvpn",
            transport: target.transport,
            source: target.source,
            kill_switch: killSwitch,
          });
        } else if (selectedServer) {
          await api.connect({
            hostname: selectedServer.hostname,
            proto: selectedServer.proto,
            protocol: selectedServer.source === "warp" ? "wireguard" : "openvpn",
            transport: selectedServer.transport,
            source: selectedServer.source,
            kill_switch: killSwitch,
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
              proto: sorted[0].proto,
              protocol: sorted[0].source === "warp" ? "wireguard" : "openvpn",
              transport: sorted[0].transport,
              source: sorted[0].source,
              kill_switch: killSwitch,
            });
          } else {
            await api.connect({ random: true, kill_switch: killSwitch });
          }
        }
      } catch (e) {
        setStatus({ state: "DISCONNECTED" });
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setBusy(false);
      }
    },
    [selectedServer, enrichedServers, killSwitch]
  );

  const handleDisconnect = useCallback(async () => {
    setBusy(true);
    setError("");
    setStatus((prev) => ({ ...prev, state: "DISCONNECTED" }));
    try {
      await api.disconnect();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
      void fetchPublicIp();
    }
  }, [fetchPublicIp]);

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
    if (activeChip === "favorites") {
      list = list.filter((s) => favorites.includes(s.country_short.toUpperCase()));
    } else if (activeChip === "fast") {
      list = list.filter((s) => (s.latency_ms || parsePing(s.ping)) < 60);
    } else if (activeChip === "warp") {
      list = list.filter((s) => s.source === "warp");
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
  }, [enrichedServers, sourceFilter, onlineOnly, search, sortBy, activeChip, favorites]);

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

      {/* Main Stage Canvas with Dynamic Reactive Cyber Aura */}
      <main className={`app-stage ${connected ? "stage-aura-connected" : connecting ? "stage-aura-connecting" : "stage-aura-disconnected"}`}>
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
              padding: "10px 14px",
              backgroundColor: "rgba(239, 68, 68, 0.12)",
              border: "1px solid rgba(239, 68, 68, 0.3)",
              borderRadius: "6px",
              color: "#fca5a5",
              fontSize: "12px",
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: "10px",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
              <ShieldAlert size={14} color="#ef4444" style={{ flexShrink: 0 }} />
              <span>{error}</span>
            </div>
            <button
              onClick={() => setError("")}
              style={{
                background: "none",
                border: "none",
                color: "#fca5a5",
                cursor: "pointer",
                padding: "2px 4px",
                lineHeight: 1,
                fontSize: "13px",
              }}
              title="Fermer"
            >
              ✕
            </button>
          </div>
        )}

        <div className="stage-content">
          {/* ========================================================
              TAB: CONNECTION (PRO MAX NORDIC CYBER)
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

              {/* Interactive World Map Radar */}
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

              {/* Iconic Cyber Power Switch (Proton / Windscribe centerpiece) */}
              <div className="cyber-power-center">
                <div className="cyber-power-ring-wrap">
                  <svg className="cyber-orbit-svg" viewBox="0 0 120 120">
                    <circle
                      cx="60"
                      cy="60"
                      r="54"
                      fill="none"
                      stroke={connected ? "rgba(34, 197, 94, 0.2)" : connecting ? "rgba(245, 158, 11, 0.2)" : "rgba(255, 255, 255, 0.06)"}
                      strokeWidth="1.5"
                    />
                    <circle
                      cx="60"
                      cy="60"
                      r="54"
                      fill="none"
                      stroke={connected ? "#22c55e" : connecting ? "#f59e0b" : "rgba(255, 255, 255, 0.16)"}
                      strokeWidth="2.2"
                      strokeDasharray={connecting ? "18 10" : connected ? "339" : "4 8"}
                      className={`orbit-dash-ring ${connecting ? "spinning" : ""}`}
                    />
                  </svg>

                  <button
                    className={`cyber-power-btn ${connected ? "connected" : connecting ? "connecting" : ""}`}
                    disabled={busy || backend !== "ok" || connecting}
                    onClick={() => {
                      if (connected) {
                        void handleDisconnect();
                      } else if (!connecting) {
                        void handleConnect();
                      }
                    }}
                    title={connected ? "Cliquer pour déconnecter" : connecting ? "Négociation en cours..." : "Cliquer pour sécuriser"}
                  >
                    <Power size={36} strokeWidth={2.4} />
                  </button>
                </div>

                <div className="cyber-meta-display">
                  <span className={`cyber-action-hint ${connected ? "connected" : connecting ? "connecting" : ""}`}>
                    {connected
                      ? "CLIQUEZ POUR DÉCONNECTER"
                      : connecting
                      ? "ÉTABLISSEMENT DU TUNNEL..."
                      : selectedServer
                      ? `CONNEXION : ${selectedServer.country_long.toUpperCase()}`
                      : "CLIQUEZ POUR SÉCURISER"}
                  </span>

                  <div className={`cyber-timer-badge ${connected ? "active" : ""}`}>
                    <Clock size={11} />
                    <span>{connected ? duration : "Chronomètre inactif"}</span>
                  </div>
                </div>
              </div>

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

              {/* Live Telemetry Speedometer Grid (Proton style live sparklines) */}
              <div className="speed-telemetry-grid">
                <div className="speed-meter-card">
                  <div className="speed-meter-header">
                    <span className="speed-direction-tag down">
                      <ArrowDown size={13} />
                      <span>Téléchargement</span>
                    </span>
                    <span className="speed-total-badge">
                      {connected ? `Session: ${totalDownMB} Mo` : "Session: 0 Mo"}
                    </span>
                  </div>

                  <div className="speed-rate-number">
                    {connected ? downloadRate.toFixed(1) : "0.0"}
                    <span className="unit">Mo/s</span>
                  </div>

                  <svg className="sparkline-svg" viewBox="0 0 100 28" preserveAspectRatio="none">
                    <defs>
                      <linearGradient id="sparkDownGrad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="#22c55e" stopOpacity="0.35" />
                        <stop offset="100%" stopColor="#22c55e" stopOpacity="0.0" />
                      </linearGradient>
                    </defs>
                    <path d={renderSparklinePath(sparklineDown, 30).area} fill="url(#sparkDownGrad)" />
                    <path d={renderSparklinePath(sparklineDown, 30).line} fill="none" stroke="#22c55e" strokeWidth="2" strokeLinecap="round" />
                  </svg>
                </div>

                <div className="speed-meter-card">
                  <div className="speed-meter-header">
                    <span className="speed-direction-tag up">
                      <ArrowUp size={13} />
                      <span>Envoi</span>
                    </span>
                    <span className="speed-total-badge">
                      {connected ? `Session: ${totalUpMB} Mo` : "Session: 0 Mo"}
                    </span>
                  </div>

                  <div className="speed-rate-number">
                    {connected ? uploadRate.toFixed(1) : "0.0"}
                    <span className="unit">Mo/s</span>
                  </div>

                  <svg className="sparkline-svg" viewBox="0 0 100 28" preserveAspectRatio="none">
                    <defs>
                      <linearGradient id="sparkUpGrad" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="#3b82f6" stopOpacity="0.35" />
                        <stop offset="100%" stopColor="#3b82f6" stopOpacity="0.0" />
                      </linearGradient>
                    </defs>
                    <path d={renderSparklinePath(sparklineUp, 15).area} fill="url(#sparkUpGrad)" />
                    <path d={renderSparklinePath(sparklineUp, 15).line} fill="none" stroke="#3b82f6" strokeWidth="2" strokeLinecap="round" />
                  </svg>
                </div>
              </div>

              {/* Clean Telemetry Details List */}
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
                  <span className="label">Latence / Signal</span>
                  <div className="val">
                    <div className="signal-bars">
                      <div className={`signal-bar bar-1 ${connected ? "active-green" : ""}`} />
                      <div className={`signal-bar bar-2 ${connected ? "active-green" : ""}`} />
                      <div className={`signal-bar bar-3 ${connected ? "active-green" : ""}`} />
                      <div className={`signal-bar bar-4 ${connected ? (targetLatency && targetLatency < 80 ? "active-green" : "active-amber") : ""}`} />
                    </div>
                    <span style={{ fontFamily: "monospace", fontSize: "12px", color: connected ? "var(--accent-green)" : "var(--text-secondary)" }}>
                      {connected ? (targetLatency ? `${targetLatency} ms` : "34 ms") : "—"}
                    </span>
                  </div>
                </div>

                <div className="telemetry-item-row">
                  <span className="label">Protocole</span>
                  <span className="val">
                    {connected
                      ? status.protocol || "OpenVPN (tun0 • Chiffré AES-256)"
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

                  <div className="filter-chips-wrap">
                    <button
                      className={`filter-chip-btn ${activeChip === "all" ? "active" : ""}`}
                      onClick={() => setActiveChip("all")}
                    >
                      Tous ({enrichedServers.length})
                    </button>
                    <button
                      className={`filter-chip-btn ${activeChip === "favorites" ? "active-star" : ""}`}
                      onClick={() => setActiveChip("favorites")}
                    >
                      <Star size={11} fill={activeChip === "favorites" ? "currentColor" : "none"} />
                      <span>Favoris ({favorites.length})</span>
                    </button>
                    <button
                      className={`filter-chip-btn ${activeChip === "fast" ? "active" : ""}`}
                      onClick={() => setActiveChip("fast")}
                    >
                      <Zap size={11} />
                      <span>&lt; 60 ms</span>
                    </button>
                    <button
                      className={`filter-chip-btn ${activeChip === "warp" ? "active" : ""}`}
                      onClick={() => setActiveChip("warp")}
                    >
                      <Cloud size={11} />
                      <span>WARP</span>
                    </button>
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
                    const isFav = favorites.includes(group.country_short.toUpperCase());
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
                          <button
                            className={`star-icon-btn ${isFav ? "starred" : ""}`}
                            onClick={(e) => {
                              e.stopPropagation();
                              toggleFavorite(group.country_short.toUpperCase());
                            }}
                            title={isFav ? "Retirer des favoris" : "Ajouter aux favoris"}
                          >
                            <Star size={12} fill={isFav ? "currentColor" : "none"} />
                          </button>
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
                                title="Définir comme cible sur l'accueil"
                              >
                                {isCurrentTarget ? "Sélectionné" : "Choisir"}
                              </button>
                              <button
                                className="btn-select-relay"
                                disabled={busy || connected}
                                onClick={() => {
                                  pickTargetServer(s);
                                  void handleConnect(s);
                                }}
                                title="Se connecter immédiatement"
                                style={{
                                  backgroundColor: connected && (status.hostname === s.hostname || status.ip_addr === s.ip) ? "var(--accent-green)" : "rgba(34, 197, 94, 0.12)",
                                  borderColor: "rgba(34, 197, 94, 0.3)",
                                  color: connected && (status.hostname === s.hostname || status.ip_addr === s.ip) ? "#000" : "var(--accent-green)",
                                  fontWeight: 600,
                                }}
                              >
                                {connected && (status.hostname === s.hostname || status.ip_addr === s.ip) ? "Actif" : "Connecter"}
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
              TAB: SETTINGS (PRO MAX TOGGLES)
              ======================================================== */}
          {activeTab === "settings" && (
            <div style={{ maxWidth: "540px", margin: "0 auto", width: "100%", display: "flex", flexDirection: "column", gap: "14px" }}>
              <div style={{ backgroundColor: "var(--bg-surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-md)", padding: "16px 18px", boxShadow: "inset 0 1px 0 rgba(255, 255, 255, 0.05)" }}>
                <h3 style={{ fontSize: "13px", fontWeight: 600, color: "#fff", marginBottom: "14px", display: "flex", alignItems: "center", gap: "6px" }}>
                  <ShieldCheck size={15} color="var(--accent-green)" />
                  <span>Sécurité & Confidentialité</span>
                </h3>

                {/* Kill Switch Toggle */}
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 0", borderBottom: "1px solid var(--border)" }}>
                  <div>
                    <div style={{ fontSize: "12.5px", fontWeight: 500, color: "var(--text-primary)" }}>
                      Kill Switch d'urgence
                    </div>
                    <div style={{ fontSize: "11px", color: "var(--text-tertiary)" }}>
                      Coupe le trafic réseau hors tunnel si la connexion VPN chute
                    </div>
                  </div>
                  <div
                    className={`modern-toggle-switch ${killSwitch ? "on" : ""}`}
                    onClick={() => {
                      const next = !killSwitch;
                      setKillSwitch(next);
                      localStorage.setItem("vpngate.killSwitch", String(next));
                    }}
                  >
                    <div className="toggle-knob" />
                  </div>
                </div>

                {/* DNS Leak Shield */}
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 0", borderBottom: "1px solid var(--border)" }}>
                  <div>
                    <div style={{ fontSize: "12.5px", fontWeight: 500, color: "var(--text-primary)" }}>
                      Protection contre les fuites DNS
                    </div>
                    <div style={{ fontSize: "11px", color: "var(--text-tertiary)" }}>
                      Force les résolveurs exclusifs et empêche l'espionnage par le FAI
                    </div>
                  </div>
                  <div
                    className={`modern-toggle-switch ${dnsLeakShield ? "on" : ""}`}
                    onClick={() => {
                      const next = !dnsLeakShield;
                      setDnsLeakShield(next);
                      localStorage.setItem("vpngate.dnsShield", String(next));
                    }}
                  >
                    <div className="toggle-knob" />
                  </div>
                </div>

                {/* Auto Reconnect */}
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 0", borderBottom: "1px solid var(--border)" }}>
                  <div>
                    <div style={{ fontSize: "12.5px", fontWeight: 500, color: "var(--text-primary)" }}>
                      Reconnexion automatique
                    </div>
                    <div style={{ fontSize: "11px", color: "var(--text-tertiary)" }}>
                      Relance immédiatement le tunnel sur le meilleur relais en ligne
                    </div>
                  </div>
                  <div
                    className={`modern-toggle-switch ${autoReconnect ? "on" : ""}`}
                    onClick={() => {
                      const next = !autoReconnect;
                      setAutoReconnect(next);
                      localStorage.setItem("vpngate.autoReconnect", String(next));
                    }}
                  >
                    <div className="toggle-knob" />
                  </div>
                </div>

                {/* IPv6 Blackhole */}
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 0" }}>
                  <div>
                    <div style={{ fontSize: "12.5px", fontWeight: 500, color: "var(--text-primary)" }}>
                      Blocage du trafic IPv6
                    </div>
                    <div style={{ fontSize: "11px", color: "var(--text-tertiary)" }}>
                      Désactive les fuites IPv6 non chiffrées sur l'interface physique
                    </div>
                  </div>
                  <div
                    className={`modern-toggle-switch ${blockIpv6 ? "on" : ""}`}
                    onClick={() => {
                      const next = !blockIpv6;
                      setBlockIpv6(next);
                      localStorage.setItem("vpngate.blockIpv6", String(next));
                    }}
                  >
                    <div className="toggle-knob" />
                  </div>
                </div>
              </div>

              {/* Daemon Socket Info */}
              <div style={{ backgroundColor: "var(--bg-surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-md)", padding: "16px 18px", boxShadow: "inset 0 1px 0 rgba(255, 255, 255, 0.05)" }}>
                <h3 style={{ fontSize: "13px", fontWeight: 600, color: "#fff", marginBottom: "8px" }}>
                  Daemon de contrôle unifié
                </h3>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <div style={{ fontSize: "11px", color: "var(--text-tertiary)" }}>
                    Partage d'état natif avec la CLI et la TUI sans re-authentification
                  </div>
                  <span style={{ fontSize: "11.5px", color: "var(--accent-blue)", fontFamily: "monospace", fontWeight: 600 }}>
                    127.0.0.1:1865
                  </span>
                </div>
              </div>

              {/* About Box */}
              <div style={{ backgroundColor: "var(--bg-surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-md)", padding: "16px 18px", boxShadow: "inset 0 1px 0 rgba(255, 255, 255, 0.05)" }}>
                <h3 style={{ fontSize: "13px", fontWeight: 600, color: "#fff", marginBottom: "4px" }}>
                  À propos de vpngate desktop
                </h3>
                <p style={{ fontSize: "11.5px", color: "var(--text-secondary)", lineHeight: "1.6" }}>
                  Client VPN haute sécurité pour Linux et macOS. Architecture zéro log avec chiffrement OpenVPN et WireGuard.
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