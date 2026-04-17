'use client';

import { useEffect, useRef, useState } from 'react';

// Approximate [x%, y%] positions on a flat world map for common countries
const COUNTRY_COORDS: Record<string, [number, number]> = {
  US: [18, 35], CA: [16, 25], MX: [17, 45], BR: [28, 65], AR: [26, 75],
  GB: [45, 28], DE: [48, 28], FR: [46, 32], IT: [49, 34], ES: [44, 35],
  RU: [62, 25], UA: [53, 30], PL: [50, 28], TR: [55, 36], NG: [47, 52],
  ZA: [50, 70], EG: [53, 42], KE: [54, 55], GH: [45, 52], ET: [55, 52],
  IN: [67, 43], CN: [73, 35], JP: [80, 33], KR: [79, 36], ID: [75, 57],
  PH: [78, 48], TH: [73, 48], VN: [75, 47], MY: [74, 53], PK: [65, 40],
  BD: [69, 43], AU: [78, 70], NZ: [84, 75], SG: [75, 54], HK: [77, 42],
  AE: [60, 43], SA: [58, 44], IL: [54, 38], IR: [61, 38], IQ: [57, 38],
  CO: [23, 55], PE: [23, 63], CL: [24, 70], VE: [25, 53], EC: [22, 58],
  MM: [72, 45], KH: [74, 49], NP: [68, 41], LK: [68, 51], AF: [63, 38],
};

interface MapNode {
  id: string;
  country: string;
  lat?: number;
  lng?: number;
  status: string;
  device_type?: string;
}

interface LiveMapProps {
  nodes?: MapNode[];
  height?: number;
}

export default function LiveMap({ nodes = [], height = 300 }: LiveMapProps) {
  const [liveNodes, setLiveNodes] = useState<MapNode[]>(nodes);
  const [pulseMap, setPulseMap] = useState<Record<string, number>>({});
  const wsRef = useRef<WebSocket | null>(null);
  const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL || 'https://api.exra.space';

  useEffect(() => {
    setLiveNodes(nodes);
  }, [nodes]);

  useEffect(() => {
    // Connect to live map WebSocket
    const wsUrl = API_BASE.replace('https://', 'wss://').replace('http://', 'ws://') + '/ws/map';
    try {
      const ws = new WebSocket(wsUrl);
      wsRef.current = ws;

      ws.onmessage = (e) => {
        try {
          const event = JSON.parse(e.data);
          if (event.type === 'node_online' || event.type === 'heartbeat') {
            setLiveNodes(prev => {
              const exists = prev.find(n => n.id === event.node_id);
              if (!exists && event.country) {
                return [...prev, { id: event.node_id, country: event.country, status: 'online', device_type: event.device_type }];
              }
              return prev;
            });
            setPulseMap(prev => ({ ...prev, [event.node_id]: Date.now() }));
          }
        } catch {}
      };

      ws.onerror = () => {};
    } catch {}

    return () => {
      wsRef.current?.close();
    };
  }, []);

  // Group nodes by country for rendering
  const countryGroups: Record<string, number> = {};
  liveNodes.forEach(n => {
    if (n.country) countryGroups[n.country] = (countryGroups[n.country] || 0) + 1;
  });

  return (
    <div style={{ position: 'relative', width: '100%', height, background: '#080808', borderRadius: '16px', overflow: 'hidden', border: '1px solid rgba(255,255,255,0.06)' }}>
      {/* Grid lines */}
      <svg width="100%" height="100%" style={{ position: 'absolute', inset: 0, opacity: 0.15 }}>
        {[20, 40, 60, 80].map(x => (
          <line key={`v${x}`} x1={`${x}%`} y1="0" x2={`${x}%`} y2="100%" stroke="#c8f03c" strokeWidth="0.5" strokeDasharray="4,8" />
        ))}
        {[25, 50, 75].map(y => (
          <line key={`h${y}`} x1="0" y1={`${y}%`} x2="100%" y2={`${y}%`} stroke="#c8f03c" strokeWidth="0.5" strokeDasharray="4,8" />
        ))}
      </svg>

      {/* Node dots */}
      <svg width="100%" height="100%" style={{ position: 'absolute', inset: 0 }}>
        {Object.entries(countryGroups).map(([country, count]) => {
          const coords = COUNTRY_COORDS[country];
          if (!coords) return null;
          const [x, y] = coords;
          const isPulsing = liveNodes.some(n => n.country === country && pulseMap[n.id] && Date.now() - pulseMap[n.id] < 3000);
          const size = Math.min(4 + count * 1.5, 12);

          return (
            <g key={country}>
              {isPulsing && (
                <circle cx={`${x}%`} cy={`${y}%`} r={size + 8} fill="none" stroke="#c8f03c" strokeWidth="1" opacity="0.4">
                  <animate attributeName="r" values={`${size};${size + 14}`} dur="1.5s" repeatCount="indefinite" />
                  <animate attributeName="opacity" values="0.4;0" dur="1.5s" repeatCount="indefinite" />
                </circle>
              )}
              <circle cx={`${x}%`} cy={`${y}%`} r={size} fill="#c8f03c" opacity="0.85" />
              <circle cx={`${x}%`} cy={`${y}%`} r={size * 0.5} fill="#fff" opacity="0.5" />
              {count > 1 && (
                <text x={`${x}%`} y={`${y}%`} dy="1px" textAnchor="middle" dominantBaseline="middle" fontSize="7" fill="#000" fontWeight="bold">
                  {count > 99 ? '99+' : count}
                </text>
              )}
            </g>
          );
        })}
      </svg>

      {/* Stats overlay */}
      <div style={{ position: 'absolute', bottom: 12, left: 16, display: 'flex', gap: 16, alignItems: 'center' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
          <div style={{ width: 6, height: 6, borderRadius: '50%', background: '#c8f03c', boxShadow: '0 0 6px #c8f03c' }} />
          <span style={{ fontSize: 11, color: '#9e9b92', fontFamily: 'monospace' }}>{liveNodes.filter(n => n.status === 'online').length} online</span>
        </div>
        <span style={{ fontSize: 11, color: '#5a5850', fontFamily: 'monospace' }}>{Object.keys(countryGroups).length} countries</span>
      </div>

      <div style={{ position: 'absolute', top: 12, right: 16, display: 'flex', alignItems: 'center', gap: 6 }}>
        <div style={{ width: 5, height: 5, borderRadius: '50%', background: '#c8f03c', animation: 'pulse 2s infinite' }} />
        <span style={{ fontSize: 10, color: '#5a5850', fontFamily: 'monospace', textTransform: 'uppercase' }}>live</span>
      </div>

      <style>{`
        @keyframes pulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.3; }
        }
      `}</style>
    </div>
  );
}
