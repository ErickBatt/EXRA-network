import type { Metadata } from "next";
import Link from "next/link";
import { ArrowLeft, BookOpen, Code2, Terminal, Zap } from "lucide-react";
import { Navbar } from "@/components/navbar";
import { Footer } from "@/components/footer";
import { GlassCard } from "@/components/ui/glass-card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export const metadata: Metadata = {
  title: "Developer Docs — Coming Soon · EXRA",
  description:
    "EXRA developer documentation — REST API, Node SDK, and on-chain integration guides. Coming soon.",
};

const COMING_SOON = [
  {
    icon: Code2,
    title: "REST API Reference",
    body: "Buy and route residential bandwidth programmatically. Filter by country, tier, and latency.",
  },
  {
    icon: Terminal,
    title: "Node SDK",
    body: "Integrate the EXRA worker into any Android or desktop app with our open-source SDK.",
  },
  {
    icon: BookOpen,
    title: "On-chain Integration",
    body: "Mint rewards, verify PoP attestations, and interact with the peaq pallet from your dApp.",
  },
  {
    icon: Zap,
    title: "WebSocket Streams",
    body: "Subscribe to live node events, payout confirmations, and network health metrics.",
  },
];

export default function DocsPage() {
  return (
    <>
      <Navbar />
      <main className="min-h-screen pt-32 pb-40">
        <div className="container max-w-4xl">
          {/* Back */}
          <Link
            href="/"
            className="inline-flex items-center gap-2 text-sm text-ink-muted hover:text-neon transition-colors mb-12"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            Back to home
          </Link>

          {/* Hero */}
          <div className="text-center mb-20">
            <Badge variant="violet" className="mb-6">
              Developer Docs
            </Badge>
            <h1 className="text-display-xl text-balance mb-6">
              <span className="text-gradient">Documentation</span>
              <br />
              <span className="text-neon-gradient">coming soon.</span>
            </h1>
            <p className="text-lg text-ink-muted max-w-xl mx-auto leading-relaxed">
              We&apos;re writing comprehensive guides for the EXRA API, Node SDK, and
              on-chain integrations. Sign up for early access to get notified
              when docs go live.
            </p>
            <div className="mt-8 flex flex-col sm:flex-row items-center justify-center gap-3">
              <Button variant="primary" size="lg" asChild>
                <Link href="/#waitlist">
                  Get early access
                  <Zap className="h-4 w-4" />
                </Link>
              </Button>
              <Button variant="ghost" size="lg" asChild>
                <Link href="/whitepaper">Read the White Paper</Link>
              </Button>
            </div>
          </div>

          {/* Sections preview */}
          <div className="grid sm:grid-cols-2 gap-5">
            {COMING_SOON.map((item) => {
              const Icon = item.icon;
              return (
                <GlassCard key={item.title} padding="lg" className="opacity-60">
                  <div className="flex gap-4 items-start">
                    <div className="h-10 w-10 shrink-0 rounded-xl glass-elevated flex items-center justify-center">
                      <Icon className="h-5 w-5 text-neon-violet" />
                    </div>
                    <div>
                      <h3 className="font-semibold text-ink mb-1">{item.title}</h3>
                      <p className="text-sm text-ink-muted leading-relaxed">
                        {item.body}
                      </p>
                      <span className="mt-3 inline-block text-[10px] font-mono uppercase tracking-widest text-ink-ghost">
                        Coming soon
                      </span>
                    </div>
                  </div>
                </GlassCard>
              );
            })}
          </div>
        </div>
      </main>
      <Footer />
    </>
  );
}
