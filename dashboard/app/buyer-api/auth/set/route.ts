/**
 * POST /buyer-api/auth/set
 * Body: { "api_key": "<token>" }
 *
 * Stores the buyer's API key as an httpOnly cookie so subsequent calls
 * via /buyer-api/* can authenticate without exposing the token to JS.
 *
 * The handler does NOT validate the key against the backend on its own
 * (that would couple every cookie set to a Go round-trip). Callers should
 * verify by following up with GET /buyer-api/api/buyer/me — a 401 means
 * the key was wrong and the cookie should be cleared.
 */
import { NextRequest, NextResponse } from 'next/server';

const COOKIE_NAME = 'exra_buyer_api_key';
const MAX_AGE_SECONDS = 60 * 60 * 24 * 30; // 30 days

function originOk(req: NextRequest): boolean {
  const host = req.headers.get('host') || '';
  const origin = req.headers.get('origin') || req.headers.get('referer') || '';
  if (!origin) return false;
  try {
    return new URL(origin).host === host;
  } catch {
    return false;
  }
}

export async function POST(req: NextRequest) {
  if (!originOk(req)) {
    return NextResponse.json({ error: 'cross-origin request denied' }, { status: 403 });
  }

  let body: { api_key?: unknown };
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: 'invalid json' }, { status: 400 });
  }

  const apiKey = typeof body.api_key === 'string' ? body.api_key.trim() : '';
  if (!apiKey || apiKey.length > 512) {
    return NextResponse.json({ error: 'api_key required' }, { status: 400 });
  }

  const res = NextResponse.json({ ok: true });
  res.cookies.set({
    name: COOKIE_NAME,
    value: apiKey,
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    path: '/',
    maxAge: MAX_AGE_SECONDS,
  });
  return res;
}
