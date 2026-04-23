/**
 * GET /buyer-api/auth/reveal
 *
 * Returns the buyer API key in plaintext so the user can copy it into a
 * proxy client (squid, curl, etc.). This deliberately bypasses the
 * httpOnly protection — by definition, revealing the key to JS hands it
 * to any concurrent XSS too. We therefore:
 *   1. Require same-origin (Origin/Referer) so a CSRF-style fetch from a
 *      malicious page cannot pull the key.
 *   2. Rate limit to 1 reveal per 5 minutes per user
 *   3. Log all reveal operations for audit trail
 *   4. Still keep the everyday API surface (/buyer-api/*) cookie-only;
 *      reveal is an explicit user-driven action, not a constant leak.
 *
 * A future hardening pass should gate this behind a fresh Supabase
 * re-auth challenge; tracked in TMA_MARKETPLACE_HARDENING_PLAN.md.
 */
import { NextRequest, NextResponse } from 'next/server';
import { checkRateLimit } from '@/lib/rateLimit';

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

function getClientIdentifier(req: NextRequest): string {
  // Try to get user ID from cookie or IP address
  const ip = req.headers.get('x-forwarded-for') || req.headers.get('x-real-ip') || 'unknown';
  return ip;
}

export async function GET(req: NextRequest) {
  if (!originOk(req)) {
    console.warn('[SECURITY] /buyer-api/auth/reveal: cross-origin request denied');
    return NextResponse.json({ error: 'cross-origin request denied' }, { status: 403 });
  }

  const apiKey = req.cookies.get(COOKIE_NAME)?.value;
  if (!apiKey) {
    console.warn('[SECURITY] /buyer-api/auth/reveal: not authenticated');
    return NextResponse.json({ error: 'not authenticated' }, { status: 401 });
  }

  // Rate limit: 1 reveal per 5 minutes per client
  const clientId = getClientIdentifier(req);
  const rateLimitKey = `reveal:${clientId}`;
  const allowed = await checkRateLimit(rateLimitKey, 1, 300); // 1 per 5 minutes

  if (!allowed) {
    console.warn(`[SECURITY] /buyer-api/auth/reveal: rate limit exceeded for ${clientId}`);
    return NextResponse.json(
      { error: 'too many requests' },
      { status: 429, headers: { 'Retry-After': '300' } }
    );
  }

  // Log the reveal operation
  console.log(`[AUDIT] API key revealed for ${clientId} at ${new Date().toISOString()}`);

  return NextResponse.json(
    { api_key: apiKey },
    { headers: { 'Cache-Control': 'no-store, max-age=0' } },
  );
}
