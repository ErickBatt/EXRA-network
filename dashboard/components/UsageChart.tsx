'use client';

import { useEffect, useState } from 'react';

export default function UsageChart({ isActive, dataPoints = [] }: { isActive: boolean, dataPoints?: number[] }) {
  const [points, setPoints] = useState<number[]>(dataPoints);

  useEffect(() => {
    if (!isActive) return;
    const interval = setInterval(() => {
      setPoints(prev => {
        const next = [...prev, Math.random() * 50 + 10].slice(-20);
        return next;
      });
    }, 2000);
    return () => clearInterval(interval);
  }, [isActive]);

  const max = Math.max(...points, 100);
  const height = 60;
  const width = 200;
  const step = width / (points.length - 1 || 1);

  const pathData = points.map((p, i) => {
    const x = i * step;
    const y = height - (p / max) * height;
    return `${i === 0 ? 'M' : 'L'} ${x} ${y}`;
  }).join(' ');

  return (
    <div className="usage-chart-container">
      <svg width={width} height={height} className="usage-chart-svg">
        <path 
          d={pathData} 
          fill="none" 
          stroke="var(--accent)" 
          strokeWidth="2" 
          strokeLinecap="round" 
          strokeLinejoin="round" 
        />
        <path 
          d={`${pathData} L ${width} ${height} L 0 ${height} Z`} 
          fill="url(#usage-grad)" 
          opacity="0.1" 
        />
        <defs>
          <linearGradient id="usage-grad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--accent)" />
            <stop offset="100%" stopColor="transparent" />
          </linearGradient>
        </defs>
      </svg>
    </div>
  );
}
