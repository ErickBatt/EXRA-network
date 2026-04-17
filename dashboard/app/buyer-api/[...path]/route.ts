/**
 * Server-side proxy for buyer-authenticated calls to the Go backend.
 *
 * Why this exists
 * ----------------
 * The buyer API key (`X-Exra-Token`) used to live in `localStorage`, which
 * means any XSS on the marketplace UI exfiltrates the key permanently.
 * Moving the key into an httpOnly cookie that's only readable by this
 * Next.js proxy reduces the blast radius: an attacker who lands an XSS
 * payload can drive the proxy during their window, but cannot exfiltrate
 * the raw key.
 *
 * nginx routing (mirrors /next-tma/*):
 *   /api/*         -> Go backend (port 8080)
 *   /buyer-api/*   -> Next.js (port 3000) -- THIS proxy
 *
 * CSRF
 * ----
 * Because the cookie auto-rides on cross-site requests, we must verify
 * Origin (or fall back to Referer) against the request host for any
 * state-changing method. Same-site cookie attribute is a defence in depth,
 * not the only check (older browsers, opt-out scenarios).
 */
import { NextRequest, NextResponse } from 'next/server';

const GO_BACKEND =
  process.env.BUYER_API_BASE ||
  process.env.TMA_API_BASE ||
  'https://api.exra.space';

const COOKIE_NAME = 'exra_buyer_api_key';

const STATE_CHANGING = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

function originOk(req: NextRequest): boolean {
  if (!STATE_CHANGING.has(req.method)) return true;
  const host = req.headers.get('host') || '';
  const origin = req.headers.get('origin') || req.headers.get('referer') || '';
  if (!origin) return false;
  try {
    const u = new URL(origin);
    return u.host === host;
  } catch {
    return false;
  }
}

async function handler(
  req: NextRequest,
  { params }: { params: { path: string[] } },
) {
  const segments = params.path || [];
  // Defence in depth: never forward auth-handler paths to the backend; they
  // are owned by the local /buyer-api/auth/* route handlers.
  if (segments[0] === 'auth') {
    return NextResponse.json({ error: 'not found' }, { status: 404 });
  }

  if (!originOk(req)) {
    return NextResponse.json({ error: 'cross-origin request denied' }, { status: 403 });
  }

  const apiKey = req.cookies.get(COOKIE_NAME)?.value;
  if (!apiKey) {
    return NextResponse.json({ error: 'not authenticated' }, { status: 401 });
  }

  const url = new URL(req.url);
  const path = segments.join('/');
  const backendUrl = `${GO_BACKEND}/api/${path}${url.search}`;

  const headers = new Headers();
  headers.set('X-Exra-Token', apiKey);
  const ct = req.headers.get('content-type');
  if (ct) headers.set('content-type', ct);

  let body: BodyInit | undefined;
  if (req.method !== 'GET' && req.method !== 'HEAD') {
    body = await req.text();
  }

  const res = await fetch(backendUrl, {
    method: req.method,
    headers,
    body,
    cache: 'no-store',
  });

  const text = await res.text();
  const respHeaders: Record<string, string> = {};
  const upstreamCt = res.headers.get('content-type');
  if (upstreamCt) respHeaders['content-type'] = upstreamCt;
  return new NextResponse(text, { status: res.status, headers: respHeaders });
}

export {
  handler as GET,
  handler as POST,
  handler as PUT,
  handler as PATCH,
  handler as DELETE,
};
