import * as React from "react";
import { cn } from "@/lib/utils";

interface BadgeProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: "default" | "neon" | "violet" | "success";
}

const variantMap = {
  default: "border-glass-borderBright text-ink-muted bg-glass-fill",
  neon: "border-neon/40 text-neon-bright bg-neon/5",
  violet: "border-neon-violet/40 text-neon-violet bg-neon-violet/5",
  success: "border-success/40 text-success bg-success/5",
};

export function Badge({ className, variant = "default", ...props }: BadgeProps) {
  return (
    <div
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium tracking-wide backdrop-blur-md",
        variantMap[variant],
        className
      )}
      {...props}
    />
  );
}
