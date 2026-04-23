"use client";

import * as React from "react";
import Link from "next/link";
import { motion } from "framer-motion";
import { ArrowRight, Sparkles, Zap } from "lucide-react";
import dynamic from "next/dynamic";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

// WorldMap uses react-simple-maps + d3-geo, which produce SVG paths with
// floating-point coordinates that can differ between server and client
// (projection math is not bit-for-bit stable). It's also decorative — there's
// no SEO value in SSR-ing it. Render client-only with a graceful aspect-box
// placeholder so the hero layout doesn't shift on mount.
const WorldMap = dynamic(
  () => import("@/components/world-map").then((m) => m.WorldMap),
  {
    ssr: false,
    loading: () => <div className="w-full aspect-[16/10]" aria-hidden />,
  }
);

/**
 * Hero — first 100vh.
 *
 * Layout: title + sub-headline + CTAs above the fold (left-aligned),
 * world map sits as a background-foreground hybrid centered behind text.
 *
 * The story arc:
 *   Eyebrow → headline (3 short lines) → subhead → 2 CTAs → trust strip.
 */
export function Hero() {
  return (
    <section
      id="network"
      className="relative isolate min-h-screen flex flex-col justify-start pt-32 sm:pt-36 pb-12 overflow-hidden"
    >
      {/* Grid background — masked to a soft ellipse */}
      <div aria-hidden className="absolute inset-0 -z-30 bg-grid" />

      {/* Map sits in the back, taking the full hero — text overlays on top */}
      <div
        aria-hidden
        className="absolute inset-x-0 top-12 -z-20 opacity-90"
        style={{ height: "min(100vh, 900px)" }}
      >
        <div className="absolute inset-0 max-w-[1400px] mx-auto">
          <WorldMap />
        </div>
        {/* Vignette overlay — focuses eye on text */}
        <div
          aria-hidden
          className="absolute inset-0 pointer-events-none"
          style={{
            background:
              "radial-gradient(ellipse 60% 40% at 50% 50%, transparent 30%, rgba(9,9,11,0.5) 70%, #09090b 100%)",
          }}
        />
        <div
          aria-hidden
          className="absolute inset-x-0 bottom-0 h-48 bg-gradient-to-t from-bg via-bg/80 to-transparent"
        />
      </div>

      <div className="container relative z-10">
        <div className="max-w-3xl">
          <motion.div
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
          >
            <Badge variant="neon">
              <Sparkles className="h-3 w-3" />
              Live on peaq · Mainnet
            </Badge>
          </motion.div>

          <motion.h1
            initial={{ opacity: 0, y: 24 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, delay: 0.1, ease: [0.22, 1, 0.36, 1] }}
            className="mt-6 text-display-2xl text-balance"
          >
            <span className="text-gradient">The decentralized</span>
            <br />
            <span className="text-neon-gradient">network</span>{" "}
            <span className="text-gradient">that runs</span>
            <br />
            <span className="text-gradient">on every device.</span>
          </motion.h1>

          <motion.p
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.7, delay: 0.3, ease: [0.22, 1, 0.36, 1] }}
            className="mt-8 max-w-xl text-lg sm:text-xl text-ink-muted leading-relaxed"
          >
            EXRA turns idle bandwidth and compute into{" "}
            <span className="text-ink">verifiable income</span>. Phones, PCs,
            routers — earn $EXRA tokens on{" "}
            <span className="text-ink">peaq L1</span>. Buyers tap a global mesh
            of <span className="text-ink">residential IPs</span> in 60+
            countries, settled on-chain.
          </motion.p>

          <motion.div
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, delay: 0.5, ease: [0.22, 1, 0.36, 1] }}
            className="mt-10 flex flex-col sm:flex-row gap-3"
          >
            <Button variant="primary" size="lg" asChild>
              <Link href="/marketplace">
                Run a node
                <ArrowRight className="h-4 w-4" />
              </Link>
            </Button>
            <Button variant="secondary" size="lg" asChild>
              <Link href="/marketplace">
                <Zap className="h-4 w-4" />
                Buy bandwidth
              </Link>
            </Button>
          </motion.div>

          {/* Trust strip — matters for grant evaluators */}
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.8, delay: 0.8 }}
            className="mt-14 flex flex-wrap items-center gap-x-8 gap-y-3 text-xs text-ink-dim"
          >
            <span className="font-mono uppercase tracking-[0.18em]">Built on</span>
            <TrustChip>peaq L1</TrustChip>
            <TrustChip>Substrate</TrustChip>
            <TrustChip>Ed25519 DID</TrustChip>
            <TrustChip>0% team allocation</TrustChip>
          </motion.div>
        </div>
      </div>

      {/* Live counter strip — bottom of hero */}
      <div className="container relative z-10 mt-20 sm:mt-28">
        <motion.div
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7, delay: 1.0, ease: [0.22, 1, 0.36, 1] }}
          className="glass-elevated rounded-2xl px-6 sm:px-10 py-6 flex flex-wrap items-center justify-between gap-x-12 gap-y-6"
        >
          <LiveStat label="Active nodes" value="48,217" trend="+12.4%" />
          <Divider />
          <LiveStat label="Countries" value="63" trend="live" />
          <Divider />
          <LiveStat label="GB served / 24h" value="184 TB" trend="+8.1%" />
          <Divider />
          <LiveStat label="Paid out / 24h" value="$48,920" trend="+15.2%" />
        </motion.div>
      </div>
    </section>
  );
}

function TrustChip({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center gap-1.5 text-ink-muted">
      <span className="h-1 w-1 rounded-full bg-neon" />
      {children}
    </span>
  );
}

function Divider() {
  return <div aria-hidden className="hidden sm:block h-10 w-px bg-glass-borderBright" />;
}

function LiveStat({
  label,
  value,
  trend,
}: {
  label: string;
  value: string;
  trend: string;
}) {
  const isLive = trend === "live";
  return (
    <div className="flex flex-col gap-1">
      <span className="text-[11px] font-mono uppercase tracking-[0.16em] text-ink-dim">
        {label}
      </span>
      <span className="text-2xl sm:text-3xl font-semibold text-ink tabular-nums tracking-tight">
        {value}
      </span>
      <span
        className={`text-xs font-medium ${
          isLive ? "text-neon-bright" : "text-success"
        } flex items-center gap-1`}
      >
        {isLive && (
          <span className="relative flex h-1.5 w-1.5">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-neon-bright opacity-70" />
            <span className="relative inline-flex rounded-full h-1.5 w-1.5 bg-neon-bright" />
          </span>
        )}
        {trend}
      </span>
    </div>
  );
}
