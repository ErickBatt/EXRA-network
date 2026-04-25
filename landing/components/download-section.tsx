"use client";

import * as React from "react";
import Link from "next/link";
import { Send, Smartphone, Monitor, ArrowRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { GlassCard } from "@/components/ui/glass-card";
import { AnimatedSection, AnimatedItem } from "@/components/animated";
import { cn } from "@/lib/utils";

const PLATFORMS = [
  {
    icon: Send,
    name: "Telegram Mini App",
    tag: "Fastest start",
    tagVariant: "neon" as const,
    description:
      "Open EXRA right inside Telegram. No install, no APK — just tap and start earning.",
    cta: "Open in Telegram",
    href: "https://t.me/exranetworkbot",
    external: true,
    download: undefined,
    disabled: false,
    glowColor: "neon",
  },
  {
    icon: Smartphone,
    name: "Android App",
    tag: "Full node",
    tagVariant: "success" as const,
    description:
      "Runs as a background service. Earns while you sleep, charges nothing you don't use.",
    cta: "Download APK",
    href: "/exra-node.apk",
    external: false,
    download: "EXRA-Node.apk",
    disabled: false,
    glowColor: "neon",
  },
  {
    icon: Monitor,
    name: "Desktop Agent",
    tag: "Coming soon",
    tagVariant: "default" as const,
    description:
      "Windows & Linux. GPU compute tasks on top of bandwidth. Sign up for early access.",
    cta: "Notify me",
    href: "https://app.exra.space/waitlist",
    external: true,
    download: undefined,
    disabled: true,
    glowColor: undefined,
  },
];

export function DownloadSection() {
  return (
    <section id="download" className="relative py-32 sm:py-40">
      <div
        aria-hidden
        className="absolute inset-x-0 top-1/2 -translate-y-1/2 -z-10 h-[500px] max-w-5xl mx-auto"
        style={{
          background:
            "radial-gradient(ellipse at 30% 50%, rgba(34,211,238,0.1) 0%, transparent 70%)",
          filter: "blur(60px)",
        }}
      />

      <AnimatedSection className="container">
        <div className="max-w-2xl">
          <AnimatedItem>
            <Badge variant="neon">Get started</Badge>
          </AnimatedItem>
          <AnimatedItem>
            <h2 className="mt-5 text-display-lg text-balance">
              <span className="text-gradient">Up and running </span>
              <span className="text-neon-gradient">in 60 seconds.</span>
            </h2>
          </AnimatedItem>
          <AnimatedItem>
            <p className="mt-6 text-lg text-ink-muted leading-relaxed">
              No KYC. No waitlist. No seed phrase on setup. Pick your platform
              and plug in — withdraw from $1.
            </p>
          </AnimatedItem>
        </div>

        <div className="mt-16 grid gap-5 sm:gap-6 grid-cols-1 md:grid-cols-3">
          {PLATFORMS.map((p) => {
            const Icon = p.icon;
            return (
              <AnimatedItem key={p.name}>
                <GlassCard
                  interactive={!p.disabled}
                  glow={p.disabled ? undefined : (p.glowColor as "neon" | "violet" | undefined)}
                  padding="lg"
                  className={cn("h-full flex flex-col", p.disabled && "opacity-50")}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="h-12 w-12 rounded-xl glass-elevated flex items-center justify-center shrink-0">
                      <Icon className={cn(
                        "h-6 w-6",
                        p.disabled ? "text-ink-dim" : "text-neon-bright"
                      )} />
                    </div>
                    <Badge variant={p.tagVariant}>{p.tag}</Badge>
                  </div>

                  <h3 className="mt-5 text-xl font-semibold text-ink tracking-tight">
                    {p.name}
                  </h3>
                  <p className="mt-3 text-sm text-ink-muted leading-relaxed flex-1">
                    {p.description}
                  </p>

                  <div className="mt-6">
                    <Button
                      variant={p.disabled ? "ghost" : "primary"}
                      size="sm"
                      className="w-full"
                      asChild={!p.disabled}
                      disabled={p.disabled}
                    >
                      {p.disabled ? (
                        <span>{p.cta}</span>
                      ) : (
                        <Link
                          href={p.href}
                          target={p.external ? "_blank" : undefined}
                          rel={p.external ? "noopener noreferrer" : undefined}
                          download={p.download ?? undefined}
                        >
                          {p.cta}
                          <ArrowRight className="h-3.5 w-3.5" />
                        </Link>
                      )}
                    </Button>
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
