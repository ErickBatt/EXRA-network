'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import './LiveMap.css';

const COUNTRY_LATLNG: Record<string, [number, number]> = {
  US: [39.82, -98.58], CA: [56.13, -106.34], MX: [23.63, -102.55], BR: [-14.24, -51.93], AR: [-38.42, -63.62],
  GB: [55.37, -3.44], DE: [51.17, 10.45], FR: [46.23, 2.21], IT: [41.87, 12.57], ES: [40.46, -3.75],
  RU: [61.52, 105.32], UA: [48.38, 31.17], PL: [51.92, 19.15], TR: [38.96, 35.24], NG: [9.08, 8.67],
  ZA: [-30.56, 22.94], EG: [26.82, 30.80], KE: [-0.02, 37.91], GH: [7.95, -1.03], ET: [9.15, 40.49],
  IN: [20.59, 78.96], CN: [35.86, 104.20], JP: [36.20, 138.25], KR: [35.91, 127.77], ID: [-0.79, 113.92],
  PH: [12.88, 121.77], TH: [15.87, 100.99], VN: [14.06, 108.28], MY: [4.21, 101.98], PK: [30.38, 69.35],
  BD: [23.68, 90.36], AU: [-25.27, 133.78], NZ: [-40.90, 174.89], SG: [1.35, 103.82], HK: [22.32, 114.17],
  AE: [23.42, 53.85], SA: [23.89, 45.08], IL: [31.05, 34.85], IR: [32.43, 53.69], IQ: [33.22, 43.68],
  CO: [4.57, -74.30], PE: [-9.19, -75.02], CL: [-35.68, -71.54], VE: [6.42, -66.59], EC: [-1.83, -78.18],
  MM: [21.92, 95.96], KH: [12.57, 104.99], NP: [28.39, 84.12], LK: [7.87, 80.77], AF: [33.94, 67.71],
};

type MapNode = {
  id: string;
  country: string;
  lat?: number;
  lng?: number;
  status: string;
  device_type?: string;
  bandwidth_mbps?: number;
  price_per_gb?: number;
};

type Hub = {
  key: string;
  country: string;
  lat: number;
  lng: number;
  count: number;
  online: number;
  bandwidthAvg: number;
  priceAvg: number;
  representativeNodeId: string;
};

type LiveMapProps = {
  nodes?: MapNode[];
  height?: number;
};

type MetricMode = 'bandwidth' | 'nodes' | 'price';
type MapMode = 'explore' | 'connect';

function project(lat: number, lng: number, rot: number, cx: number, cy: number, radius: number) {
  const la = (lat * Math.PI) / 180;
  const lo = (lng * Math.PI) / 180 + rot;
  const x = Math.cos(la) * Math.sin(lo);
  const y = -Math.sin(la);
  const z = Math.cos(la) * Math.cos(lo);
  return { x: cx + x * radius, y: cy + y * radius, z };
}

function hubRadius(h: Hub, metric: MetricMode) {
  if (metric === 'nodes') return Math.min(3 + Math.sqrt(h.count) * 1.2, 12);
  if (metric === 'price') return Math.min(3 + h.priceAvg * 30, 12);
  return Math.min(3 + h.bandwidthAvg / 250, 12);
}

function aggregateNodes(nodes: MapNode[]): Hub[] {
  const groups = new Map<string, Hub>();

  for (const node of nodes) {
    if (!node.country) continue;
    const fallback = COUNTRY_LATLNG[node.country] || [0, 0];
    const lat = Number.isFinite(node.lat as number) ? (node.lat as number) : fallback[0];
    const lng = Number.isFinite(node.lng as number) ? (node.lng as number) : fallback[1];
    const latKey = Math.round(lat * 10) / 10;
    const lngKey = Math.round(lng * 10) / 10;
    const key = `${node.country}:${latKey}:${lngKey}`;

    const cur = groups.get(key);
    const bandwidth = Number.isFinite(node.bandwidth_mbps as number) ? (node.bandwidth_mbps as number) : 0;
    const price = Number.isFinite(node.price_per_gb as number) ? (node.price_per_gb as number) : 0;
    const isOnline = node.status === 'online' ? 1 : 0;

    if (!cur) {
      groups.set(key, {
        key,
        country: node.country,
        lat,
        lng,
        count: 1,
        online: isOnline,
        bandwidthAvg: bandwidth,
        priceAvg: price,
        representativeNodeId: node.id,
      });
      continue;
    }

    const nextCount = cur.count + 1;
    cur.online += isOnline;
    cur.bandwidthAvg = (cur.bandwidthAvg * cur.count + bandwidth) / nextCount;
    cur.priceAvg = (cur.priceAvg * cur.count + price) / nextCount;
    cur.count = nextCount;
    if (bandwidth > cur.bandwidthAvg) cur.representativeNodeId = node.id;
  }

  return Array.from(groups.values()).sort((a, b) => b.count - a.count);
}

export default function LiveMap({ nodes = [], height = 420 }: LiveMapProps) {
  const [metric, setMetric] = useState<MetricMode>('bandwidth');
  const [mapMode, setMapMode] = useState<MapMode>('explore');
  const [rotating, setRotating] = useState(true);
  const [hoveredHubKey, setHoveredHubKey] = useState<string | null>(null);
  const [selectedHubKey, setSelectedHubKey] = useState<string | null>(null);

  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const stageRef = useRef<HTMLDivElement | null>(null);
  const dprRef = useRef(1);
  const rotRef = useRef(0);
  const rafRef = useRef<number | null>(null);
  const hoveredRef = useRef<string | null>(null);
  const selectedRef = useRef<string | null>(null);
  const rotatingRef = useRef(true);

  const hubs = useMemo(() => aggregateNodes(nodes), [nodes]);
  const totalOnline = useMemo(() => nodes.filter((n) => n.status === 'online').length, [nodes]);
  const countriesCount = useMemo(() => new Set(nodes.map((n) => n.country).filter(Boolean)).size, [nodes]);
  const selectedHub = useMemo(() => hubs.find((h) => h.key === selectedHubKey) || null, [hubs, selectedHubKey]);
  const hoveredHub = useMemo(() => hubs.find((h) => h.key === hoveredHubKey) || null, [hubs, hoveredHubKey]);

  useEffect(() => {
    hoveredRef.current = hoveredHubKey;
  }, [hoveredHubKey]);

  useEffect(() => {
    selectedRef.current = selectedHubKey;
  }, [selectedHubKey]);

  useEffect(() => {
    rotatingRef.current = rotating;
  }, [rotating]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const stage = stageRef.current;
    if (!canvas || !stage) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    let width = 0;
    let heightPx = 0;
    let cx = 0;
    let cy = 0;
    let radius = 0;

    const resize = () => {
      const rect = stage.getBoundingClientRect();
      width = rect.width;
      heightPx = rect.height;
      dprRef.current = Math.min(window.devicePixelRatio || 1, 2);
      canvas.width = Math.floor(width * dprRef.current);
      canvas.height = Math.floor(heightPx * dprRef.current);
      canvas.style.width = `${width}px`;
      canvas.style.height = `${heightPx}px`;
      ctx.setTransform(dprRef.current, 0, 0, dprRef.current, 0, 0);
      cx = width / 2;
      cy = heightPx / 2;
      radius = Math.min(width * 0.32, heightPx * 0.44);
    };

    const drawFrame = (t: number) => {
      ctx.clearRect(0, 0, width, heightPx);

      const glow = ctx.createRadialGradient(cx, cy, radius * 0.2, cx, cy, radius * 1.4);
      glow.addColorStop(0, 'rgba(34,211,238,0.10)');
      glow.addColorStop(0.55, 'rgba(34,211,238,0.03)');
      glow.addColorStop(1, 'rgba(0,0,0,0)');
      ctx.fillStyle = glow;
      ctx.beginPath();
      ctx.arc(cx, cy, radius * 1.4, 0, Math.PI * 2);
      ctx.fill();

      const disc = ctx.createRadialGradient(cx - radius * 0.25, cy - radius * 0.35, radius * 0.12, cx, cy, radius);
      disc.addColorStop(0, '#101922');
      disc.addColorStop(0.72, '#0a1016');
      disc.addColorStop(1, '#07090b');
      ctx.fillStyle = disc;
      ctx.beginPath();
      ctx.arc(cx, cy, radius, 0, Math.PI * 2);
      ctx.fill();

      ctx.save();
      ctx.beginPath();
      ctx.arc(cx, cy, radius, 0, Math.PI * 2);
      ctx.clip();
      ctx.strokeStyle = 'rgba(95,227,255,0.10)';
      ctx.lineWidth = 0.7;

      for (let la = -60; la <= 60; la += 15) {
        ctx.beginPath();
        let started = false;
        for (let lo = -180; lo <= 180; lo += 3) {
          const p = project(la, lo, rotRef.current, cx, cy, radius);
          if (p.z < -0.02) {
            started = false;
            continue;
          }
          if (!started) {
            ctx.moveTo(p.x, p.y);
            started = true;
          } else {
            ctx.lineTo(p.x, p.y);
          }
        }
        ctx.stroke();
      }
      ctx.restore();

      const projectedHubs = hubs
        .map((hub) => ({ hub, p: project(hub.lat, hub.lng, rotRef.current, cx, cy, radius) }))
        .filter((entry) => entry.p.z > -0.04);

      if (mapMode === 'connect' && selectedRef.current) {
        const selected = projectedHubs.find((entry) => entry.hub.key === selectedRef.current);
        if (selected) {
          const user = project(52.52, 13.4, rotRef.current, cx, cy, radius);
          if (user.z > -0.02) {
            const mx = (user.x + selected.p.x) / 2;
            const my = (user.y + selected.p.y) / 2;
            const cpx = mx + (mx - cx) * 0.26;
            const cpy = my + (my - cy) * 0.26;

            ctx.beginPath();
            ctx.moveTo(user.x, user.y);
            ctx.quadraticCurveTo(cpx, cpy, selected.p.x, selected.p.y);
            const route = ctx.createLinearGradient(user.x, user.y, selected.p.x, selected.p.y);
            route.addColorStop(0, 'rgba(167,139,250,0.95)');
            route.addColorStop(1, 'rgba(95,227,255,0.95)');
            ctx.strokeStyle = route;
            ctx.lineWidth = 1.8;
            ctx.stroke();

            const phase = ((t % 2200) / 2200);
            const u = phase;
            const px = (1 - u) * (1 - u) * user.x + 2 * (1 - u) * u * cpx + u * u * selected.p.x;
            const py = (1 - u) * (1 - u) * user.y + 2 * (1 - u) * u * cpy + u * u * selected.p.y;
            const rg = ctx.createRadialGradient(px, py, 0, px, py, 10);
            rg.addColorStop(0, 'rgba(255,255,255,1)');
            rg.addColorStop(0.4, 'rgba(138,234,255,0.8)');
            rg.addColorStop(1, 'rgba(95,227,255,0)');
            ctx.fillStyle = rg;
            ctx.beginPath();
            ctx.arc(px, py, 10, 0, Math.PI * 2);
            ctx.fill();
          }
        }
      }

      for (const entry of projectedHubs) {
        const isHovered = hoveredRef.current === entry.hub.key;
        const isSelected = selectedRef.current === entry.hub.key;
        const depth = Math.max(0.3, Math.min(1, entry.p.z + 0.14));
        const size = hubRadius(entry.hub, metric) * depth;

        if (isHovered || isSelected || entry.hub.bandwidthAvg > 1200) {
          ctx.strokeStyle = `rgba(95,227,255,${(isSelected ? 0.42 : 0.26) * depth})`;
          ctx.lineWidth = 1;
          ctx.beginPath();
          ctx.arc(entry.p.x, entry.p.y, size + 5, 0, Math.PI * 2);
          ctx.stroke();
        }

        const nodeGlow = ctx.createRadialGradient(entry.p.x, entry.p.y, 0, entry.p.x, entry.p.y, size * 3);
        nodeGlow.addColorStop(0, `rgba(138,234,255,${0.62 * depth})`);
        nodeGlow.addColorStop(1, 'rgba(95,227,255,0)');
        ctx.fillStyle = nodeGlow;
        ctx.beginPath();
        ctx.arc(entry.p.x, entry.p.y, size * 3, 0, Math.PI * 2);
        ctx.fill();

        ctx.fillStyle = `rgba(95,227,255,${0.95 * depth})`;
        ctx.beginPath();
        ctx.arc(entry.p.x, entry.p.y, size, 0, Math.PI * 2);
        ctx.fill();

        ctx.fillStyle = `rgba(255,255,255,${0.9 * depth})`;
        ctx.beginPath();
        ctx.arc(entry.p.x, entry.p.y, size * 0.34, 0, Math.PI * 2);
        ctx.fill();
      }

      if (rotatingRef.current) {
        rotRef.current += mapMode === 'connect' ? 0.0019 : 0.0025;
      }
      rafRef.current = requestAnimationFrame(drawFrame);
    };

    resize();
    rafRef.current = requestAnimationFrame(drawFrame);
    window.addEventListener('resize', resize);
    return () => {
      window.removeEventListener('resize', resize);
      if (rafRef.current) cancelAnimationFrame(rafRef.current);
    };
  }, [hubs, metric, mapMode]);

  const pickHubByPointer = (evt: React.MouseEvent<HTMLDivElement>) => {
    const stage = stageRef.current;
    if (!stage) return null;
    const rect = stage.getBoundingClientRect();
    const mx = evt.clientX - rect.left;
    const my = evt.clientY - rect.top;
    const cx = rect.width / 2;
    const cy = rect.height / 2;
    const radius = Math.min(rect.width * 0.32, rect.height * 0.44);
    let best: { key: string; dist: number } | null = null;

    for (const hub of hubs) {
      const p = project(hub.lat, hub.lng, rotRef.current, cx, cy, radius);
      if (p.z < -0.04) continue;
      const d = Math.hypot(p.x - mx, p.y - my);
      const threshold = hubRadius(hub, metric) + 10;
      if (d > threshold) continue;
      if (!best || d < best.dist) best = { key: hub.key, dist: d };
    }
    return best?.key || null;
  };

  const onMove = (evt: React.MouseEvent<HTMLDivElement>) => {
    const key = pickHubByPointer(evt);
    setHoveredHubKey(key);
  };

  const onLeave = () => {
    setHoveredHubKey(null);
  };

  const onClick = (evt: React.MouseEvent<HTMLDivElement>) => {
    if (mapMode !== 'connect') return;
    const key = pickHubByPointer(evt);
    setSelectedHubKey(key);
  };

  return (
    <div className="exra-map-wrap">
      <div className="exra-map-strip">
        <div className="exra-strip-cell">
          <div className="exra-strip-label"><span className="pulse-dot" /> active nodes</div>
          <div className="exra-strip-value">{nodes.length.toLocaleString()}</div>
        </div>
        <div className="exra-strip-cell">
          <div className="exra-strip-label">online</div>
          <div className="exra-strip-value">{totalOnline.toLocaleString()}</div>
        </div>
        <div className="exra-strip-cell">
          <div className="exra-strip-label">countries</div>
          <div className="exra-strip-value">{countriesCount}</div>
        </div>
        <div className="exra-strip-cell">
          <div className="exra-strip-label">visible hubs</div>
          <div className="exra-strip-value">{hubs.length}</div>
        </div>
      </div>

      <div className="exra-map-card">
        <div className="exra-map-chrome">
          <div className="exra-map-title">
            Global node map
            <span className="exra-live-pill"><span className="pulse-dot" /> Live</span>
          </div>
          <div className="exra-map-controls">
            <div className="seg">
              <button className={mapMode === 'explore' ? 'on' : ''} onClick={() => setMapMode('explore')}>Explore</button>
              <button className={mapMode === 'connect' ? 'on' : ''} onClick={() => setMapMode('connect')}>Connect</button>
            </div>
            <div className="seg">
              <button className={metric === 'bandwidth' ? 'on' : ''} onClick={() => setMetric('bandwidth')}>Bandwidth</button>
              <button className={metric === 'nodes' ? 'on' : ''} onClick={() => setMetric('nodes')}>Nodes</button>
              <button className={metric === 'price' ? 'on' : ''} onClick={() => setMetric('price')}>$/GB</button>
            </div>
            <div className="seg">
              <button className={!rotating ? 'on' : ''} onClick={() => setRotating(false)}>Pause</button>
              <button className={rotating ? 'on' : ''} onClick={() => setRotating(true)}>Play</button>
            </div>
          </div>
        </div>

        <div
          ref={stageRef}
          className={`exra-map-stage ${mapMode === 'connect' ? 'connect' : ''}`}
          style={{ height }}
          onMouseMove={onMove}
          onMouseLeave={onLeave}
          onClick={onClick}
        >
          <canvas ref={canvasRef} />
          {mapMode === 'connect' && (
            <div className="connect-banner">Connect mode: click any hub to build route from Berlin</div>
          )}
          {hoveredHub && (
            <div className="map-tooltip">
              <div className="tip-country">{hoveredHub.country}</div>
              <div className="tip-row"><span>Nodes</span><b>{hoveredHub.count}</b></div>
              <div className="tip-row"><span>Online</span><b>{hoveredHub.online}</b></div>
              <div className="tip-row"><span>Avg BW</span><b>{Math.round(hoveredHub.bandwidthAvg)} Mbps</b></div>
              <div className="tip-row"><span>Avg Price</span><b>${hoveredHub.priceAvg.toFixed(3)}</b></div>
            </div>
          )}
          {mapMode === 'connect' && selectedHub && (
            <div className="route-readout">
              <div className="route-title">Route established</div>
              <div className="route-path">Berlin {"->"} {selectedHub.country}</div>
              <div className="route-meta">
                <span>Hub nodes: {selectedHub.count}</span>
                <span>Price avg: ${selectedHub.priceAvg.toFixed(3)}</span>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
