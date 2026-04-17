/**
 * GET /buyer-api/auth/reveal
 *
 * Returns the buyer API key in plaintext so the user can copy it into a
 * proxy client (squid, curl, etc.). This deliberately bypasses the
 * httpOnly protection — by definition, revealing the key to JS hands it
 * to any concurrent XSS too. We therefore:
 *   1. Require same-origin (Origin/Referer) so a CSRF-style fetch from a
 *      malicious page cannot pull the key.
 *   2. Still keep the everyday API surface (/buyer-api/*) cookie-only;
 *      reveal is an explicit user-driven action, not a constant leak.
 *
 * A future hardening pass should gate this behind a fresh Supabase
 * re-auth challenge; tracked in TMA_MARKETPLACE_HARDENING_PLAN.md.
 */
import { NextRequest, NextResponse } from 'next/server';

const COOKIE_NAME = 'exra_buyer_api_key';

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

export async function GET(req: NextRequest) {
  if (!originOk(req)) {
    return NextResponse.json({ error: 'cross-origin request denied' }, { status: 403 });
  }
  const apiKey = req.cookies.get(COOKIE_NAME)?.value;
  if (!apiKey) {
    return NextResponse.json({ error: 'not authenticated' }, { status: 401 });
  }
  return NextResponse.json(
    { api_key: apiKey },
    { headers: { 'Cache-Control': 'no-store, max-age=0' } },
  );
}
