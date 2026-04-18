"use client";

import * as React from "react";
import Link from "next/link";
import { ArrowRight, Github, BookOpen } from "lucide-react";
import { Button } from "@/components/ui/button";
import { GlassCard } from "@/components/ui/glass-card";
import { AnimatedSection, AnimatedItem } from "@/components/animated";

export function FinalCta() {
  return (
    <section className="relative py-32 sm:py-40">
      <div
        aria-hidden
        className="absolute inset-x-0 top-1/2 -translate-y-1/2 -z-10 h-[480px] mx-auto max-w-5xl rounded-full opacity-40"
        style={{
          background:
            "radial-gradient(ellipse at center, rgba(34,211,238,0.4) 0%, rgba(167,139,250,0.2) 40%, transparent 70%)",
          filter: "blur(80px)",
        }}
      />

      <AnimatedSection className="container">
        <AnimatedItem>
          <GlassCard elevated padding="none" className="overflow-hidden">
            <div className="relative px-8 sm:px-16 py-20 sm:py-28 text-center">
              <div
                aria-hidden
                className="absolute inset-0 -z-10 bg-grid opacity-50"
              />

              <h2 className="text-display-xl text-balance max-w-3xl mx-auto">
                <span className="text-gradient">The network is </span>
                <span className="text-neon-gradient">already on.</span>
                <br />
                <span className="text-gradient">Plug in.</span>
              </h2>

              <p className="mt-8 max-w-xl mx-auto text-lg text-ink-muted leading-relaxed">
                48,217 nodes across 63 countries. $48,920 paid out yesterday.
                Run a node in two minutes, no waitlist.
              </p>

              <div className="mt-12 flex flex-col sm:flex-row items-center justify-center gap-3">
                <Button variant="primary" size="lg" asChild>
                  <Link href="https://app.exra.space/start">
                    Run a node
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                </Button>
                <Button variant="secondary" size="lg" asChild>
                  <Link href="https://docs.exra.space">
                    <BookOpen className="h-4 w-4" />
                    Read the whitepaper
                  </Link>
                </Button>
                <Button variant="ghost" size="lg" asChild>
                  <Link href="https://github.com/exra">
                    <Github className="h-4 w-4" />
                    GitHub
                  </Link>
                </Button>
              </div>
            </div>
          </GlassCard>
        </AnimatedItem>
      </AnimatedSection>
    </section>
  );
}
