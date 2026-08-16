import { useCallback, useEffect, useRef, useState } from "react";
import { api, ServerInfo, StatusInfo } from "./api";

const SOURCES = ["", "vpngate", "vpnbook", "warp"];
const TRANSPORTS = ["", "tcp443", "udp53", "tcp80", "udp25000"];

const STATE_LABEL: Record<string, string> = {
  DISCONNECTED: "Déconnecté",
  CONNECTING: "Connexion en cours…",
  CONNECTED: "Connecté",
  EXITING: "Fermeture…",
};

function statusText(status: StatusInfo): string {
  if (status.state === "DISCONNECTED" || !status.hostname) {
    return STATE_LABEL[status.state] ?? status.state;
  }
  const parts = [STATE_LABEL[status.state] ?? status.state, status.hostname];
  if (status.country) parts.push(status.country);
  if (status.ip_addr) parts.push(status.ip_addr);
  return parts.join(" · ");
}

export default function App() {
  const [backend, setBackend] = useState<"checking" | "ok" | "down">("checking");
  const [tab, setTab] = useState<"servers" | "logs">("servers");
  const [status, setStatus] = useState<StatusInfo>({ state: "DISCONNECTED" });
  const [servers, setServers] = useState<ServerInfo[]>([]);
  const [error, setError] = useState<string>("");
  const [busy, setBusy] = useState(false);

  const [source, setSource] = useState("");
  const [transport, setTransport] = useState("");
  const [country, setCountry] = useState("");
  const [refresh, setRefresh] = useState(false);

  // État de l'API (poll 2 s) et de la liste des serveurs.
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
        setStatus(await api.status());
      } catch {
        // backend mort entre deux polls ; le prochain tick le détectera
      }
    };
    void tick();
    const id = setInterval(tick, 2000);
    return () => clearInterval(id);
  }, []);

  const loadServers = useCallback(async () => {
    try {
      setError("");
      const params: Record<string, string> = {};
      if (source) params.source = source;
      if (transport) params.transport = transport;
      if (country) params.country = country;
      if (refresh) params.refresh = "1";
      setServers(await api.servers(params));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [source, transport, country, refresh]);

  useEffect(() => {
    if (backend === "ok") void loadServers();
  }, [backend, loadServers]);

  const connect = useCallback(async (body: Parameters<typeof api.connect>[0]) => {
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

  const disconnect = useCallback(async () => {
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

  const connected = status.state !== "DISCONNECTED";

  return (
    <div className="app">
      <header>
        <h1>vpngate</h1>
        <div className={`status status-${status.state.toLowerCase()}`} title="État du tunnel">
          <span className="dot" />
          {statusText(status)}
        </div>
        <div className={`backend backend-${backend}`} title="Backend GUI (api vpngate serve)">
          {backend === "checking" && "backend…"}
          {backend === "ok" && "backend ok"}
          {backend === "down" && "backend injoignable"}
        </div>
        {connected && (
          <button className="danger" disabled={busy} onClick={() => void disconnect()}>
            Déconnecter
          </button>
        )}
      </header>

      {backend === "down" && (
        <p className="notice">
          Le backend ne répond pas. Sur desktop il est lancé automatiquement ; sinon, lancez
          <code> vpngate serve</code>.
        </p>
      )}

      <nav>
        <button className={tab === "servers" ? "tab active" : "tab"} onClick={() => setTab("servers")}>
          Serveurs
        </button>
        <button className={tab === "logs" ? "tab active" : "tab"} onClick={() => setTab("logs")}>
          Logs
        </button>
      </nav>

      {error && <p className="notice error">{error}</p>}

      {tab === "servers" ? (
        <>
          <div className="filters">
            <select value={source} onChange={(e) => setSource(e.target.value)} aria-label="Source">
              <option value="">Toutes les sources</option>
              {SOURCES.filter(Boolean).map((s) => (
                <option key={s} value={s}>{s}</option>
              ))}
            </select>
            <select value={transport} onChange={(e) => setTransport(e.target.value)} aria-label="Transport">
              <option value="">Tous les transports</option>
              {TRANSPORTS.filter(Boolean).map((t) => (
                <option key={t} value={t}>{t}</option>
              ))}
            </select>
            <input
              value={country}
              onChange={(e) => setCountry(e.target.value)}
              placeholder="Pays (ex: japan, us)"
              aria-label="Pays"
            />
            <label className="check">
              <input type="checkbox" checked={refresh} onChange={(e) => setRefresh(e.target.checked)} />
              rafraîchir la liste
            </label>
            <button className="primary" onClick={() => void loadServers()} disabled={backend !== "ok"}>
              Actualiser
            </button>
            <button
              className="primary"
              disabled={busy || backend !== "ok"}
              onClick={() => void connect({ random: true, source: source || undefined, transport: transport || undefined, country: country || undefined })}
            >
              Aléatoire
            </button>
          </div>

          <table>
            <thead>
              <tr>
                <th>Serveur</th>
                <th>Pays</th>
                <th>IP</th>
                <th>Proto</th>
                <th>Transport</th>
                <th>Source</th>
                <th className="num">Score</th>
                <th className="num">Ping</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {servers.map((s) => (
                <tr key={s.hostname}>
                  <td>{s.hostname}</td>
                  <td>{s.country_long}</td>
                  <td>{s.ip}</td>
                  <td>{s.proto || "-"}</td>
                  <td>{s.transport || "-"}</td>
                  <td>{s.source || "-"}</td>
                  <td className="num">{s.score}</td>
                  <td className="num">{s.ping}</td>
                  <td className="actions">
                    <button
                      disabled={busy || backend !== "ok" || connected}
                      title={connected ? "Déconnectez-vous d'abord" : `Se connecter à ${s.hostname}`}
                      onClick={() =>
                        void connect({
                          hostname: s.hostname,
                          protocol: "openvpn",
                          transport: s.transport || undefined,
                          source: s.source || undefined,
                        })
                      }
                    >
                      Connecter
                    </button>
                  </td>
                </tr>
              ))}
              {servers.length === 0 && (
                <tr>
                  <td colSpan={9} className="empty">
                    Aucun serveur trouvé.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </>
      ) : (
        <Logs backend={backend} />
      )}
    </div>
  );
}

function Logs({ backend }: { backend: string }) {
  const [log, setLog] = useState("");
  const [auto, setAuto] = useState(true);
  const preRef = useRef<HTMLPreElement>(null);

  useEffect(() => {
    if (backend !== "ok" || !auto) return;
    const tick = async () => {
      try {
        setLog((await api.logs()).log);
      } catch {
        // backend mort ; la barre d'état le signale déjà
      }
    };
    void tick();
    const id = setInterval(tick, 3000);
    return () => clearInterval(id);
  }, [backend, auto]);

  useEffect(() => {
    const el = preRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [log]);

  return (
    <div className="logs">
      <label className="check">
        <input type="checkbox" checked={auto} onChange={(e) => setAuto(e.target.checked)} />
        actualisation automatique
      </label>
      <pre ref={preRef}>{log || "Aucun log pour l'instant."}</pre>
    </div>
  );
}