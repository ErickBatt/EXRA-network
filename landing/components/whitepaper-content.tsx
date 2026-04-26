"use client";

import * as React from "react";
import Link from "next/link";
import {
  Download,
  ArrowLeft,
  ArrowRight,
  Shield,
  Zap,
  TrendingUp,
  Coins,
  Lock,
  CheckCircle2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { GlassCard } from "@/components/ui/glass-card";
import { AnimatedSection, AnimatedItem } from "@/components/animated";

const TOC = [
  { id: "abstract", label: "1. Abstract" },
  { id: "problem", label: "2. Problem Statement" },
  { id: "solution", label: "3. Solution & Product" },
  { id: "market", label: "4. Market & Unit Economics" },
  { id: "tokenomics", label: "5. Tokenomics" },
  { id: "governance", label: "6. Governance & Security" },
];

export function WhitepaperContent() {
  const [activeSection, setActiveSection] = React.useState("abstract");

  React.useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            setActiveSection(entry.target.id);
          }
        }
      },
      { rootMargin: "-30% 0px -60% 0px" }
    );
    TOC.forEach(({ id }) => {
      const el = document.getElementById(id);
      if (el) observer.observe(el);
    });
    return () => observer.disconnect();
  }, []);

  return (
    <div className="min-h-screen pt-24 pb-32">
      {/* ── Hero ── */}
      <AnimatedSection className="container">
        <AnimatedItem>
          <div className="mx-auto max-w-4xl text-center">
            <div className="flex flex-wrap items-center justify-center gap-2 mb-6">
              <Badge variant="neon">v2.5 · Sovereign Edition</Badge>
              <Badge variant="violet">Production Spec</Badge>
              <Badge variant="success">Audit v2.4.1 Passed</Badge>
            </div>

            <h1 className="text-display-xl text-balance">
              <span className="text-gradient">EXRA White Paper:</span>
              <br />
              <span className="text-neon-gradient">
                Sovereign DePIN Infrastructure
              </span>
            </h1>

            <p className="mt-6 text-ink-muted text-lg max-w-2xl mx-auto leading-relaxed">
              Blockchain: peaq L1 (Polkadot Ecosystem) · Date: April 22, 2026
              · Technical readiness: 100%
            </p>

            <div className="mt-8 flex flex-col sm:flex-row items-center justify-center gap-3">
              <Button variant="primary" size="lg" asChild>
                <a href="/whitepaper.pdf" download="EXRA-Whitepaper-v2.5.pdf">
                  <Download className="h-4 w-4" />
                  Download PDF
                </a>
              </Button>
              <Button variant="ghost" size="lg" asChild>
                <Link href="/">
                  <ArrowLeft className="h-4 w-4" />
                  Back to home
                </Link>
              </Button>
            </div>
          </div>
        </AnimatedItem>
      </AnimatedSection>

      {/* ── Body: sidebar + content ── */}
      <div className="container mt-16 lg:mt-20">
        <div className="mx-auto max-w-6xl flex gap-10 items-start">

          {/* Sticky TOC sidebar */}
          <aside className="hidden lg:block w-52 shrink-0 sticky top-28 self-start">
            <Link
              href="/"
              className="flex items-center gap-2 text-sm text-ink-muted hover:text-neon transition-colors mb-6"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              Back to home
            </Link>
            <p className="text-xs font-semibold uppercase tracking-widest text-ink-dim mb-4">
              Contents
            </p>
            <nav className="space-y-1">
              {TOC.map(({ id, label }) => (
                <a
                  key={id}
                  href={`#${id}`}
                  className={`block text-sm py-1.5 px-3 rounded-lg transition-colors ${
                    activeSection === id
                      ? "text-neon bg-neon/8 border-l-2 border-neon"
                      : "text-ink-muted hover:text-ink"
                  }`}
                >
                  {label}
                </a>
              ))}
            </nav>
            <div className="mt-6 pt-6 border-t border-glass-border">
              <a
                href="/whitepaper.pdf"
                download="EXRA-Whitepaper-v2.5.pdf"
                className="flex items-center gap-2 text-xs text-ink-muted hover:text-neon transition-colors"
              >
                <Download className="h-3.5 w-3.5" />
                Download PDF
              </a>
            </div>
          </aside>

          {/* Main content */}
          <div className="flex-1 min-w-0 space-y-8">

            {/* 1. Abstract */}
            <Section id="abstract" icon={<Zap className="h-5 w-5 text-neon" />} title="1. Abstract">
              <p className="text-ink-muted leading-relaxed">
                EXRA is a decentralized physical infrastructure network (DePIN)
                designed to aggregate idle internet bandwidth and computing power
                from billions of Android devices. Leveraging a custom pallet on the{" "}
                <span className="text-neon font-medium">peaq</span> blockchain and a
                unique two-tier trust architecture, EXRA delivers censorship-resistant
                B2B infrastructure at scale.
              </p>
              <p className="mt-4 text-ink-muted leading-relaxed">
                The project solves the &ldquo;useless token&rdquo; problem by tying real
                dollar revenue from residential proxy sales and compute resources to
                the deflationary{" "}
                <span className="text-neon-violet font-medium">$EXRA</span> token model.
              </p>
            </Section>

            {/* 2. Problem Statement */}
            <Section id="problem" icon={<Shield className="h-5 w-5 text-neon-violet" />} title="2. Problem Statement: The Infrastructure Deadlock">
              <div className="space-y-4">
                {[
                  {
                    title: "Datacenter IP Blocks",
                    body: "Modern anti-fraud systems (Google, Meta, TikTok) instantly identify and block cloud provider IP addresses. Businesses need live residential IPs to operate effectively.",
                    color: "neon" as const,
                  },
                  {
                    title: "Economic Inefficiency",
                    body: "Billions of devices sit idle 90% of the time, remaining a passive liability rather than an income-generating asset for their owners.",
                    color: "violet" as const,
                  },
                  {
                    title: "Painful Web3 Onboarding",
                    body: "95% of Web3 projects are inaccessible to mainstream users due to the complexity of wallet management and gas fee mechanics.",
                    color: "neon" as const,
                  },
                ].map((item) => (
                  <GlassCard key={item.title} glow={item.color} padding="md">
                    <h4 className="font-semibold text-ink mb-1">{item.title}</h4>
                    <p className="text-sm text-ink-muted leading-relaxed">{item.body}</p>
                  </GlassCard>
                ))}
              </div>
            </Section>

            {/* 3. Solution */}
            <Section id="solution" icon={<Lock className="h-5 w-5 text-neon" />} title="3. Solution & Product">
              <h3 className="text-sm font-semibold uppercase tracking-widest text-ink-dim mb-4">
                3.1. Two-Tier Network
              </h3>
              <div className="overflow-x-auto rounded-xl border border-glass-border">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-glass-border bg-glass-fill">
                      <th className="px-4 py-3 text-left font-semibold text-ink">Tier</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Requirements</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Tax</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Withdrawal</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Staking</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Use case</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr className="border-b border-glass-border/50">
                      <td className="px-4 py-3 font-medium text-ink-muted">Anon</td>
                      <td className="px-4 py-3 text-ink-muted">Machine ID / Fingerprint</td>
                      <td className="px-4 py-3 text-amber-400 font-medium">25%</td>
                      <td className="px-4 py-3 text-ink-muted">24 hours</td>
                      <td className="px-4 py-3 text-ink-muted">0 EXRA</td>
                      <td className="px-4 py-3 text-ink-muted">Scraping, mass-market pools</td>
                    </tr>
                    <tr>
                      <td className="px-4 py-3 font-medium text-neon">Peak</td>
                      <td className="px-4 py-3 text-ink-muted">peaq DID + KYC/VC</td>
                      <td className="px-4 py-3 text-emerald-400 font-semibold">0%</td>
                      <td className="px-4 py-3 text-ink-muted">Instant</td>
                      <td className="px-4 py-3 text-neon font-medium">100 EXRA</td>
                      <td className="px-4 py-3 text-ink-muted">B2B, AI, GPU compute</td>
                    </tr>
                  </tbody>
                </table>
              </div>

              <h3 className="mt-8 text-sm font-semibold uppercase tracking-widest text-ink-dim mb-4">
                3.2. Sentinel Guard: Trust Technology
              </h3>
              <div className="grid sm:grid-cols-2 gap-4">
                {[
                  {
                    title: "Frictionless Onboarding",
                    body: "Telegram Mini App (TMA) + Native Android SDK. One-click registration via a verified phone number — Sybil protection at the entry point.",
                  },
                  {
                    title: "Oracle Consensus",
                    body: "Consensus from three geographically distributed Go-oracles (2/3 signatures) to confirm work volume and authorize reward minting.",
                  },
                  {
                    title: "ZK-light Attestation",
                    body: "Lightweight cryptographic proofs on the client side: traffic integrity without significant battery drain.",
                  },
                  {
                    title: "Slashing & Burn",
                    body: "Canary Tasks system. Any detected fraud results in the Peak node&apos;s collateral (100 EXRA) being permanently burned (True Burn).",
                  },
                ].map((item) => (
                  <GlassCard key={item.title} padding="md">
                    <div className="flex items-start gap-3">
                      <CheckCircle2 className="h-4 w-4 text-neon mt-0.5 shrink-0" />
                      <div>
                        <h4 className="font-semibold text-ink text-sm mb-1">{item.title}</h4>
                        <p className="text-xs text-ink-muted leading-relaxed">{item.body}</p>
                      </div>
                    </div>
                  </GlassCard>
                ))}
              </div>
            </Section>

            {/* 4. Market */}
            <Section id="market" icon={<TrendingUp className="h-5 w-5 text-neon" />} title="4. Market Analysis & Unit Economics">
              <p className="text-ink-muted leading-relaxed mb-6">
                The residential proxy and distributed compute market is valued at{" "}
                <span className="text-neon font-semibold">$20B+</span>. EXRA
                delivers{" "}
                <span className="text-neon font-semibold">15–50% discounts</span>{" "}
                over incumbents by eliminating server operational costs entirely
                (Zero DC costs).
              </p>

              <h3 className="text-sm font-semibold uppercase tracking-widest text-ink-dim mb-4">
                Revenue projection · 10,000 active nodes
              </h3>
              <p className="text-xs text-ink-dim mb-3">
                Network ratio: 8,000 Anon (80%) / 2,000 Peak (20%)
              </p>
              <div className="overflow-x-auto rounded-xl border border-glass-border">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-glass-border bg-glass-fill">
                      <th className="px-4 py-3 text-left font-semibold text-ink">Metric</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Value / mo.</th>
                      <th className="px-4 py-3 text-left font-semibold text-ink">Notes</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-glass-border/50">
                    {[
                      ["Total Revenue (B2B)", "$50,000", "Traffic sales to enterprise clients"],
                      ["Protocol commission (20%)", "$10,000", "Base system take rate"],
                      ["Worker reward pool", "$40,000", "Amount distributed across nodes"],
                      ["Peak nodes share (50% pool)", "$20,000", "Elite nodes, zero tax (0%)"],
                      ["Anon nodes share (50% pool)", "$20,000", "Mass-market income"],
                      ["Anon tax (25%)", "$5,000", "Penalty tax → Treasury"],
                      ["Actually paid to workers", "$35,000", "$20,000 Peak + $15,000 Anon"],
                      ["Net project profit", "$15,000", "$10,000 (commission) + $5,000 (tax)"],
                    ].map(([metric, value, note]) => (
                      <tr key={metric}>
                        <td className="px-4 py-3 text-ink-muted">{metric}</td>
                        <td className="px-4 py-3 font-semibold text-neon">{value}</td>
                        <td className="px-4 py-3 text-ink-dim text-xs">{note}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Section>

            {/* 5. Tokenomics */}
            <Section id="tokenomics" icon={<Coins className="h-5 w-5 text-neon-violet" />} title="5. Tokenomics: The Digital Fuel Economy">
              <h3 className="text-sm font-semibold uppercase tracking-widest text-ink-dim mb-4">
                5.1. $EXRA Utility Value
              </h3>
              <div className="space-y-3 mb-8">
                {[
                  {
                    num: "01",
                    title: "Discounted Utility",
                    body: "B2B clients receive a 20% discount when paying for services in EXRA tokens — incentivizing organic market buyback.",
                  },
                  {
                    num: "02",
                    title: "Staking for Revenue",
                    body: "Locking 100 EXRA unlocks access to premium orders and zero-commission withdrawals.",
                  },
                  {
                    num: "03",
                    title: "Oracle-Driven Payouts",
                    body: "Rewards are fixed in USD, paid in EXRA at the current market rate — built-in protection from token volatility.",
                  },
                ].map((item) => (
                  <div key={item.num} className="flex gap-4 items-start">
                    <span className="text-xs font-mono text-neon-violet opacity-60 mt-0.5 shrink-0">
                      {item.num}
                    </span>
                    <div>
                      <span className="font-medium text-ink">{item.title} — </span>
                      <span className="text-ink-muted text-sm">{item.body}</span>
                    </div>
                  </div>
                ))}
              </div>

              <h3 className="text-sm font-semibold uppercase tracking-widest text-ink-dim mb-4">
                5.2. Deflationary Emission Model
              </h3>
              <div className="grid sm:grid-cols-3 gap-4">
                <GlassCard glow="neon" padding="md" className="text-center">
                  <p className="text-xs text-ink-dim mb-1 uppercase tracking-wider">Max Supply</p>
                  <p className="text-2xl font-bold text-neon">1,000,000,000</p>
                  <p className="text-xs text-ink-muted mt-1">$EXRA</p>
                </GlassCard>
                <GlassCard glow="violet" padding="md">
                  <p className="text-xs text-ink-dim mb-2 uppercase tracking-wider">Double Burn</p>
                  <p className="text-sm text-ink-muted">
                    <span className="text-neon-violet font-medium">Buyback & Burn:</span> 50%
                    of net profit → buyback and permanent destruction
                  </p>
                  <p className="text-sm text-ink-muted mt-2">
                    <span className="text-neon-violet font-medium">Slashing Burn:</span> 100%
                    of confiscated collateral burned forever
                  </p>
                </GlassCard>
                <GlassCard padding="md">
                  <p className="text-xs text-ink-dim mb-2 uppercase tracking-wider">Recycling Pool</p>
                  <p className="text-sm text-ink-muted">
                    Anon node taxes flow into the Recycling Pool — sustaining a
                    perpetual reward cycle without new token emission.
                  </p>
                </GlassCard>
              </div>
            </Section>

            {/* 6. Governance & Security */}
            <Section id="governance" icon={<Shield className="h-5 w-5 text-neon" />} title="6. Governance & Security">
              <p className="text-ink-muted leading-relaxed mb-4">
                After Phase 2 completes, the protocol transitions to{" "}
                <span className="text-neon font-medium">Zero Admin</span> mode.
                Network parameters (fees, staking) are governed entirely by
                on-chain votes from Peak-status holders and oracle multisig wallets
                — no admin keys, no single point of control.
              </p>
              <p className="text-ink-muted leading-relaxed mb-6">
                The protocol was subjected to an{" "}
                <span className="text-neon font-medium">independent security audit (v2.4.1)</span>
                {" "}that certified the system as production-ready. The audit identified
                7 findings across DoS resistance and race condition edge cases; every
                finding was remediated and independently verified before mainnet
                launch. Zero critical vulnerabilities remain open.
              </p>
              <GlassCard glow="neon" padding="md">
                <p className="text-center text-ink leading-relaxed italic">
                  &ldquo;EXRA is the nervous system of the new machine economy.
                  <br />
                  Every smartphone is an asset; every{" "}
                  <span className="text-neon font-semibold">$EXRA</span> is backed
                  by real traffic.&rdquo;
                </p>
              </GlassCard>
            </Section>

            {/* ── Final CTA ── */}
            <AnimatedSection>
              <AnimatedItem>
                <GlassCard elevated padding="none" className="overflow-hidden">
                  <div className="relative px-8 py-12 text-center">
                    <div
                      aria-hidden
                      className="absolute inset-0 -z-10 bg-grid opacity-40"
                    />
                    <h2 className="text-display-lg text-balance mb-4">
                      <span className="text-gradient">Ready to connect?</span>
                    </h2>
                    <p className="text-ink-muted mb-8 max-w-md mx-auto">
                      Run a node in 2 minutes or buy residential bandwidth right now.
                    </p>
                    <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
                      <Button variant="primary" size="lg" asChild>
                        <Link href="/marketplace">
                          Run a node
                          <ArrowRight className="h-4 w-4" />
                        </Link>
                      </Button>
                      <Button variant="secondary" size="lg" asChild>
                        <Link href="/marketplace">
                          Buy bandwidth
                        </Link>
                      </Button>
                      <Button variant="ghost" size="lg" asChild>
                        <a href="/whitepaper.pdf" download="EXRA-Whitepaper-v2.5.pdf">
                          <Download className="h-4 w-4" />
                          Download PDF
                        </a>
                      </Button>
                    </div>
                  </div>
                </GlassCard>
              </AnimatedItem>
            </AnimatedSection>

          </div>
        </div>
      </div>
    </div>
  );
}

function Section({
  id,
  icon,
  title,
  children,
}: {
  id: string;
  icon: React.ReactNode;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <AnimatedSection id={id}>
      <AnimatedItem>
        <div className="flex items-center gap-3 mb-5">
          <div className="p-2 rounded-lg bg-glass-fill border border-glass-border">
            {icon}
          </div>
          <h2 className="text-xl font-semibold text-ink">{title}</h2>
        </div>
        {children}
      </AnimatedItem>
    </AnimatedSection>
  );
}
