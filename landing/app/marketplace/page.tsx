"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

/**
 * Marketplace redirect page
 * Redirects to app.exra.space (dashboard with marketplace)
 */
export default function MarketplacePage() {
  const router = useRouter();

  useEffect(() => {
    // Redirect to the actual marketplace on app.exra.space
    window.location.href = "https://app.exra.space";
  }, []);

  return (
    <div className="min-h-screen flex items-center justify-center bg-zinc-950">
      <div className="text-center">
        <div className="inline-block animate-spin rounded-full h-12 w-12 border-b-2 border-cyan-400 mb-4"></div>
        <p className="text-zinc-400">Redirecting to marketplace...</p>
      </div>
    </div>
  );
}
