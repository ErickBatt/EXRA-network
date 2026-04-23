/**
 * CSRF Token Management
 * 
 * Generates and validates CSRF tokens for protecting against Cross-Site Request Forgery attacks.
 * Tokens are stored in-memory with TTL (Time To Live) and are one-time use.
 */

import crypto from 'crypto';

interface CsrfToken {
  token: string;
  createdAt: number;
  used: boolean;
}

const tokenStore = new Map<string, CsrfToken>();

/**
 * Generate a new CSRF token for a user.
 * Each token is valid for 1 hour and can only be used once.
 */
export function generateCsrfToken(): string {
  const token = crypto.randomBytes(32).toString('hex');
  const now = Date.now();

  tokenStore.set(token, {
    token,
    createdAt: now,
    used: false,
  });

  return token;
}

/**
 * Validate a CSRF token.
 * Returns true if token is valid and hasn't been used.
 * Marks token as used after validation (one-time use).
 */
export function validateCsrfToken(token: string): boolean {
  if (!token) {
    return false;
  }

  const entry = tokenStore.get(token);
  if (!entry) {
    console.warn('[SECURITY] CSRF token not found');
    return false;
  }

  // Check if token has expired (1 hour)
  const age = Date.now() - entry.createdAt;
  if (age > 3600000) {
    console.warn('[SECURITY] CSRF token expired');
    tokenStore.delete(token);
    return false;
  }

  // Check if token has already been used
  if (entry.used) {
    console.warn('[SECURITY] CSRF token already used (replay attack?)');
    return false;
  }

  // Mark as used
  entry.used = true;

  // Delete after a short delay to prevent immediate reuse
  setTimeout(() => {
    tokenStore.delete(token);
  }, 1000);

  return true;
}

/**
 * Clear all CSRF tokens (for testing).
 */
export function clearCsrfTokenStore(): void {
  tokenStore.clear();
}

/**
 * Cleanup expired tokens periodically.
 */
export function cleanupExpiredCsrfTokens(): void {
  const now = Date.now();
  const oneHour = 3600000;

  for (const [token, entry] of tokenStore.entries()) {
    if (now - entry.createdAt > oneHour) {
      tokenStore.delete(token);
    }
  }
}

// Run cleanup every 30 minutes
if (typeof setInterval !== 'undefined') {
  setInterval(cleanupExpiredCsrfTokens, 30 * 60 * 1000);
}
