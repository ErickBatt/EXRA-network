/**
 * POST /next-tma/auth
 * 
 * Validates Telegram initData and returns user profile.
 * This endpoint is called by TMA on startup to authenticate the user.
 */

import { NextRequest, NextResponse } from 'next/server';
import { validateTelegramInitData } from '@/lib/tmaAuth';

export async function POST(request: NextRequest) {
  try {
    const { init_data } = await request.json();

    if (!init_data) {
      console.warn('[TMA] Auth request missing init_data');
      return NextResponse.json(
        { error: 'init_data required' },
        { status: 400 }
      );
    }

    // Get bot token from environment
    const botToken = process.env.TELEGRAM_BOT_TOKEN;
    if (!botToken) {
      console.error('[TMA] TELEGRAM_BOT_TOKEN not configured');
      return NextResponse.json(
        { error: 'server misconfigured' },
        { status: 500 }
      );
    }

    // Validate initData signature and TTL
    const { valid, data } = validateTelegramInitData(init_data, botToken, 300); // 5 minutes

    if (!valid || !data?.user) {
      console.warn('[TMA] Invalid initData');
      return NextResponse.json(
        { error: 'invalid init_data' },
        { status: 401 }
      );
    }

    const telegramId = data.user.id;
    const firstName = data.user.first_name || 'User';
    const username = data.user.username || '';

    // Return user profile
    // In a real implementation, you would:
    // 1. Look up user in database by telegram_id
    // 2. Create user if doesn't exist
    // 3. Return user profile with balance, devices, etc.
    return NextResponse.json({
      telegram_id: telegramId,
      first_name: firstName,
      username: username,
      devices: [],
      total_usd: 0,
      total_exra: 0,
    });
  } catch (error) {
    console.error('[TMA] Auth error:', error);
    return NextResponse.json(
      { error: 'internal server error' },
      { status: 500 }
    );
  }
}
