/**
 * Telegram Mini App (TMA) Authentication
 * 
 * Validates Telegram initData signature and TTL (Time To Live).
 * This prevents spoofing and replay attacks.
 */

import crypto from 'crypto';

export interface TelegramInitData {
  user?: {
    id: number;
    first_name: string;
    username?: string;
    is_bot?: boolean;
    is_premium?: boolean;
    language_code?: string;
  };
  auth_date: number;
  hash: string;
  chat_instance?: string;
  chat_type?: string;
}

/**
 * Validate Telegram initData signature and TTL.
 * 
 * Returns { valid: true, data: TelegramInitData } if valid,
 * or { valid: false } if invalid.
 */
export function validateTelegramInitData(
  initData: string,
  botToken: string,
  maxAgeSec: number = 300 // 5 minutes
): { valid: boolean; data?: TelegramInitData } {
  try {
    if (!initData || !botToken) {
      console.warn('[TMA] Missing initData or botToken');
      return { valid: false };
    }

    // Parse initData as URLSearchParams
    const params = new URLSearchParams(initData);
    const hash = params.get('hash');

    if (!hash) {
      console.warn('[TMA] Missing hash in initData');
      return { valid: false };
    }

    // Create data check string (sorted by key)
    const dataCheckString = Array.from(params.entries())
      .filter(([key]) => key !== 'hash')
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([key, value]) => `${key}=${value}`)
      .join('\n');

    // Compute HMAC-SHA256
    // Step 1: Create secret key from bot token
    const secretKey = crypto
      .createHmac('sha256', 'WebAppData')
      .update(botToken)
      .digest('hex');

    // Step 2: Compute hash of data check string
    const computedHash = crypto
      .createHmac('sha256', secretKey)
      .update(dataCheckString)
      .digest('hex');

    // Step 3: Compare hashes (timing-safe comparison)
    if (!timingSafeEqual(computedHash, hash)) {
      console.warn('[TMA] Hash mismatch - possible tampering');
      return { valid: false };
    }

    // Check TTL (Time To Live)
    const authDate = parseInt(params.get('auth_date') || '0', 10);
    const now = Math.floor(Date.now() / 1000);
    const age = now - authDate;

    if (age < 0) {
      console.warn('[TMA] initData has future timestamp');
      return { valid: false };
    }

    if (age > maxAgeSec) {
      console.warn(`[TMA] initData expired: ${age}s old (max ${maxAgeSec}s)`);
      return { valid: false };
    }

    // Parse user data
    const userJson = params.get('user');
    const user = userJson ? JSON.parse(userJson) : undefined;

    return {
      valid: true,
      data: {
        user,
        auth_date: authDate,
        hash,
      },
    };
  } catch (error) {
    console.error('[TMA] Validation error:', error);
    return { valid: false };
  }
}

/**
 * Timing-safe string comparison to prevent timing attacks.
 */
function timingSafeEqual(a: string, b: string): boolean {
  if (a.length !== b.length) {
    return false;
  }

  let result = 0;
  for (let i = 0; i < a.length; i++) {
    result |= a.charCodeAt(i) ^ b.charCodeAt(i);
  }

  return result === 0;
}
