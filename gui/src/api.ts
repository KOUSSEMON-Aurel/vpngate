// Client HTTP du backend vpngate. Le desktop parle au sidecar local ;
// sur mobile, la même API pointe vers un daemon distant. La base URL se
// règle via localStorage (clé "vpngate.apiBase") pour le débogage et le
// futur mode mobile.

const DEFAULT_API = "http://127.0.0.1:1865";

export function apiBase(): string {
  return localStorage.getItem("vpngate.apiBase") ?? DEFAULT_API;
}

export interface ServerInfo {
  hostname: string;
  country_long: string;
  country_short: string;
  score: number;
  ip: string;
  ping: string;
  proto: string;
  transport?: string;
  source?: string;
  health?: "working" | "failed" | "checking" | "unknown";
  latency_ms?: number;
}

export interface StatusInfo {
  state: string;
  hostname?: string;
  ip_addr?: string;
  country?: string;
  started_at?: string;
  pid?: number;
  protocol?: string;
  transport?: string;
}

export interface ConnectRequest {
  hostname?: string;
  random?: boolean;
  protocol?: string;
  transport?: string;
  country?: string;
  source?: string;
  reconnect?: boolean;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(apiBase() + path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    let detail = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      if (typeof body?.error === "string") detail = body.error;
    } catch {
      // non-JSON error body; keep the status text
    }
    throw new Error(detail);
  }
  return (await res.json()) as T;
}

export const api = {
  health: () => request<{ ok: boolean }>("/api/health"),
  servers: (params: Record<string, string> = {}) => {
    const q = new URLSearchParams(params).toString();
    return request<ServerInfo[]>(`/api/servers${q ? `?${q}` : ""}`);
  },
  serversHealth: () =>
    request<Record<string, { status: "working" | "failed" | "checking" | "unknown"; latency_ms?: number }>>(
      "/api/servers/health"
    ),
  status: () => request<StatusInfo>("/api/status"),
  connect: (body: ConnectRequest) =>
    request<{ state: string }>("/api/connect", { method: "POST", body: JSON.stringify(body) }),
  disconnect: () =>
    request<{ state: string }>("/api/disconnect", { method: "POST" }),
  logs: (n = 400) => request<{ log: string }>(`/api/logs?n=${n}`),
};