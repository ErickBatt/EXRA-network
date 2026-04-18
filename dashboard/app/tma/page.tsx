"use client"

// TMA route — @twa-dev/sdk touches `window.Telegram` at module top-level,
// which crashes SSR. Load TMAApp client-only to skip the server render pass.
// NODE_SECRET never reaches the browser; all /api/tma/* calls go through /next-tma/*.
import dynamic from "next/dynamic"

const TMAApp = dynamic(() => import("./TMAApp"), { ssr: false })

export default function TmaPage() {
  return <TMAApp />
}
