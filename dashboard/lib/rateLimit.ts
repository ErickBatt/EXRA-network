/**
 * In-memory rate limiting for development/testing.
 * For production, replace with Redis-backed implementation using Upstash.
 */

interface RateLimitEntry {
  count: number;
  resetAt: number;
}

const store = new Map<string, RateLimitEntry>();

/**
 * Check if a request should be rate limited.
 * Returns true if the request is allowed, false if it exceeds the limit.
 */
export async function checkRateLimit(
  key: string,
  limit: number = 1,
  windowSeconds: number = 300
): Promise<boolean> {
  const now = Date.now();
  const entry = store.get(key);

  // If no entry or window expired, create new entry
  if (!entry || now >= entry.resetAt) {
    store.set(key, {
      count: 1,
      resetAt: now + windowSeconds * 1000,
    });
    return true;
  }

  // Increment count
  entry.count++;

  // Check if limit exceeded
  if (entry.count > limit) {
    return false;
  }

  return true;
}

/**
 * Get remaining requests for a key.
 * Returns -1 if key doesn't exist or window expired.
 */
export function getRemainingRequests(
  key: string,
  limit: number = 1
): number {
  const entry = store.get(key);
  if (!entry || Date.now() >= entry.resetAt) {
    return limit;
  }
  return Math.max(0, limit - entry.count);
}

/**
 * Clear all rate limit entries (for testing).
 */
export function clearRateLimitStore(): void {
  store.clear();
}

/**
 * Cleanup expired entries periodically (optional).
 */
export function cleanupExpiredEntries(): void {
  const now = Date.now();
  for (const [key, entry] of store.entries()) {
    if (now >= entry.resetAt) {
      store.delete(key);
    }
  }
}

// Run cleanup every 5 minutes
if (typeof setInterval !== 'undefined') {
  setInterval(cleanupExpiredEntries, 5 * 60 * 1000);
}
