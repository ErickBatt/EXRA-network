"use client";

import * as React from "react";
import { Smartphone, Laptop, MonitorPlay, Router } from "lucide-react";
import { GlassCard } from "@/components/ui/glass-card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { AnimatedSection, AnimatedItem } from "@/components/animated";
import { ArrowRight } from "lucide-react";

const DEVICES = [
  {
    icon: Smartphone,
    name: "Phone",
    detail: "iOS / Android",
    perDay: "$1.20",
    perMonth: "~$36",
    bandwidth: "TIER 1 proxy",
    accent: "neon" as const,
  },
  {
    icon: Laptop,
    name: "Laptop",
    detail: "macOS / Windows / Linux",
    perDay: "$2.40",
    perMonth: "~$72",
    bandwidth: "TIER 1-2 proxy + compute",
    accent: "violet" as const,
  },
  {
    icon: MonitorPlay,
    name: "Gaming PC",
    detail: "GPU-equipped, 24/7",
    perDay: "$8.00",
    perMonth: "~$240",
    bandwidth: "TIER 2 GPU + proxy",
    accent: "neon" as const,
    highlight: true,
  },
  {
    icon: Router,
    name: "Router / RPi",
    detail: "Always-on edge",
    perDay: "$0.60",
    perMonth: "~$18",
    bandwidth: "TIER 1 24/7 proxy",
    accent: "violet" as const,
  },
];

export function Earnings() {
  return (
    <section id="earn" className="relative py-32 sm:py-40">
      <AnimatedSection className="container">
        <div className="grid lg:grid-cols-12 gap-12 lg:gap-16 items-end">
          <div className="lg:col-span-5">
            <AnimatedItem>
              <Badge variant="neon">Earn</Badge>
            </AnimatedItem>
            <AnimatedItem>
              <h2 className="mt-5 text-display-lg text-balance">
                <span className="text-gradient">Every device </span>
                <span className="text-neon-gradient">a paycheck.</span>
              </h2>
            </AnimatedItem>
            <AnimatedItem>
              <p className="mt-6 text-lg text-ink-muted leading-relaxed max-w-md">
                Numbers are 30-day medians from real EXRA workers in the launch
                corridor. Your mileage depends on uptime, geo, and tier.
              </p>
            </AnimatedItem>
            <AnimatedItem>
              <Button variant="primary" size="lg" className="mt-10">
                <span className="flex items-center gap-2">
                  Estimate my device
                  <ArrowRight className="h-4 w-4" />
                </span>
              </Button>
            </AnimatedItem>
          </div>

          <div className="lg:col-span-7 grid grid-cols-1 sm:grid-cols-2 gap-4 sm:gap-5">
            {DEVICES.map((device) => {
              const Icon = device.icon;
              return (
                <AnimatedItem key={device.name}>
                  <GlassCard
                    interactive
                    glow={device.accent}
                    elevated={device.highlight}
                    padding="md"
                    className="h-full"
                  >
                    <div className="flex flex-col gap-5">
                      <div className="flex items-start justify-between">
                        <div className="h-10 w-10 rounded-lg glass-elevated flex items-center justify-center">
                          <Icon
                            className={`h-4 w-4 ${
                              device.accent === "violet"
                                ? "text-neon-violet"
                                : "text-neon-bright"
                            }`}
                          />
                        </div>
                        {device.highlight && (
                          <Badge variant="neon" className="text-[10px]">
                            Top earner
                          </Badge>
                        )}
                      </div>
                      <div>
                        <div className="text-base font-semibold text-ink leading-none">
                          {device.name}
                        </div>
                        <div className="mt-1.5 text-xs text-ink-dim">{device.detail}</div>
                      </div>
                      <div className="hairline opacity-60" />
                      <div className="flex items-baseline gap-2">
                        <span className="text-3xl font-semibold text-ink tabular-nums tracking-tight">
                          {device.perDay}
                        </span>
                        <span className="text-xs text-ink-dim font-mono uppercase tracking-wider">
                          / day
                        </span>
                      </div>
                      <div className="flex items-center justify-between text-xs">
                        <span className="text-ink-dim">{device.bandwidth}</span>
                        <span className="text-ink-muted font-mono tabular-nums">
                          {device.perMonth}
                        </span>
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
