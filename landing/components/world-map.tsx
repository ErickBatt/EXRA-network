"use client";

import * as React from "react";
import {
  ComposableMap,
  Geographies,
  Geography,
  Marker,
  Line,
} from "react-simple-maps";
import { motion } from "framer-motion";

/**
 * WorldMap — the hero centerpiece.
 *
 * Visual goals:
 *   1. Quietly dark continents (no ocean fill, no graticule) — they recede.
 *   2. Pulsing neon pins on EXRA's launch corridor: India, SEA, LATAM, Africa.
 *   3. Faint connection arcs between hub pairs — implies a working mesh.
 *   4. Subtle parallax on pointer move — depth without gimmickry.
 *
 * The TopoJSON is loaded from jsdelivr (Natural Earth 110m, ~100KB cached).
 * We avoid bundling it locally so we don't ship 100KB of geo to every user
 * — the browser caches the CDN copy across sites.
 */

const GEO_URL =
  "https://cdn.jsdelivr.net/npm/world-atlas@2/countries-110m.json";

type Node = {
  id: string;
  name: string;
  coords: [number, number]; // [lng, lat]
  size?: "sm" | "md" | "lg";
  delay?: number;
};

// EXRA launch corridor — India, SEA, LATAM, Africa
const NODES: Node[] = [
  { id: "in", name: "Mumbai, IN", coords: [72.87, 19.07], size: "lg", delay: 0 },
  { id: "in2", name: "Bangalore, IN", coords: [77.59, 12.97], size: "md", delay: 0.6 },
  { id: "id", name: "Jakarta, ID", coords: [106.84, -6.21], size: "lg", delay: 0.3 },
  { id: "vn", name: "Ho Chi Minh, VN", coords: [106.66, 10.82], size: "md", delay: 0.9 },
  { id: "ph", name: "Manila, PH", coords: [120.98, 14.6], size: "md", delay: 0.4 },
  { id: "th", name: "Bangkok, TH", coords: [100.5, 13.75], size: "sm", delay: 1.1 },
  { id: "pk", name: "Karachi, PK", coords: [67.0, 24.86], size: "md", delay: 0.7 },
  { id: "ng", name: "Lagos, NG", coords: [3.38, 6.52], size: "lg", delay: 0.2 },
  { id: "ke", name: "Nairobi, KE", coords: [36.82, -1.29], size: "sm", delay: 1.0 },
  { id: "br", name: "São Paulo, BR", coords: [-46.63, -23.55], size: "lg", delay: 0.5 },
  { id: "mx", name: "Mexico City, MX", coords: [-99.13, 19.43], size: "md", delay: 0.8 },
  { id: "ar", name: "Buenos Aires, AR", coords: [-58.38, -34.6], size: "sm", delay: 1.2 },
  { id: "co", name: "Bogotá, CO", coords: [-74.07, 4.71], size: "sm", delay: 0.4 },
  { id: "eg", name: "Cairo, EG", coords: [31.24, 30.04], size: "sm", delay: 0.9 },
];

// Connection arcs — hub-to-hub shimmer to imply a working mesh.
// Pick a sparse set so it reads as "network" not "noise".
const ARCS: Array<[Node, Node]> = [
  [NODES[0], NODES[2]], // Mumbai → Jakarta
  [NODES[2], NODES[9]], // Jakarta → São Paulo
  [NODES[7], NODES[10]], // Lagos → Mexico City
  [NODES[0], NODES[7]], // Mumbai → Lagos
  [NODES[9], NODES[10]], // São Paulo → Mexico
  [NODES[0], NODES[3]], // Mumbai → HCMC
];

const sizeMap = {
  sm: { dot: 2.5, ring: 14 },
  md: { dot: 3.5, ring: 18 },
  lg: { dot: 4.5, ring: 24 },
};

interface WorldMapProps {
  /** Disable the parallax effect (useful for low-power scenarios). */
  staticView?: boolean;
}

export function WorldMap({ staticView = false }: WorldMapProps) {
  const containerRef = React.useRef<HTMLDivElement>(null);
  const [tilt, setTilt] = React.useState({ x: 0, y: 0 });

  const handleMouseMove = React.useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      if (staticView) return;
      const rect = containerRef.current?.getBoundingClientRect();
      if (!rect) return;
      const px = (e.clientX - rect.left) / rect.width - 0.5;
      const py = (e.clientY - rect.top) / rect.height - 0.5;
      // Capped tilt — subtle, never disorienting
      setTilt({ x: py * -4, y: px * 4 });
    },
    [staticView]
  );

  const handleMouseLeave = React.useCallback(() => {
    setTilt({ x: 0, y: 0 });
  }, []);

  return (
    <div
      ref={containerRef}
      onMouseMove={handleMouseMove}
      onMouseLeave={handleMouseLeave}
      className="relative w-full aspect-[16/10] select-none"
      style={{ perspective: "1200px" }}
    >
      {/* Backdrop glow — lives behind map for depth */}
      <div
        aria-hidden
        className="absolute inset-0 -z-10"
        style={{
          background:
            "radial-gradient(ellipse 70% 50% at 50% 50%, rgba(34,211,238,0.16) 0%, rgba(167,139,250,0.08) 35%, transparent 70%)",
          filter: "blur(40px)",
        }}
      />

      <motion.div
        className="w-full h-full"
        style={{
          transformStyle: "preserve-3d",
        }}
        animate={{ rotateX: tilt.x, rotateY: tilt.y }}
        transition={{ type: "spring", stiffness: 80, damping: 20, mass: 0.5 }}
      >
        <ComposableMap
          projectionConfig={{ scale: 165 }}
          projection="geoEqualEarth"
          width={980}
          height={520}
          style={{ width: "100%", height: "100%" }}
        >
          {/* Continents — quiet, no fill, hairline stroke */}
          <Geographies geography={GEO_URL}>
            {({ geographies }) =>
              geographies.map((geo) => (
                <Geography
                  key={geo.rsmKey}
                  geography={geo}
                  style={{
                    default: {
                      fill: "rgba(255,255,255,0.025)",
                      stroke: "rgba(255,255,255,0.08)",
                      strokeWidth: 0.5,
                      outline: "none",
                    },
                    hover: {
                      fill: "rgba(34,211,238,0.04)",
                      stroke: "rgba(34,211,238,0.18)",
                      strokeWidth: 0.5,
                      outline: "none",
                    },
                    pressed: { outline: "none" },
                  }}
                />
              ))
            }
          </Geographies>

          {/* Connection arcs — drawn before markers so pins sit on top */}
          {ARCS.map(([a, b], i) => (
            <Line
              key={`arc-${i}`}
              from={a.coords}
              to={b.coords}
              stroke="url(#arcGrad)"
              strokeWidth={0.7}
              strokeLinecap="round"
              strokeDasharray="2 4"
              fill="none"
            />
          ))}

          {/* Gradient defs for arcs */}
          <defs>
            <linearGradient id="arcGrad" x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stopColor="#22d3ee" stopOpacity="0.6" />
              <stop offset="100%" stopColor="#a78bfa" stopOpacity="0.6" />
            </linearGradient>
            <radialGradient id="dotGrad">
              <stop offset="0%" stopColor="#67e8f9" />
              <stop offset="100%" stopColor="#22d3ee" />
            </radialGradient>
          </defs>

          {/* Pulsing node markers */}
          {NODES.map((node) => {
            const dim = sizeMap[node.size ?? "md"];
            return (
              <Marker key={node.id} coordinates={node.coords}>
                {/* Outer expanding ring — CSS animation, GPU-cheap */}
                <circle
                  r={dim.ring}
                  fill="none"
                  stroke="#22d3ee"
                  strokeWidth={1}
                  opacity={0.5}
                  className="pulse-ring"
                  style={{ animationDelay: `${node.delay ?? 0}s` }}
                />
                <circle
                  r={dim.ring}
                  fill="none"
                  stroke="#a78bfa"
                  strokeWidth={1}
                  opacity={0.4}
                  className="pulse-ring-delayed"
                  style={{ animationDelay: `${(node.delay ?? 0) + 0.6}s` }}
                />
                {/* Soft halo — static glow under the dot */}
                <circle
                  r={dim.dot * 2.4}
                  fill="#22d3ee"
                  opacity={0.18}
                  filter="blur(4px)"
                />
                {/* Core dot */}
                <circle
                  r={dim.dot}
                  fill="url(#dotGrad)"
                  stroke="#ffffff"
                  strokeWidth={0.4}
                  strokeOpacity={0.6}
                />
              </Marker>
            );
          })}
        </ComposableMap>
      </motion.div>

      {/* Floating data chips (overlay) — hard-coded selection of 3 hubs */}
      <FloatingChip
        position={{ top: "18%", left: "62%" }}
        label="Jakarta"
        sub="2,481 nodes online"
        delay={0.2}
      />
      <FloatingChip
        position={{ top: "44%", left: "48%" }}
        label="Mumbai"
        sub="3,920 nodes online"
        delay={0.5}
      />
      <FloatingChip
        position={{ top: "62%", left: "26%" }}
        label="São Paulo"
        sub="1,108 nodes online"
        delay={0.9}
      />
    </div>
  );
}

function FloatingChip({
  position,
  label,
  sub,
  delay = 0,
}: {
  position: React.CSSProperties;
  label: string;
  sub: string;
  delay?: number;
}) {
  return (
    <motion.div
      className="absolute pointer-events-none hidden sm:block"
      style={position}
      initial={{ opacity: 0, y: 12 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.6, delay: 0.8 + delay, ease: [0.22, 1, 0.36, 1] }}
    >
      <div className="glass-elevated rounded-xl px-3 py-2 flex items-center gap-2.5 min-w-[140px]">
        <span className="relative flex h-2 w-2">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-neon opacity-60" />
          <span className="relative inline-flex rounded-full h-2 w-2 bg-neon-bright" />
        </span>
        <div className="flex flex-col">
          <span className="text-[11px] font-mono uppercase tracking-wider text-ink-muted leading-none">
            {label}
          </span>
          <span className="text-xs font-medium text-ink leading-tight mt-0.5">{sub}</span>
        </div>
      </div>
    </motion.div>
  );
}
