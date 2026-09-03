import React, { useMemo, useState } from "react";
import { ServerInfo } from "./api";

// Approximate coordinates for world countries
export const COUNTRY_COORDS: Record<string, { lat: number; lon: number; name: string }> = {
  JP: { lat: 36.2, lon: 138.2, name: "Japon" },
  US: { lat: 37.1, lon: -95.7, name: "États-Unis" },
  CA: { lat: 56.1, lon: -106.3, name: "Canada" },
  GB: { lat: 55.3, lon: -3.4, name: "Royaume-Uni" },
  FR: { lat: 46.2, lon: 2.2, name: "France" },
  DE: { lat: 51.1, lon: 10.4, name: "Allemagne" },
  NL: { lat: 52.1, lon: 5.2, name: "Pays-Bas" },
  KR: { lat: 35.9, lon: 127.7, name: "Corée du Sud" },
  TW: { lat: 23.6, lon: 120.9, name: "Taïwan" },
  HK: { lat: 22.3, lon: 114.1, name: "Hong Kong" },
  SG: { lat: 1.3, lon: 103.8, name: "Singapour" },
  AU: { lat: -25.2, lon: 133.7, name: "Australie" },
  IN: { lat: 20.5, lon: 78.9, name: "Inde" },
  RU: { lat: 61.5, lon: 105.3, name: "Russie" },
  BR: { lat: -14.2, lon: -51.9, name: "Brésil" },
  AR: { lat: -38.4, lon: -63.6, name: "Argentine" },
  MX: { lat: 23.6, lon: -102.5, name: "Mexique" },
  ZA: { lat: -30.5, lon: 22.9, name: "Afrique du Sud" },
  CH: { lat: 46.8, lon: 8.2, name: "Suisse" },
  SE: { lat: 60.1, lon: 18.6, name: "Suède" },
  NO: { lat: 60.4, lon: 8.4, name: "Norvège" },
  FI: { lat: 61.9, lon: 25.7, name: "Finlande" },
  PL: { lat: 51.9, lon: 19.1, name: "Pologne" },
  IT: { lat: 41.8, lon: 12.5, name: "Italie" },
  ES: { lat: 40.4, lon: -3.7, name: "Espagne" },
  TR: { lat: 38.9, lon: 35.2, name: "Turquie" },
  UA: { lat: 48.3, lon: 31.1, name: "Ukraine" },
  RO: { lat: 45.9, lon: 24.9, name: "Roumanie" },
  TH: { lat: 15.8, lon: 100.9, name: "Thaïlande" },
  VN: { lat: 14.0, lon: 108.2, name: "Vietnam" },
  ID: { lat: -0.7, lon: 113.9, name: "Indonésie" },
  MY: { lat: 4.2, lon: 101.9, name: "Malaisie" },
  NZ: { lat: -40.9, lon: 174.8, name: "Nouvelle-Zélande" },
};

// Convert Lat/Lon to SVG viewBox (1000 x 500)
function project(lat: number, lon: number): { x: number; y: number } {
  // Clamped equirectangular projection
  const x = ((lon + 180) * 1000) / 360;
  // Latitude clamped from -60 to 80
  const clampedLat = Math.max(-60, Math.min(80, lat));
  const y = ((85 - clampedLat) * 500) / 145;
  return { x, y };
}

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

  // Group servers by country code with live working status count
  const countryData = useMemo(() => {
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
    return Array.from(map.values());
  }, [servers]);

  // Destination node position
  const activeCode = connectedCountry || selectedCountry || "JP";
  const targetCoords = COUNTRY_COORDS[activeCode.toUpperCase()] || COUNTRY_COORDS["JP"];
  const targetPos = project(targetCoords.lat, targetCoords.lon);

  // Origin (User local position - default France / Europe center)
  const localPos = project(46.5, 2.5);

  return (
    <div className="world-map-wrapper">
      <svg
        viewBox="0 0 1000 500"
        className="world-map-svg"
        preserveAspectRatio="xMidYMid meet"
      >
        <defs>
          {/* Subtle Grid Pattern */}
          <pattern id="grid" width="40" height="40" patternUnits="userSpaceOnUse">
            <path
              d="M 40 0 L 0 0 0 40"
              fill="none"
              stroke="rgba(255, 255, 255, 0.025)"
              strokeWidth="1"
            />
          </pattern>

          {/* Connection Arc Gradient */}
          <linearGradient id="arcGrad" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="#3b82f6" stopOpacity="0.8" />
            <stop offset="100%" stopColor="#22c55e" stopOpacity="0.9" />
          </linearGradient>

          {/* Glow filter */}
          <filter id="glow" x="-20%" y="-20%" width="140%" height="140%">
            <feGaussianBlur stdDeviation="3" result="blur" />
            <feComposite in="SourceGraphic" in2="blur" operator="over" />
          </filter>
        </defs>

        {/* Ocean Background & Grid */}
        <rect width="1000" height="500" fill="transparent" />
        <rect width="1000" height="500" fill="url(#grid)" />

        {/* Continents Simplified Geometric Silhouettes */}
        <g className="continents-group" fill="#15171f" stroke="#222633" strokeWidth="0.8">
          {/* North America */}
          <path d="M 120 70 L 260 65 L 310 95 L 290 150 L 250 170 L 230 220 L 200 240 L 160 210 L 130 160 L 100 110 Z" />
          <path d="M 280 40 L 360 45 L 340 90 L 290 85 Z" /> {/* Greenland */}
          
          {/* South America */}
          <path d="M 270 260 L 350 270 L 380 340 L 340 430 L 290 410 L 260 330 Z" />
          
          {/* Europe */}
          <path d="M 450 80 L 550 75 L 560 130 L 510 160 L 460 150 L 440 110 Z" />
          <path d="M 430 90 L 460 85 L 450 120 L 430 115 Z" /> {/* British Isles */}
          <path d="M 490 50 L 540 50 L 520 100 L 480 80 Z" /> {/* Scandinavia */}

          {/* Africa */}
          <path d="M 450 170 L 560 175 L 590 240 L 560 340 L 500 370 L 460 280 L 430 210 Z" />
          <path d="M 590 310 L 610 320 L 600 360 L 585 345 Z" /> {/* Madagascar */}

          {/* Asia */}
          <path d="M 560 70 L 820 65 L 890 120 L 850 180 L 760 210 L 710 240 L 670 210 L 610 170 L 560 130 Z" />
          <path d="M 680 210 L 740 215 L 710 290 L 670 250 Z" /> {/* India */}
          <path d="M 770 220 L 820 230 L 800 280 L 760 260 Z" /> {/* SE Asia */}
          <path d="M 830 130 L 860 140 L 850 190 L 820 170 Z" /> {/* Japan Arc */}

          {/* Australia & Oceania */}
          <path d="M 790 320 L 890 325 L 880 400 L 800 395 Z" />
          <path d="M 890 410 L 920 415 L 910 445 L 885 435 Z" /> {/* New Zealand */}
        </g>

        {/* Active Connection Arc Line */}
        {isConnected && (
          <g className="active-connection-arc">
            {/* Curved bezier arc from origin to target */}
            <path
              d={`M ${localPos.x} ${localPos.y} Q ${(localPos.x + targetPos.x) / 2} ${
                Math.min(localPos.y, targetPos.y) - 60
              } ${targetPos.x} ${targetPos.y}`}
              fill="none"
              stroke="url(#arcGrad)"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeDasharray="6 4"
              className="animated-arc-path"
              filter="url(#glow)"
            />

            {/* Local Origin Ping */}
            <circle cx={localPos.x} cy={localPos.y} r="4" fill="#3b82f6" />
            <circle cx={localPos.x} cy={localPos.y} r="8" fill="none" stroke="#3b82f6" opacity="0.4" />
          </g>
        )}

        {/* Server Country Markers on Map */}
        {countryData.map((c) => {
          const coords = COUNTRY_COORDS[c.code];
          if (!coords) return null;
          const pos = project(coords.lat, coords.lon);

          const isTarget =
            selectedCountry?.toUpperCase() === c.code ||
            connectedCountry?.toUpperCase() === c.code;

          const isHovered = hoveredCountry === c.code;
          const hasWorking = c.working > 0;

          return (
            <g
              key={c.code}
              className={`map-pin-group ${isTarget ? "active-target" : ""}`}
              onClick={() => onSelectCountry(c.code)}
              onMouseEnter={() => setHoveredCountry(c.code)}
              onMouseLeave={() => setHoveredCountry(null)}
              style={{ cursor: "pointer" }}
            >
              {/* Outer Pulse for selected or connected node */}
              {isTarget && (
                <circle
                  cx={pos.x}
                  cy={pos.y}
                  r="12"
                  fill="none"
                  stroke={isConnected ? "#22c55e" : "#fafafa"}
                  strokeWidth="1.5"
                  className="pulsing-beacon"
                  opacity="0.7"
                />
              )}

              {/* Pin Base Circle */}
              <circle
                cx={pos.x}
                cy={pos.y}
                r={isTarget ? "6" : isHovered ? "5.5" : "4"}
                fill={
                  isTarget
                    ? isConnected
                      ? "#22c55e"
                      : "#ffffff"
                    : hasWorking
                    ? "#22c55e"
                    : "#ef4444"
                }
                stroke="#090a0d"
                strokeWidth="1.5"
                filter={isTarget ? "url(#glow)" : undefined}
                className="pin-center-dot"
              />

              {/* Node ISO label (visible on hover or target) */}
              {(isHovered || isTarget) && (
                <g
                  transform={`translate(${pos.x}, ${pos.y - 14})`}
                  className="map-tooltip-group"
                >
                  <rect
                    x="-32"
                    y="-18"
                    width="64"
                    height="18"
                    rx="4"
                    fill="#171922"
                    stroke="rgba(255, 255, 255, 0.15)"
                    strokeWidth="1"
                  />
                  <text
                    x="0"
                    y="-5"
                    textAnchor="middle"
                    fill="#ffffff"
                    fontSize="9.5"
                    fontFamily="Inter, sans-serif"
                    fontWeight="600"
                  >
                    {c.code} • {c.bestPing < 900 ? `${c.bestPing}ms` : `${c.count}`}
                  </text>
                </g>
              )}
            </g>
          );
        })}
      </svg>

      {/* Map Legend Overlay */}
      <div className="map-legend-overlay">
        <span className="legend-item">
          <span className="legend-marker green" />
          <span>En ligne</span>
        </span>
        <span className="legend-item">
          <span className="legend-marker red" />
          <span>Inaccessible</span>
        </span>
        {isConnected && (
          <span className="legend-item connected-tag">
            <span>● Tunnel Chiffré Actif</span>
          </span>
        )}
      </div>
    </div>
  );
};
