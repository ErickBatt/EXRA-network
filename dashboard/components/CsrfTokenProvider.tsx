'use client';

import { useEffect } from 'react';
import { generateCsrfToken } from '@/lib/csrf';

/**
 * Client-side component that generates and stores CSRF token on page load.
 * Token is stored in sessionStorage and sent with all POST/PUT/DELETE requests.
 */
export default function CsrfTokenProvider({ children }: { children: React.ReactNode }) {
  useEffect(() => {
    const initCsrfToken = async () => {
      try {
        // Generate new CSRF token
        const token = generateCsrfToken();

        // Store in sessionStorage (cleared on browser close)
        if (typeof window !== 'undefined') {
          sessionStorage.setItem('csrf_token', token);
        }
      } catch (error) {
        console.error('[CSRF] Failed to initialize token:', error);
      }
    };

    initCsrfToken();
  }, []);

  return <>{children}</>;
}
