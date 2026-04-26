"use client";

import * as React from "react";
import Link from "next/link";
import { motion, useScroll, useTransform } from "framer-motion";
import { Menu, X, ArrowUpRight } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

const NAV_ITEMS = [
  { label: "Network", href: "#network" },
  { label: "How it works", href: "#how" },
  { label: "For Buyers", href: "#buyers" },
  { label: "Download", href: "#download" },
  { label: "Tokenomics", href: "#tokenomics" },
  { label: "Docs", href: "/docs" },
];

export function Navbar() {
  const [mobileOpen, setMobileOpen] = React.useState(false);
  const { scrollY } = useScroll();

  // Navbar gets more opaque as user scrolls — Apple-style behaviour
  const bg = useTransform(scrollY, [0, 80], ["rgba(9,9,11,0)", "rgba(9,9,11,0.7)"]);
  const blur = useTransform(scrollY, [0, 80], ["blur(0px)", "blur(16px)"]);
  const borderOpacity = useTransform(scrollY, [0, 80], [0, 0.08]);

  return (
    <>
      <motion.header
        style={{
          backgroundColor: bg,
          backdropFilter: blur,
          WebkitBackdropFilter: blur,
        }}
        className="fixed inset-x-0 top-0 z-50"
      >
        <motion.div
          aria-hidden
          style={{ opacity: borderOpacity }}
          className="absolute inset-x-0 bottom-0 h-px bg-gradient-to-r from-transparent via-white/12 to-transparent"
        />
        <nav className="container flex items-center justify-between h-16">
          <Link href="/" className="flex items-center gap-2.5 group">
            <Logomark />
            <span className="font-semibold tracking-tight text-ink text-[17px]">EXRA</span>
            <span className="hidden sm:inline-block text-[11px] font-mono uppercase tracking-[0.18em] text-ink-dim ml-1.5 mt-0.5">
              DePIN
            </span>
          </Link>

          <div className="hidden md:flex items-center gap-1">
            {NAV_ITEMS.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                target={item.external ? "_blank" : undefined}
                rel={item.external ? "noopener noreferrer" : undefined}
                className="px-4 py-2 text-sm text-ink-muted hover:text-ink transition-colors rounded-full"
              >
                {item.label}
              </Link>
            ))}
          </div>

          <div className="hidden md:flex items-center gap-2">
            <Button variant="primary" size="sm" asChild>
              <Link href="/marketplace">
                Launch app
                <ArrowUpRight className="h-3.5 w-3.5" />
              </Link>
            </Button>
          </div>

          <button
            type="button"
            className="md:hidden p-2 -mr-2 text-ink"
            onClick={() => setMobileOpen((v) => !v)}
            aria-label={mobileOpen ? "Close menu" : "Open menu"}
            aria-expanded={mobileOpen}
          >
            {mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
        </nav>
      </motion.header>

      {/* Mobile drawer */}
      <div
        className={cn(
          "md:hidden fixed inset-x-0 top-16 z-40 transition-[opacity,transform] duration-300",
          mobileOpen ? "opacity-100 translate-y-0" : "opacity-0 -translate-y-4 pointer-events-none"
        )}
      >
        <div className="container">
          <div className="glass-elevated rounded-2xl mt-2 p-4 flex flex-col gap-1">
            {NAV_ITEMS.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                target={item.external ? "_blank" : undefined}
                rel={item.external ? "noopener noreferrer" : undefined}
                onClick={() => setMobileOpen(false)}
                className="px-4 py-3 text-base text-ink hover:bg-glass-fillHover rounded-xl"
              >
                {item.label}
              </Link>
            ))}
            <div className="hairline my-2" />
            <Link
              href="/marketplace"
              onClick={() => setMobileOpen(false)}
              className="px-4 py-3 text-base text-neon-bright font-medium flex items-center gap-1.5"
            >
              Launch app <ArrowUpRight className="h-4 w-4" />
            </Link>
          </div>
        </div>
      </div>
    </>
  );
}

/** Inline SVG logo so we don't ship a binary asset. */
function Logomark() {
  return (
    <svg
      width="28"
      height="28"
      viewBox="0 0 32 32"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
      className="transition-transform duration-300 group-hover:rotate-90"
    >
      <defs>
        <linearGradient id="lm-grad" x1="0" y1="0" x2="32" y2="32" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stopColor="#67e8f9" />
          <stop offset="100%" stopColor="#a78bfa" />
        </linearGradient>
      </defs>
      <path
        d="M16 2 L29 9.5 L29 22.5 L16 30 L3 22.5 L3 9.5 Z"
        stroke="url(#lm-grad)"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      <circle cx="16" cy="16" r="3" fill="url(#lm-grad)" />
      <circle cx="16" cy="16" r="6" stroke="url(#lm-grad)" strokeOpacity="0.4" strokeWidth="1" />
    </svg>
  );
}
