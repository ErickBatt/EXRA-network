/**
 * POST /next-tma/auth — forward to Go backend /api/tma/auth.
 *
 * The previous version validated initData locally and returned a stub empty profile,
 * bypassing the real Go backend entirely (no session cookie, no real data).
 *
 * Now: pure proxy. initData validation + session cookie issuance is exclusively
 * handled by Go (HMAC-SHA256, 1h TTL, HttpOnly SameSite=None JWT).
 * The catch-all [...path] handles all other /next-tma/* paths identically.
 */
import { NextRequest, NextResponse } from 'next/server';

const GO_BACKEND = process.env.TMA_API_BASE || 'https://api.exra.space';

const FORWARD_RES_HEADERS = ['content-type', 'set-cookie'];

export async function POST(req: NextRequest) {
  const body = await req.text();

  const res = await fetch(`${GO_BACKEND}/api/tma/auth`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body,
    cache: 'no-store',
  });

  const data = await res.text();
  const out = new NextResponse(data, { status: res.status });

  for (const name of FORWARD_RES_HEADERS) {
    const v = res.headers.get(name);
    if (v) out.headers.set(name, v);
  }

  return out;
}
