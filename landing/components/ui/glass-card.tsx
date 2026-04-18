import * as React from "react";
import { cn } from "@/lib/utils";

/**
 * GlassCard — the workhorse surface for the landing.
 *
 * Two layers:
 *  1. Outer container with `glass` (or `glass-elevated`) gives the frosted bg + 1px border.
 *  2. An absolutely-positioned ::before-style div paints a subtle gradient corner-glow.
 *
 * The `glow` prop adds a hover-reactive neon edge — used for clickable cards.
 */
interface GlassCardProps extends React.HTMLAttributes<HTMLDivElement> {
  elevated?: boolean;
  glow?: "neon" | "violet" | "none";
  interactive?: boolean;
  padding?: "sm" | "md" | "lg" | "none";
}

const paddingMap = {
  sm: "p-5",
  md: "p-7",
  lg: "p-9",
  none: "",
};

export const GlassCard = React.forwardRef<HTMLDivElement, GlassCardProps>(
  (
    {
      className,
      elevated = false,
      glow = "none",
      interactive = false,
      padding = "md",
      children,
      ...props
    },
    ref
  ) => {
    return (
      <div
        ref={ref}
        className={cn(
          "relative rounded-2xl overflow-hidden isolate",
          elevated ? "glass-elevated" : "glass",
          interactive && "glass-hover cursor-pointer",
          paddingMap[padding],
          className
        )}
        {...props}
      >
        {/* Top-left gradient highlight — gives the surface depth */}
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 -z-10 opacity-60"
          style={{
            background:
              glow === "violet"
                ? "radial-gradient(ellipse 60% 40% at 20% 0%, rgba(167,139,250,0.18), transparent 70%)"
                : glow === "neon"
                  ? "radial-gradient(ellipse 60% 40% at 20% 0%, rgba(34,211,238,0.18), transparent 70%)"
                  : "radial-gradient(ellipse 60% 40% at 20% 0%, rgba(255,255,255,0.06), transparent 70%)",
          }}
        />
        {/* Inner border highlight (top edge) — Apple-style "lit from above" */}
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 top-0 h-px"
          style={{
            background:
              "linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.18) 50%, transparent 100%)",
          }}
        />
        {children}
      </div>
    );
  }
);
GlassCard.displayName = "GlassCard";
