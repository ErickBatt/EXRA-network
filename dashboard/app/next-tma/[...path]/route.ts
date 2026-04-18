/**
 * Server-side proxy for TMA API calls.
 *
 * Auth model (v2.4.1): cookie-based TMA session (exra_tma_session, HttpOnly).
 * The Next.js proxy just forwards the caller's Cookie header + body to the Go
 * backend — no server-side shared secret. Authority lives in the signed JWT
 * cookie that the browser already holds, so a random visitor without a valid
 * session cannot impersonate another user.
 *
 * nginx routing (unchanged):
 *   /api/*       → Go backend (port 8080)
 *   /next-tma/*  → Next.js (port 3000)  ← this proxy
 */
import { NextRequest, NextResponse } from 'next/server';

const GO_BACKEND = process.env.TMA_API_BASE || 'https://api.exra.space';

const FORWARD_REQ_HEADERS = ['content-type', 'cookie', 'x-real-ip', 'x-forwarded-for'];
const FORWARD_RES_HEADERS = ['content-type', 'set-cookie'];

async function handler(req: NextRequest, { params }: { params: { path: string[] } }) {
  const path = params.path.join('/');
  const url = new URL(req.url);
  const backendUrl = `${GO_BACKEND}/api/tma/${path}${url.search}`;

  const headers = new Headers();
  for (const name of FORWARD_REQ_HEADERS) {
    const v = req.headers.get(name);
    if (v) headers.set(name, v);
  }
  if (!headers.has('content-type')) headers.set('content-type', 'application/json');

  let body: string | undefined;
  if (req.method !== 'GET' && req.method !== 'HEAD') {
    body = await req.text();
  }

  const res = await fetch(backendUrl, {
    method: req.method,
    headers,
    body,
    cache: 'no-store',
    redirect: 'manual',
  });

  const data = await res.text();
  const out = new NextResponse(data, { status: res.status });
  for (const name of FORWARD_RES_HEADERS) {
    const v = res.headers.get(name);
    if (v) out.headers.set(name, v);
  }
  return out;
}

export { handler as GET, handler as POST, handler as PUT, handler as DELETE };
