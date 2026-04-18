import * as React from "react";
import Link from "next/link";
import { Github, Twitter, MessageCircle, Send } from "lucide-react";

const COLUMNS = [
  {
    title: "Network",
    links: [
      { label: "Run a node", href: "https://app.exra.space/start" },
      { label: "Buy bandwidth", href: "https://buy.exra.space" },
      { label: "Status", href: "https://status.exra.space" },
      { label: "Explorer", href: "https://peaq.subscan.io" },
    ],
  },
  {
    title: "Build",
    links: [
      { label: "Docs", href: "https://docs.exra.space" },
      { label: "API", href: "https://docs.exra.space/api" },
      { label: "Whitepaper", href: "https://exra.space/whitepaper.pdf" },
      { label: "GitHub", href: "https://github.com/exra" },
    ],
  },
  {
    title: "Community",
    links: [
      { label: "Telegram", href: "https://t.me/exra" },
      { label: "Discord", href: "https://discord.gg/exra" },
      { label: "X / Twitter", href: "https://x.com/exranetwork" },
      { label: "Blog", href: "https://exra.space/blog" },
    ],
  },
  {
    title: "Company",
    links: [
      { label: "About", href: "/about" },
      { label: "Careers", href: "/careers" },
      { label: "Brand kit", href: "/brand" },
      { label: "Contact", href: "mailto:hi@exra.space" },
    ],
  },
];

export function Footer() {
  return (
    <footer className="relative pt-20 pb-10 border-t border-glass-border">
      <div className="container">
        <div className="grid grid-cols-2 md:grid-cols-6 gap-12 gap-y-12">
          {/* Brand block */}
          <div className="col-span-2">
            <Link href="/" className="inline-flex items-center gap-2.5">
              <Logomark />
              <span className="font-semibold text-ink text-lg tracking-tight">EXRA</span>
            </Link>
            <p className="mt-5 text-sm text-ink-muted leading-relaxed max-w-xs">
              The decentralized network that runs on every device.
              Built on peaq L1.
            </p>
            <div className="mt-6 flex items-center gap-2">
              <SocialIcon icon={Github} href="https://github.com/exra" label="GitHub" />
              <SocialIcon icon={Twitter} href="https://x.com/exranetwork" label="X" />
              <SocialIcon icon={Send} href="https://t.me/exra" label="Telegram" />
              <SocialIcon icon={MessageCircle} href="https://discord.gg/exra" label="Discord" />
            </div>
          </div>

          {COLUMNS.map((col) => (
            <div key={col.title}>
              <h4 className="text-xs font-mono uppercase tracking-[0.18em] text-ink-dim mb-5">
                {col.title}
              </h4>
              <ul className="flex flex-col gap-3">
                {col.links.map((link) => (
                  <li key={link.label}>
                    <Link
                      href={link.href}
                      className="text-sm text-ink-muted hover:text-ink transition-colors"
                    >
                      {link.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="mt-20 pt-8 border-t border-glass-border flex flex-col sm:flex-row items-center justify-between gap-4">
          <p className="text-xs text-ink-dim font-mono">
            © {new Date().getFullYear()} EXRA Network · Built on peaq · Open source
          </p>
          <div className="flex items-center gap-6 text-xs text-ink-dim">
            <Link href="/privacy" className="hover:text-ink-muted transition-colors">
              Privacy
            </Link>
            <Link href="/terms" className="hover:text-ink-muted transition-colors">
              Terms
            </Link>
            <Link href="/security" className="hover:text-ink-muted transition-colors">
              Security
            </Link>
          </div>
        </div>
      </div>
    </footer>
  );
}

function SocialIcon({
  icon: Icon,
  href,
  label,
}: {
  icon: React.ComponentType<{ className?: string }>;
  href: string;
  label: string;
}) {
  return (
    <Link
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={label}
      className="h-9 w-9 rounded-lg glass flex items-center justify-center text-ink-muted hover:text-neon-bright hover:border-neon/40 transition-colors"
    >
      <Icon className="h-4 w-4" />
    </Link>
  );
}

function Logomark() {
  return (
    <svg width="24" height="24" viewBox="0 0 32 32" fill="none" aria-hidden>
      <defs>
        <linearGradient id="ftr-grad" x1="0" y1="0" x2="32" y2="32" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#67e8f9" />
          <stop offset="100%" stopColor="#a78bfa" />
        </linearGradient>
      </defs>
      <path d="M16 2 L29 9.5 L29 22.5 L16 30 L3 22.5 L3 9.5 Z" stroke="url(#ftr-grad)" strokeWidth="1.5" strokeLinejoin="round" />
      <circle cx="16" cy="16" r="3" fill="url(#ftr-grad)" />
    </svg>
  );
}
