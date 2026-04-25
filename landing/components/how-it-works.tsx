"use client";

import * as React from "react";
import { Smartphone, Globe2, Coins, ShieldCheck } from "lucide-react";
import { GlassCard } from "@/components/ui/glass-card";
import { Badge } from "@/components/ui/badge";
import { AnimatedSection, AnimatedItem } from "@/components/animated";

const STEPS = [
  {
    icon: Smartphone,
    step: "01",
    title: "Install on any device",
    body:
      "Phone, laptop, gaming PC, router, RPi. The EXRA node runs in the background, capped at policies you set.",
  },
  {
    icon: Globe2,
    step: "02",
    title: "Join the global mesh",
    body:
      "Your IP and idle compute become a verified DID on peaq L1. Buyers route real traffic through your node — never logs, never sells data.",
  },
  {
    icon: ShieldCheck,
    step: "03",
    title: "Proof of Presence",
    body:
      "Every 5 minutes the network signs a heartbeat under your Ed25519 key. PoP is what proves you served traffic — and what mints rewards.",
  },
  {
    icon: Coins,
    step: "04",
    title: "Earn $EXRA, withdraw from $1",
    body:
      "50% of every paid byte goes to the worker. 10–30% to your referrer. Withdraw in USDT or $EXRA via peaq — minimum $1, no lock-up.",
  },
];

export function HowItWorks() {
  return (
    <section id="how" className="relative py-32 sm:py-40">
      <AnimatedSection className="container">
        <div className="max-w-2xl">
          <AnimatedItem>
            <Badge variant="violet">How it works</Badge>
          </AnimatedItem>
          <AnimatedItem>
            <h2 className="mt-5 text-display-lg text-balance">
              <span className="text-gradient">From your device to a </span>
              <span className="text-neon-gradient">verifiable income stream</span>
              <span className="text-gradient"> — in four steps.</span>
            </h2>
          </AnimatedItem>
          <AnimatedItem>
            <p className="mt-6 text-lg text-ink-muted max-w-xl leading-relaxed">
              No mining rigs. No special hardware. EXRA runs on what you already
              own — and pays you in proportion to what the network actually
              served.
            </p>
          </AnimatedItem>
        </div>

        <div className="mt-20 grid gap-5 sm:gap-6 grid-cols-1 md:grid-cols-2 lg:grid-cols-4">
          {STEPS.map((step) => {
            const Icon = step.icon;
            return (
              <AnimatedItem key={step.step}>
                <GlassCard interactive glow="neon" padding="lg" className="h-full">
                  <div className="flex flex-col h-full gap-5">
                    <div className="flex items-center justify-between">
                      <div className="h-11 w-11 rounded-xl glass-elevated flex items-center justify-center">
                        <Icon className="h-5 w-5 text-neon-bright" />
                      </div>
                      <span className="font-mono text-xs text-ink-dim tracking-[0.16em]">
                        {step.step}
                      </span>
                    </div>
                    <h3 className="text-xl font-semibold text-ink tracking-tight leading-tight">
                      {step.title}
                    </h3>
                    <p className="text-sm text-ink-muted leading-relaxed flex-1">
                      {step.body}
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
