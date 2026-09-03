import React, { useMemo, useState } from "react";
import countryShapes from "world-map-country-shapes";
import countryCentersData from "./country_centers.json";
import { ServerInfo } from "./api";

const countryCenters = countryCentersData as Record<string, { x: number; y: number }>;

interface WorldMapProps {
  servers: ServerInfo[];
  selectedCountry?: string;
  connectedCountry?: string;
  isConnected: boolean;
  onSelectCountry: (code: string) => void;
}

export const WorldMap: React.FC<WorldMapProps> = ({
  servers,
  selectedCountry,
  connectedCountry,
  isConnected,
  onSelectCountry,
}) => {
  const [hoveredCountry, setHoveredCountry] = useState<string | null>(null);

  // Group servers by country code
  const countryServerMap = useMemo(() => {
    const map = new Map<
      string,
      { code: string; count: number; working: number; bestPing: number; name: string }
    >();

    for (const s of servers) {
      const code = s.country_short.toUpperCase();
      const existing = map.get(code);
      const isUp = s.health === "working";
      const ping = s.latency_ms || parseInt(s.ping, 10) || 999;

      if (!existing) {
        map.set(code, {
          code,
          count: 1,
          working: isUp ? 1 : 0,
          bestPing: ping,
          name: s.country_long,
        });
      } else {
        existing.count++;
        if (isUp) existing.working++;
        if (ping < existing.bestPing) existing.bestPing = ping;
      }
    }
    return map;
  }, [servers]);

  // Active target country code
  const activeCode = (connectedCountry || selectedCountry || "JP").toUpperCase();
  const targetPos = countryCenters[activeCode] || countryCenters["JP"] || { x: 1693, y: 342 };

  // Origin (User local position: France default center x: 985, y: 275)
  const localPos = countryCenters["FR"] || { x: 985, y: 275 };

  // Arc control point for nice natural curve
  const arcControlY = Math.min(localPos.y, targetPos.y) - 100;
  const arcControlX = (localPos.x + targetPos.x) / 2;

  return (
    <div className="world-map-wrapper">
      <svg
        viewBox="0 0 2000 950"
        className="world-map-svg"
        preserveAspectRatio="xMidYMid meet"
      >
        <defs>
          {/* Subtle Grid Lines */}
          <pattern id="world-grid" width="80" height="80" patternUnits="userSpaceOnUse">
            <path
              d="M 80 0 L 0 0 0 80"
              fill="none"
              stroke="rgba(255, 255, 255, 0.02)"
              strokeWidth="1"
            />
          </pattern>

          {/* Connection Arc Gradient */}
          <linearGradient id="mapArcGrad" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="#3b82f6" stopOpacity="0.85" />
            <stop offset="100%" stopColor="#22c55e" stopOpacity="0.95" />
          </linearGradient>

          {/* Soft Glow */}
          <filter id="pinGlow" x="-30%" y="-30%" width="160%" height="160%">
            <feGaussianBlur stdDeviation="4" result="blur" />
            <feComposite in="SourceGraphic" in2="blur" operator="over" />
          </filter>
        </defs>

        {/* Ocean Background with grid */}
        <rect width="2000" height="950" fill="#0b0d12" />
        <rect width="2000" height="950" fill="url(#world-grid)" />

        {/* Real Detailed Vector Countries */}
        <g className="all-countries-layer">
          {countryShapes.map((c) => {
            const hasServers = countryServerMap.has(c.id);
            const isTarget = activeCode === c.id;
            const isHovered = hoveredCountry === c.id;

            let fill = "#13161f";
            let stroke = "#1c222e";
            let strokeWidth = 0.8;

            if (hasServers) {
              fill = "#181d28";
              stroke = "#293245";
            }
            if (isHovered) {
              fill = "#242c3d";
              stroke = "#4a5978";
              strokeWidth = 1.2;
            }
            if (isTarget) {
              fill = isConnected ? "rgba(34, 197, 94, 0.2)" : "#2c364c";
              stroke = isConnected ? "#22c55e" : "#e4e4e7";
              strokeWidth = 1.4;
            }

            return (
              <path
                key={c.id}
                d={c.shape}
                fill={fill}
                stroke={stroke}
                strokeWidth={strokeWidth}
                style={{
                  cursor: hasServers ? "pointer" : "default",
                  transition: "fill 0.15s ease, stroke 0.15s ease",
                }}
                onClick={() => {
                  if (hasServers) onSelectCountry(c.id);
                }}
                onMouseEnter={() => {
                  if (hasServers) setHoveredCountry(c.id);
                }}
                onMouseLeave={() => setHoveredCountry(null)}
              />
            );
          })}
        </g>

        {/* Active Animated Connection Arc */}
        {isConnected && (
          <g className="active-connection-layer">
            <path
              d={`M ${localPos.x} ${localPos.y} Q ${arcControlX} ${arcControlY} ${targetPos.x} ${targetPos.y}`}
              fill="none"
              stroke="url(#mapArcGrad)"
              strokeWidth="3"
              strokeLinecap="round"
              strokeDasharray="8 6"
              className="animated-arc-path"
              filter="url(#pinGlow)"
            />

            {/* Local Client Node */}
            <circle cx={localPos.x} cy={localPos.y} r="5" fill="#3b82f6" />
            <circle cx={localPos.x} cy={localPos.y} r="10" fill="none" stroke="#3b82f6" opacity="0.4" />
          </g>
        )}

        {/* Fixed Markers / Pins for Countries with Servers (NO CSS SCALE = NO JUMPING) */}
        <g className="markers-layer">
          {Array.from(countryServerMap.values()).map((c) => {
            const pos = countryCenters[c.code];
            if (!pos) return null;

            const isTarget = activeCode === c.code;
            const isHovered = hoveredCountry === c.code;
            const isUp = c.working > 0;

            const pinColor = isTarget
              ? isConnected
                ? "#22c55e"
                : "#ffffff"
              : isUp
              ? "#22c55e"
              : "#ef4444";

            return (
              <g
                key={c.code}
                style={{ cursor: "pointer" }}
                onClick={() => onSelectCountry(c.code)}
                onMouseEnter={() => setHoveredCountry(c.code)}
                onMouseLeave={() => setHoveredCountry(null)}
              >
                {/* Target Pulsing Beacon (Anchored at exact pos.x, pos.y) */}
                {isTarget && (
                  <circle
                    cx={pos.x}
                    cy={pos.y}
                    r="16"
                    fill="none"
                    stroke={isConnected ? "#22c55e" : "#ffffff"}
                    strokeWidth="2"
                    className="pulsing-beacon"
                    opacity="0.8"
                  />
                )}

                {/* Outer halo on hover */}
                {isHovered && !isTarget && (
                  <circle
                    cx={pos.x}
                    cy={pos.y}
                    r="12"
                    fill="none"
                    stroke={pinColor}
                    strokeWidth="1.5"
                    opacity="0.5"
                  />
                )}

                {/* Base Anchor Circle - Fixed position cx/cy, only radius changes */}
                <circle
                  cx={pos.x}
                  cy={pos.y}
                  r={isTarget ? 7 : isHovered ? 6.5 : 5}
                  fill={pinColor}
                  stroke="#080a0f"
                  strokeWidth="2"
                  filter={isTarget ? "url(#pinGlow)" : undefined}
                />

                {/* Interactive Tooltip Card Above Pin */}
                {(isHovered || isTarget) && (
                  <g
                    transform={`translate(${pos.x}, ${pos.y - 20})`}
                    style={{ pointerEvents: "none" }}
                  >
                    <rect
                      x="-55"
                      y="-26"
                      width="110"
                      height="26"
                      rx="5"
                      fill="#141722"
                      stroke="rgba(255, 255, 255, 0.2)"
                      strokeWidth="1"
                    />
                    <text
                      x="0"
                      y="-9"
                      textAnchor="middle"
                      fill="#ffffff"
                      fontSize="11.5"
                      fontFamily="Inter, sans-serif"
                      fontWeight="600"
                    >
                      {c.code} • {c.name.length > 10 ? c.name.slice(0, 9) + "…" : c.name}
                    </text>
                    <text
                      x="0"
                      y="-1"
                      textAnchor="middle"
                      fill={isUp ? "#22c55e" : "#ef4444"}
                      fontSize="9"
                      fontFamily="JetBrains Mono, monospace"
                      fontWeight="500"
                    >
                      {isUp ? `${c.bestPing}ms • ${c.working} en ligne` : "Hors ligne"}
                    </text>
                  </g>
                )}
              </g>
            );
          })}
        </g>
      </svg>

      {/* Map Legend Overlay */}
      <div className="map-legend-overlay">
        <span className="legend-item">
          <span className="legend-marker green" />
          <span>Serveurs en ligne</span>
        </span>
        <span className="legend-item">
          <span className="legend-marker red" />
          <span>Inaccessible</span>
        </span>
        {isConnected && (
          <span className="legend-item connected-tag">
            <span>● Connecté au relais</span>
          </span>
        )}
      </div>
    </div>
  );
};
