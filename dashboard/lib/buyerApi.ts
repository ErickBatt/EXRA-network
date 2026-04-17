/**
 * Browser-side helper for talking to the buyer-scoped Go endpoints
 * through the Next.js cookie-auth proxy.
 *
 * Use this instead of `fetchJson(path, apiKey)` for any buyer call. The
 * cookie is attached automatically (`credentials: 'include'`), so callers
 * never see the API key.
 */

const PROXY_PREFIX = '/buyer-api';

export class BuyerApiUnauthorized extends Error {
  constructor(message = 'unauthorized') {
    super(message);
    this.name = 'BuyerApiUnauthorized';
  }
}

export async function buyerFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  // Backend lives under /api/*; the proxy expects /buyer-api/<rest>
  // where <rest> is the part after /api/. Normalise both shapes.
  const trimmed = path.startsWith('/api/')
    ? path.slice('/api/'.length)
    : path.replace(/^\/+/, '');
  const url = `${PROXY_PREFIX}/${trimmed}`;

  const headers = new Headers(init?.headers || {});
  if (!headers.has('Content-Type') && init?.body) {
    headers.set('Content-Type', 'application/json');
  }

  const res = await fetch(url, {
    ...init,
    headers,
    credentials: 'include',
    cache: 'no-store',
  });

  if (res.status === 401) throw new BuyerApiUnauthorized();
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`API ${res.status}: ${body}`);
  }
  // Empty body tolerated.
  const text = await res.text();
  if (!text) return undefined as unknown as T;
  return JSON.parse(text) as T;
}

export async function setBuyerApiKey(apiKey: string): Promise<void> {
  const res = await fetch(`${PROXY_PREFIX}/auth/set`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ api_key: apiKey }),
    credentials: 'include',
  });
  if (!res.ok) throw new Error(`failed to set buyer cookie (${res.status})`);
}

export async function clearBuyerApiKey(): Promise<void> {
  await fetch(`${PROXY_PREFIX}/auth/clear`, {
    method: 'POST',
    credentials: 'include',
  }).catch(() => {});
}

export async function revealBuyerApiKey(): Promise<string> {
  const res = await fetch(`${PROXY_PREFIX}/auth/reveal`, {
    credentials: 'include',
    cache: 'no-store',
  });
  if (!res.ok) throw new Error(`reveal failed (${res.status})`);
  const data = (await res.json()) as { api_key?: string };
  if (!data.api_key) throw new Error('reveal: empty');
  return data.api_key;
}
