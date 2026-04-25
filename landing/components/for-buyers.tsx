"use client";

import * as React from "react";
import Link from "next/link";
import { ArrowRight, Globe2, ShieldCheck, Zap } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { GlassCard } from "@/components/ui/glass-card";
import { AnimatedSection, AnimatedItem } from "@/components/animated";

const FEATURES = [
  {
    icon: Globe2,
    title: "63 countries, real IPs",
    body: "Every node is a real device on a residential ISP. Bypasses geo-blocks that datacenter proxies can't touch.",
  },
  {
    icon: ShieldCheck,
    title: "Clean traffic, on-chain proof",
    body: "PoP attestation on peaq L1 means every GB is verifiable. No black-box pools, no fake nodes.",
  },
  {
    icon: Zap,
    title: "REST + WebSocket API",
    body: "Pay per GB, no contracts. Filter by country, tier, and bandwidth. Auto-route to the fastest node.",
  },
];

export function ForBuyers() {
  return (
    <section id="buyers" className="relative py-32 sm:py-40">
      <div
        aria-hidden
        className="absolute inset-x-0 top-1/2 -translate-y-1/2 -z-10 h-[600px] max-w-4xl mx-auto"
        style={{
          background:
            "radial-gradient(ellipse at 80% 50%, rgba(167,139,250,0.12) 0%, transparent 70%)",
          filter: "blur(60px)",
        }}
      />

      <AnimatedSection className="container">
        <div className="grid lg:grid-cols-2 gap-16 lg:gap-24 items-center">
          {/* Left — copy */}
          <div>
            <AnimatedItem>
              <Badge variant="violet">For Buyers</Badge>
            </AnimatedItem>
            <AnimatedItem>
              <h2 className="mt-5 text-display-lg text-balance">
                <span className="text-gradient">48,000+ residential IPs.</span>
                <br />
                <span className="text-neon-gradient">Pay per gigabyte.</span>
              </h2>
            </AnimatedItem>
            <AnimatedItem>
              <p className="mt-6 text-lg text-ink-muted leading-relaxed">
                Residential proxies that look real because they are.
                Used by AI scrapers, price monitors, arbitrageurs, and data teams
                that can't afford to get blocked.
              </p>
            </AnimatedItem>
            <AnimatedItem>
              <div className="mt-8 flex flex-col sm:flex-row gap-3">
                <Button variant="primary" size="lg" asChild>
                  <Link href="https://app.exra.space">
                    Browse the marketplace
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                </Button>
                <Button variant="secondary" size="lg" asChild>
                  <Link href="https://docs.exra.space/api" target="_blank" rel="noopener noreferrer">
                    API docs
                  </Link>
                </Button>
              </div>
            </AnimatedItem>
          </div>

          {/* Right — feature cards */}
          <div className="flex flex-col gap-4">
            {FEATURES.map((f) => {
              const Icon = f.icon;
              return (
                <AnimatedItem key={f.title}>
                  <GlassCard interactive glow="violet" padding="lg">
                    <div className="flex gap-5 items-start">
                      <div className="h-10 w-10 shrink-0 rounded-xl glass-elevated flex items-center justify-center">
                        <Icon className="h-5 w-5 text-neon-violet" />
                      </div>
                      <div>
                        <h3 className="text-base font-semibold text-ink tracking-tight">
                          {f.title}
                        </h3>
                        <p className="mt-1.5 text-sm text-ink-muted leading-relaxed">
                          {f.body}
                        </p>
                      </div>
                    </div>
                  </GlassCard>
                </AnimatedItem>
              );
            })}
          </div>
        </div>
      </AnimatedSection>
    </section>
  );
}
