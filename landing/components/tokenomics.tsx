"use client";

import * as React from "react";
import { GlassCard } from "@/components/ui/glass-card";
import { Badge } from "@/components/ui/badge";
import { AnimatedSection, AnimatedItem } from "@/components/animated";

const SPLIT = [
  { label: "Worker", pct: 50, color: "from-neon-bright to-neon" },
  { label: "Referrer", pct: 20, color: "from-neon to-neon-violet" },
  { label: "Treasury", pct: 18, color: "from-neon-violet to-neon-violet-deep" },
  { label: "Buyback / burn", pct: 12, color: "from-neon-violet-deep to-neon-violet" },
];

const TIERS = [
  { name: "Street Scout", refs: "1–100", commission: "10%" },
  { name: "Network Builder", refs: "101–300", commission: "15%" },
  { name: "Crypto Boss", refs: "301–600", commission: "20%" },
  { name: "Ambassador", refs: "601–1000", commission: "30%" },
];

export function Tokenomics() {
  return (
    <section id="tokenomics" className="relative py-32 sm:py-40">
      <AnimatedSection className="container">
        <div className="max-w-2xl">
          <AnimatedItem>
            <Badge variant="violet">Tokenomics</Badge>
          </AnimatedItem>
          <AnimatedItem>
            <h2 className="mt-5 text-display-lg text-balance">
              <span className="text-gradient">Zero team premine. </span>
              <span className="text-neon-gradient">Workers earn first.</span>
            </h2>
          </AnimatedItem>
          <AnimatedItem>
            <p className="mt-6 text-lg text-ink-muted leading-relaxed">
              $EXRA mints only when traffic is served. No allocation to founders,
              no VC unlocks. Every block, every byte — accounted for on peaq L1.
            </p>
          </AnimatedItem>
        </div>

        <div className="mt-16 grid lg:grid-cols-12 gap-6">
          {/* Distribution split — visual stacked bar */}
          <AnimatedItem className="lg:col-span-7">
            <GlassCard padding="lg" elevated className="h-full">
              <div className="flex items-baseline justify-between mb-8">
                <h3 className="text-xl font-semibold text-ink tracking-tight">
                  Per-byte distribution
                </h3>
                <span className="font-mono text-xs uppercase tracking-[0.16em] text-ink-dim">
                  Each $1 paid by a buyer
                </span>
              </div>

              {/* Stacked horizontal bar */}
              <div className="flex h-14 rounded-xl overflow-hidden border border-glass-borderBright">
                {SPLIT.map((s) => (
                  <div
                    key={s.label}
                    className={`relative bg-gradient-to-r ${s.color}`}
                    style={{ width: `${s.pct}%` }}
                  >
                    <div className="absolute inset-0 shimmer-line opacity-30" />
                  </div>
                ))}
              </div>

              {/* Legend */}
              <div className="mt-8 grid grid-cols-2 sm:grid-cols-4 gap-4">
                {SPLIT.map((s) => (
                  <div key={s.label} className="flex flex-col gap-1">
                    <div className="flex items-center gap-2">
                      <div className={`h-2 w-2 rounded-full bg-gradient-to-r ${s.color}`} />
                      <span className="text-xs text-ink-dim font-mono uppercase tracking-wider">
                        {s.label}
                      </span>
                    </div>
                    <span className="text-2xl font-semibold text-ink tabular-nums">
                      {s.pct}%
                    </span>
                  </div>
                ))}
              </div>
            </GlassCard>
          </AnimatedItem>

          {/* Referral tiers */}
          <AnimatedItem className="lg:col-span-5">
            <GlassCard padding="lg" glow="violet" className="h-full">
              <h3 className="text-xl font-semibold text-ink tracking-tight">
                Referral tiers
              </h3>
              <p className="mt-2 text-sm text-ink-muted">
                Bring nodes online, take a cut of their earnings — forever.
              </p>
              <div className="mt-6 flex flex-col">
                {TIERS.map((tier, i) => (
                  <div
                    key={tier.name}
                    className={`flex items-center justify-between py-4 ${
                      i < TIERS.length - 1 ? "border-b border-glass-border" : ""
                    }`}
                  >
                    <div className="flex flex-col">
                      <span className="text-sm font-semibold text-ink">{tier.name}</span>
                      <span className="text-xs text-ink-dim font-mono mt-0.5">
                        {tier.refs} referrals
                      </span>
                    </div>
                    <div className="flex items-baseline gap-1">
                      <span className="text-2xl font-semibold text-neon-bright tabular-nums">
                        {tier.commission}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            </GlassCard>
          </AnimatedItem>
        </div>
      </AnimatedSection>
    </section>
  );
}
