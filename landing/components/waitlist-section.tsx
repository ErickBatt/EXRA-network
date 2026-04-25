"use client";

import * as React from "react";
import { motion, AnimatePresence, useReducedMotion } from "framer-motion";
import {
  Bug, TrendingUp, Wifi, Ghost,
  ArrowRight, Check, Loader2, AlertCircle,
  Terminal, Zap,
} from "lucide-react";
import { GlassCard } from "@/components/ui/glass-card";
import { Button } from "@/components/ui/button";
import { AnimatedSection, AnimatedItem } from "@/components/animated";
import { cn } from "@/lib/utils";
import { submitWaitlist } from "@/app/actions/waitlist";

// ── Role registry ──────────────────────────────────────────────────────────
// To add a new role: append here + add its fields in the JSX below.

const BASE_ROLES = [
  {
    id: "tester" as const,
    label: "Tester",
    clearance: "BETA-1",
    icon: Bug,
    description: "Get early beta access and help shape the product with real feedback.",
    accent: "rgba(34,211,238,0.10)",
    border: "rgba(34,211,238,0.32)",
    glow: "rgba(34,211,238,0.18)",
    iconColor: "#22d3ee",
  },
  {
    id: "investor" as const,
    label: "Investor",
    clearance: "CAP-2",
    icon: TrendingUp,
    description: "Join the cap table and grow with the decentralised bandwidth economy.",
    accent: "rgba(167,139,250,0.10)",
    border: "rgba(167,139,250,0.32)",
    glow: "rgba(167,139,250,0.18)",
    iconColor: "#a78bfa",
  },
  {
    id: "buyer" as const,
    label: "Buyer",
    clearance: "RES-IP",
    icon: Wifi,
    description: "Purchase bandwidth backed by real residential IPs across 63 countries.",
    accent: "rgba(52,211,153,0.10)",
    border: "rgba(52,211,153,0.32)",
    glow: "rgba(52,211,153,0.18)",
    iconColor: "#34d399",
  },
] as const;

const GHOST_ROLE = {
  id: "ghost" as const,
  label: "Ghost Node",
  clearance: "VOID-0",
  icon: Ghost,
  description: "An anomaly in the network. We see you.",
  accent: "rgba(239,68,68,0.08)",
  border: "rgba(239,68,68,0.28)",
  glow: "rgba(239,68,68,0.14)",
  iconColor: "#f87171",
} as const;

type Role = (typeof BASE_ROLES)[number] | typeof GHOST_ROLE;
type RoleId = Role["id"];

// ── Konami code ─────────────────────────────────────────────────────────────
const KONAMI = [
  "ArrowUp","ArrowUp","ArrowDown","ArrowDown",
  "ArrowLeft","ArrowRight","ArrowLeft","ArrowRight",
  "b","a",
];

// ── Form state ──────────────────────────────────────────────────────────────
interface FormData {
  role: RoleId | null;
  name: string;
  email: string;
  telegram: string;
  deviceType: string;
  country: string;
  useCase: string;
}

const INITIAL: FormData = {
  role: null, name: "", email: "", telegram: "",
  deviceType: "", country: "", useCase: "",
};

type Status = "idle" | "loading" | "success" | "error";

// ── WaitlistSection ──────────────────────────────────────────────────────────

export function WaitlistSection() {
  const reduced = useReducedMotion();
  const [ghostUnlocked, setGhostUnlocked] = React.useState(false);
  const [konamiIdx, setKonamiIdx] = React.useState(0);
  const [form, setForm] = React.useState<FormData>(INITIAL);
  const [status, setStatus] = React.useState<Status>("idle");
  const [errorMsg, setErrorMsg] = React.useState("");

  // Konami listener — unlocks Ghost Node role
  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === KONAMI[konamiIdx]) {
        const next = konamiIdx + 1;
        if (next === KONAMI.length) {
          setGhostUnlocked(true);
          setKonamiIdx(0);
        } else {
          setKonamiIdx(next);
        }
      } else {
        setKonamiIdx(e.key === KONAMI[0] ? 1 : 0);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [konamiIdx]);

  const roles: Role[] = ghostUnlocked
    ? [...BASE_ROLES, GHOST_ROLE]
    : [...BASE_ROLES];

  function patch(key: keyof FormData, value: string) {
    setForm(prev => ({ ...prev, [key]: value }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.role || !form.name.trim() || !form.email.trim()) return;
    setStatus("loading");
    setErrorMsg("");
    try {
      const res = await submitWaitlist({
        role: form.role,
        name: form.name,
        email: form.email,
        telegram: form.telegram,
        deviceType: form.deviceType,
        country: form.country,
        useCase: form.useCase,
      });
      setStatus(res.ok ? "success" : "error");
      if (!res.ok) setErrorMsg(res.error ?? "Unknown error.");
    } catch {
      setStatus("error");
      setErrorMsg("Network error. Please try again.");
    }
  }

  const selectedRole = roles.find(r => r.id === form.role) ?? null;
  const canSubmit = !!form.role && !!form.name.trim() && !!form.email.trim() && status !== "loading";

  if (status === "success") {
    return (
      <section id="waitlist" className="relative py-24 sm:py-32">
        <SuccessScreen role={form.role!} name={form.name} />
      </section>
    );
  }

  return (
    <section id="waitlist" className="relative py-24 sm:py-32">
      {/* Ambient glow */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-x-0 top-1/2 -translate-y-1/2 -z-10 h-[640px] max-w-4xl mx-auto"
        style={{
          background:
            "radial-gradient(ellipse at 50% 50%, rgba(34,211,238,0.11) 0%, rgba(167,139,250,0.07) 40%, transparent 70%)",
          filter: "blur(90px)",
        }}
      />

      <AnimatedSection className="container max-w-3xl">
        {/* Header */}
        <AnimatedItem>
          <div className="text-center mb-12">
            <span className="eyebrow mb-5">
              <Zap className="h-3 w-3 text-neon" />
              Early Access
            </span>
            <h2 className="text-display-lg mt-5">
              <span className="text-gradient">Identify yourself</span>
              <br />
              <span className="text-neon-gradient">to the network.</span>
            </h2>
            <p className="mt-5 text-ink-muted max-w-md mx-auto">
              Select your access level. We route you to the right onboarding
              track automatically.
            </p>
          </div>
        </AnimatedItem>

        <AnimatedItem>
          <form onSubmit={handleSubmit} noValidate>
            {/* ── Role cards ── */}
            <div
              className={cn(
                "grid gap-3 mb-8",
                roles.length <= 3 ? "sm:grid-cols-3" : "grid-cols-2 sm:grid-cols-4"
              )}
            >
              {roles.map((role, i) => {
                const Icon = role.icon;
                const selected = form.role === role.id;
                return (
                  <motion.button
                    key={role.id}
                    type="button"
                    onClick={() => patch("role", role.id)}
                    initial={reduced ? false : { opacity: 0, y: 18 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{
                      delay: i * 0.07,
                      duration: 0.45,
                      ease: [0.22, 1, 0.36, 1],
                    }}
                    className="relative text-left rounded-2xl p-5 border transition-all duration-300 overflow-hidden group focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neon/50"
                    style={{
                      background: selected
                        ? role.accent
                        : "rgba(255,255,255,0.025)",
                      borderColor: selected
                        ? role.border
                        : "rgba(255,255,255,0.08)",
                      boxShadow: selected
                        ? `0 0 28px ${role.glow}, inset 0 1px 0 rgba(255,255,255,0.08)`
                        : "none",
                    }}
                  >
                    {/* Hover scanline */}
                    <div
                      aria-hidden
                      className="pointer-events-none absolute inset-x-0 top-0 h-px opacity-0 group-hover:opacity-100 transition-opacity duration-300"
                      style={{
                        background: `linear-gradient(90deg, transparent 0%, ${role.border} 50%, transparent 100%)`,
                      }}
                    />

                    <div className="flex items-start justify-between mb-3">
                      <Icon
                        className="h-5 w-5 transition-colors duration-200"
                        style={{ color: selected ? role.iconColor : "#71717a" }}
                      />
                      <AnimatePresence>
                        {selected && (
                          <motion.div
                            key="check"
                            initial={{ scale: 0, opacity: 0 }}
                            animate={{ scale: 1, opacity: 1 }}
                            exit={{ scale: 0, opacity: 0 }}
                            transition={{ duration: 0.2, ease: "backOut" }}
                            className="h-4 w-4 rounded-full flex items-center justify-center"
                            style={{ background: role.border }}
                          >
                            <Check className="h-2.5 w-2.5 text-bg" />
                          </motion.div>
                        )}
                      </AnimatePresence>
                    </div>

                    <div className="font-semibold text-sm leading-tight text-ink">
                      {role.label}
                    </div>
                    <div className="font-mono text-[10px] text-ink-ghost mt-0.5 tracking-[0.15em] uppercase">
                      {role.clearance}
                    </div>
                  </motion.button>
                );
              })}
            </div>

            {/* ── Role-specific fields ── */}
            <AnimatePresence mode="wait">
              {form.role && selectedRole && (
                <motion.div
                  key={form.role}
                  initial={
                    reduced
                      ? false
                      : { opacity: 0, y: 14, filter: "blur(6px)" }
                  }
                  animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
                  exit={{ opacity: 0, y: -8, filter: "blur(4px)" }}
                  transition={{ duration: 0.35, ease: [0.22, 1, 0.36, 1] }}
                >
                  <GlassCard elevated padding="none" className="overflow-visible">
                    {/* Colored accent line at top */}
                    <div
                      className="h-px rounded-t-2xl"
                      style={{
                        background: `linear-gradient(90deg, transparent 0%, ${selectedRole.border} 50%, transparent 100%)`,
                      }}
                    />

                    <div className="p-7 space-y-5">
                      {/* Description */}
                      <p className="text-sm text-ink-muted pb-4 border-b border-glass-border">
                        {selectedRole.description}
                      </p>

                      {/* Common: name + email */}
                      <div className="grid sm:grid-cols-2 gap-4">
                        <FormField label="Name" required>
                          <input
                            type="text"
                            value={form.name}
                            onChange={e => patch("name", e.target.value)}
                            placeholder="Satoshi Nakamoto"
                            required
                            autoComplete="name"
                            className="form-input"
                          />
                        </FormField>
                        <FormField label="Email" required>
                          <input
                            type="email"
                            value={form.email}
                            onChange={e => patch("email", e.target.value)}
                            placeholder="you@domain.com"
                            required
                            autoComplete="email"
                            className="form-input"
                          />
                        </FormField>
                      </div>

                      {/* Common: telegram (optional) */}
                      <FormField label="Telegram" hint="optional">
                        <input
                          type="text"
                          value={form.telegram}
                          onChange={e => patch("telegram", e.target.value)}
                          placeholder="@handle"
                          className="form-input"
                        />
                      </FormField>

                      {/* ── Role extras ── */}
                      {form.role === "tester" && (
                        <FormField label="Device type">
                          <FormSelect
                            value={form.deviceType}
                            onChange={v => patch("deviceType", v)}
                            options={[
                              "Android",
                              "iOS",
                              "Desktop — Linux",
                              "Desktop — macOS",
                              "Desktop — Windows",
                            ]}
                            placeholder="Select platform..."
                          />
                        </FormField>
                      )}

                      {form.role === "investor" && (
                        <FormField label="Investment stage">
                          <FormSelect
                            value={form.useCase}
                            onChange={v => patch("useCase", v)}
                            options={[
                              "Pre-seed",
                              "Seed",
                              "Series A",
                              "Strategic / Corporate",
                            ]}
                            placeholder="Select stage..."
                          />
                        </FormField>
                      )}

                      {form.role === "buyer" && (
                        <div className="grid sm:grid-cols-2 gap-4">
                          <FormField label="Country">
                            <input
                              type="text"
                              value={form.country}
                              onChange={e => patch("country", e.target.value)}
                              placeholder="United States"
                              autoComplete="country-name"
                              className="form-input"
                            />
                          </FormField>
                          <FormField label="Use case">
                            <FormSelect
                              value={form.useCase}
                              onChange={v => patch("useCase", v)}
                              options={[
                                "Research",
                                "Business",
                                "Personal",
                                "Enterprise",
                              ]}
                              placeholder="Select..."
                            />
                          </FormField>
                        </div>
                      )}

                      {form.role === "ghost" && (
                        <div className="font-mono text-xs tracking-wide border rounded-xl p-3 leading-relaxed"
                          style={{
                            color: "rgba(248,113,113,0.6)",
                            borderColor: "rgba(239,68,68,0.2)",
                            background: "rgba(239,68,68,0.04)",
                          }}
                        >
                          <span style={{ opacity: 0.5 }}>▓▒░</span>
                          {" "}ANOMALY_DETECTED — routing through dark nodes...{" "}
                          <span style={{ opacity: 0.5 }}>░▒▓</span>
                        </div>
                      )}

                      {/* Error */}
                      <AnimatePresence>
                        {status === "error" && (
                          <motion.div
                            initial={{ opacity: 0, y: 4 }}
                            animate={{ opacity: 1, y: 0 }}
                            exit={{ opacity: 0 }}
                            className="flex items-center gap-2 text-sm text-danger"
                          >
                            <AlertCircle className="h-4 w-4 shrink-0" />
                            {errorMsg}
                          </motion.div>
                        )}
                      </AnimatePresence>

                      <Button
                        type="submit"
                        variant="primary"
                        size="lg"
                        disabled={!canSubmit}
                        className="w-full"
                      >
                        {status === "loading" ? (
                          <>
                            <Loader2 className="h-4 w-4 animate-spin" />
                            Authenticating...
                          </>
                        ) : (
                          <>
                            Request access
                            <ArrowRight className="h-4 w-4" />
                          </>
                        )}
                      </Button>
                    </div>
                  </GlassCard>
                </motion.div>
              )}
            </AnimatePresence>
          </form>
        </AnimatedItem>
      </AnimatedSection>
    </section>
  );
}

// ── Sub-components ───────────────────────────────────────────────────────────

function FormField({
  label,
  children,
  required,
  hint,
}: {
  label: string;
  children: React.ReactNode;
  required?: boolean;
  hint?: string;
}) {
  return (
    <div className="space-y-1.5">
      <label className="flex items-center gap-1.5 font-mono text-[11px] tracking-[0.15em] uppercase text-ink-ghost">
        {label}
        {required && (
          <span className="text-neon normal-case tracking-normal font-sans text-xs">
            *
          </span>
        )}
        {hint && (
          <span className="normal-case tracking-normal opacity-50 text-[10px]">
            ({hint})
          </span>
        )}
      </label>
      {children}
    </div>
  );
}

function FormSelect({
  value,
  onChange,
  options,
  placeholder,
}: {
  value: string;
  onChange: (v: string) => void;
  options: string[];
  placeholder?: string;
}) {
  return (
    <select
      value={value}
      onChange={e => onChange(e.target.value)}
      className="form-input"
    >
      <option value="">{placeholder ?? "Select..."}</option>
      {options.map(o => (
        <option key={o} value={o}>
          {o}
        </option>
      ))}
    </select>
  );
}

// ── Success screen ───────────────────────────────────────────────────────────

const SUCCESS_LINES: Record<RoleId, string> = {
  tester:   "BETA_SLOT.RESERVED — we'll ping you when your lane opens.",
  investor: "DECK_ROUTE.QUEUED — our team will reach out within 48h.",
  buyer:    "BANDWIDTH_POOL.ALLOCATED — onboarding instructions incoming.",
  ghost:    "GHOST_NODE.ACKNOWLEDGED — the network sees you.",
};

function SuccessScreen({ role, name }: { role: RoleId; name: string }) {
  return (
    <AnimatedSection className="container max-w-lg text-center">
      <AnimatedItem>
        <GlassCard elevated glow="neon" className="overflow-hidden">
          <div className="h-px hairline" />
          <div className="px-8 py-14">
            <div
              className="h-14 w-14 rounded-2xl flex items-center justify-center mx-auto mb-6"
              style={{
                background: "rgba(34,211,238,0.08)",
                border: "1px solid rgba(34,211,238,0.28)",
                boxShadow: "0 0 28px rgba(34,211,238,0.14)",
              }}
            >
              <Terminal className="h-7 w-7 text-neon" />
            </div>

            <p className="font-mono text-[11px] tracking-[0.22em] uppercase text-neon mb-3">
              access_request . accepted
            </p>

            <h3 className="text-2xl font-semibold text-gradient mt-1">
              Welcome, {name}.
            </h3>

            <p className="mt-4 font-mono text-xs text-ink-muted tracking-wide leading-relaxed">
              {SUCCESS_LINES[role]}
            </p>

            <div className="hairline my-8" />

            <p className="text-xs text-ink-ghost mb-3">
              Know someone who should be on the network?
            </p>
            <Button variant="secondary" size="sm" asChild>
              <a
                href="https://t.me/exranetworkbot"
                target="_blank"
                rel="noopener noreferrer"
              >
                Share via Telegram
                <ArrowRight className="h-3 w-3" />
              </a>
            </Button>
          </div>
        </GlassCard>
      </AnimatedItem>
    </AnimatedSection>
  );
}
