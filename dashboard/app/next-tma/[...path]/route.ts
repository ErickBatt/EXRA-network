/**
 * Server-side proxy для TMA API вызовов.
 * Путь /next-tma/* — без /api/ префикса, поэтому nginx роутит его к Next.js (location /),
 * а не к Go backend (location /api/). NODE_SECRET остаётся server-side.
 *
 * nginx routing:
 *   /api/*       → Go backend (port 8080)   ← перехватил бы /api/tma/*
 *   /next-tma/*  → Next.js (port 3000)      ← наш прокси, безопасно
 */
import { NextRequest, NextResponse } from 'next/server';

const GO_BACKEND = process.env.TMA_API_BASE || 'https://api.exra.space';
const NODE_SECRET = process.env.TMA_NODE_SECRET || process.env.NODE_SECRET || '';

async function handler(req: NextRequest, { params }: { params: { path: string[] } }) {
  const path = params.path.join('/');
  const url = new URL(req.url);
  const backendUrl = `${GO_BACKEND}/api/tma/${path}${url.search}`;

  const headers = new Headers();
  headers.set('Content-Type', 'application/json');
  headers.set('X-Node-Secret', NODE_SECRET);

  let body: string | undefined;
  if (req.method !== 'GET' && req.method !== 'HEAD') {
    body = await req.text();
  }

  const res = await fetch(backendUrl, {
    method: req.method,
    headers,
    body,
    cache: 'no-store',
  });

  const data = await res.text();
  return new NextResponse(data, {
    status: res.status,
    headers: { 'Content-Type': 'application/json' },
  });
}

export { handler as GET, handler as POST, handler as PUT, handler as DELETE };
