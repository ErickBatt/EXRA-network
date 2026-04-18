"use client";

import * as React from "react";
import {
  Network,
  Lock,
  Cpu,
  TrendingUp,
  Shuffle,
  Eye,
} from "lucide-react";
import { GlassCard } from "@/components/ui/glass-card";
import { Badge } from "@/components/ui/badge";
import { AnimatedSection, AnimatedItem } from "@/components/animated";

const FEATURES = [
  {
    icon: Network,
    title: "Mesh on peaq L1",
    body:
      "Every node is a Ed25519 DID anchored on peaq. Routing, billing and reputation live on-chain — no central operator can blacklist you.",
    accent: "neon" as const,
    span: "lg:col-span-2",
  },
  {
    icon: Lock,
    title: "Zero data harvesting",
    body:
      "Traffic is tunneled, never logged. Headers stripped at the gateway, payload encrypted node-to-buyer.",
    accent: "violet" as const,
    span: "",
  },
  {
    icon: Cpu,
    title: "Bandwidth + compute",
    body:
      "Sell your idle GPU cycles to inference workloads. Same node, two revenue streams.",
    accent: "neon" as const,
    span: "",
  },
  {
    icon: TrendingUp,
    title: "Tiered referrals",
    body:
      "10% → 30% commission across four tiers. Build a network, compound your earn rate.",
    accent: "violet" as const,
    span: "",
  },
  {
    icon: Shuffle,
    title: "Multi-chain payouts",
    body:
      "Withdraw to TON, Solana or peaq native. Settled atomically, $1 minimum, no KYC under regulatory thresholds.",
    accent: "neon" as const,
    span: "lg:col-span-2",
  },
  {
    icon: Eye,
    title: "Transparent oracle",
    body:
      "Every byte counted, every payout traceable. Sr25519 oracle signatures with 2/3 consensus before mint.",
    accent: "violet" as const,
    span: "",
  },
];

export function Features() {
  return (
    <section id="features" className="relative py-32 sm:py-40">
      {/* Section-level glow accents */}
      <div
        aria-hidden
        className="absolute left-1/2 top-0 -translate-x-1/2 -z-10 h-[500px] w-[700px] rounded-full opacity-30"
        style={{
          background:
            "radial-gradient(ellipse at center, rgba(167,139,250,0.3) 0%, transparent 70%)",
          filter: "blur(60px)",
        }}
      />

      <AnimatedSection className="container">
        <div className="max-w-2xl mx-auto text-center">
          <AnimatedItem className="flex justify-center">
            <Badge variant="neon">Architecture</Badge>
          </AnimatedItem>
          <AnimatedItem>
            <h2 className="mt-5 text-display-lg text-balance">
              <span className="text-gradient">Built like infrastructure.</span>
              <br />
              <span className="text-neon-gradient">Earned like a wage.</span>
            </h2>
          </AnimatedItem>
          <AnimatedItem>
            <p className="mt-6 text-lg text-ink-muted leading-relaxed">
              Six design choices that separate EXRA from yet-another-proxy-app.
              All on-chain. All open source. All audited.
            </p>
          </AnimatedItem>
        </div>

        <div className="mt-20 grid gap-5 sm:gap-6 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
          {FEATURES.map((feature, i) => {
            const Icon = feature.icon;
            return (
              <AnimatedItem key={feature.title} className={feature.span}>
                <GlassCard
                  interactive
                  glow={feature.accent}
                  padding="lg"
                  className="h-full group"
                >
                  <div className="flex flex-col h-full gap-5">
                    <div
                      className={`h-12 w-12 rounded-xl glass-elevated flex items-center justify-center transition-all duration-300 group-hover:scale-110 ${
                        feature.accent === "violet"
                          ? "group-hover:shadow-[0_0_24px_rgba(167,139,250,0.4)]"
                          : "group-hover:shadow-[0_0_24px_rgba(34,211,238,0.4)]"
                      }`}
                    >
                      <Icon
                        className={`h-5 w-5 ${
                          feature.accent === "violet"
                            ? "text-neon-violet"
                            : "text-neon-bright"
                        }`}
                      />
                    </div>
                    <h3 className="text-xl font-semibold text-ink tracking-tight leading-tight">
                      {feature.title}
                    </h3>
                    <p className="text-sm text-ink-muted leading-relaxed">
                      {feature.body}
                    </p>
                  </div>
                </GlassCard>
              </AnimatedItem>
            );
          })}
        </div>
      </AnimatedSection>
    </section>
  );
}
