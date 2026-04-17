/**
 * POST /buyer-api/auth/clear
 * Removes the buyer API key cookie. Idempotent.
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

export async function POST(req: NextRequest) {
  if (!originOk(req)) {
    return NextResponse.json({ error: 'cross-origin request denied' }, { status: 403 });
  }
  const res = NextResponse.json({ ok: true });
  res.cookies.set({
    name: COOKIE_NAME,
    value: '',
    httpOnly: true,
    secure: process.env.NODE_ENV === 'production',
    sameSite: 'lax',
    path: '/',
    maxAge: 0,
  });
  return res;
}
