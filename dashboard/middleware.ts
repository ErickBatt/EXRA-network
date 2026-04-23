import { NextRequest, NextResponse } from 'next/server';
import { createClient } from '@supabase/supabase-js';
import { validateCsrfToken } from '@/lib/csrf';

/**
 * SSR-level middleware for:
 * 1. Protecting /admin route (authentication + authorization)
 * 2. Validating CSRF tokens on mutable endpoints (POST/PUT/DELETE)
 * 
 * This middleware runs on the server BEFORE the page is rendered,
 * preventing unauthorized users from seeing the admin UI at all.
 * 
 * Security checks:
 * 1. Verify Supabase session exists (for /admin)
 * 2. Verify user has 'admin' role in user_metadata (for /admin)
 * 3. Validate CSRF token on POST/PUT/DELETE requests
 * 4. Redirect to /auth if checks fail
 */

export async function middleware(request: NextRequest) {
  // Check CSRF token for mutable requests (POST/PUT/DELETE)
  if (['POST', 'PUT', 'DELETE'].includes(request.method)) {
    // Skip CSRF check for specific endpoints that don't need it
    const pathname = request.nextUrl.pathname;
    const skipCsrfPaths = [
      '/buyer-api/auth/set',
      '/buyer-api/auth/clear',
      '/api/auth',
    ];

    const shouldSkipCsrf = skipCsrfPaths.some(path => pathname.includes(path));

    if (!shouldSkipCsrf) {
      const csrfToken = request.headers.get('x-csrf-token');

      if (!csrfToken || !validateCsrfToken(csrfToken)) {
        console.warn(`[SECURITY] CSRF validation failed for ${request.method} ${pathname}`);
        return NextResponse.json(
          { error: 'csrf validation failed' },
          { status: 403 }
        );
      }
    }
  }

  // Protect /marketplace route (requires authentication)
  if (request.nextUrl.pathname.startsWith('/marketplace')) {
    try {
      const token = request.cookies.get('sb-access-token')?.value;

      if (!token) {
        console.warn('[SECURITY] /marketplace access attempt without token');
        return NextResponse.redirect(new URL('/auth', request.url));
      }

      // Verify token with Supabase
      const supabase = createClient(
        process.env.NEXT_PUBLIC_SUPABASE_URL || '',
        process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY ||
          process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ||
          ''
      );

      const {
        data: { user },
        error,
      } = await supabase.auth.getUser(token);

      if (error || !user) {
        console.warn('[SECURITY] /marketplace token verification failed:', error?.message);
        return NextResponse.redirect(new URL('/auth', request.url));
      }

      // User is authenticated - allow access
      return NextResponse.next();
    } catch (error) {
      console.error('[SECURITY] Middleware error on /marketplace:', error);
      return NextResponse.redirect(new URL('/auth', request.url));
    }
  }

  // Protect /admin routes
  if (request.nextUrl.pathname.startsWith('/admin')) {
    try {
      // Get the session token from cookies
      const token = request.cookies.get('sb-access-token')?.value;

      if (!token) {
        console.warn('[SECURITY] /admin access attempt without token');
        return NextResponse.redirect(new URL('/auth', request.url));
      }

      // Verify token with Supabase
      const supabase = createClient(
        process.env.NEXT_PUBLIC_SUPABASE_URL || '',
        process.env.NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY ||
          process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY ||
          ''
      );

      const {
        data: { user },
        error,
      } = await supabase.auth.getUser(token);

      if (error || !user) {
        console.warn('[SECURITY] /admin token verification failed:', error?.message);
        return NextResponse.redirect(new URL('/auth', request.url));
      }

      // Check if user has admin role
      const role = user.user_metadata?.role;
      if (role !== 'admin') {
        console.warn(`[SECURITY] /admin access denied for user ${user.id} with role ${role}`);
        return NextResponse.redirect(new URL('/auth', request.url));
      }

      // User is authenticated and has admin role - allow access
      return NextResponse.next();
    } catch (error) {
      console.error('[SECURITY] Middleware error:', error);
      return NextResponse.redirect(new URL('/auth', request.url));
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: ['/admin/:path*', '/buyer-api/:path*', '/api/:path*'],
};
