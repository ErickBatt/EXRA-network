"use client";

import * as React from "react";
import { motion, useReducedMotion, type Variants } from "framer-motion";
import { cn } from "@/lib/utils";

/**
 * Reusable scroll-triggered fade-in-up wrapper.
 *
 * - Honours `prefers-reduced-motion` automatically (framer-motion hook).
 * - Stagger children via `<AnimatedSection>` → `<AnimatedItem>` composition.
 *
 * Why a thin wrapper instead of inline motion.div everywhere:
 *   1. Single source of truth for easing + duration (brand consistency)
 *   2. Lets us swap to GSAP later without touching every section
 *   3. Reduces noise in section components — they describe layout, not motion
 */

const easeOut = [0.22, 1, 0.36, 1] as const;

export const fadeUpVariants: Variants = {
  hidden: { opacity: 0, y: 28 },
  visible: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.7, ease: easeOut },
  },
};

export const fadeUpStaggerVariants: Variants = {
  hidden: {},
  visible: {
    transition: {
      staggerChildren: 0.08,
      delayChildren: 0.05,
    },
  },
};

interface AnimatedSectionProps extends React.HTMLAttributes<HTMLDivElement> {
  /** When true, animation triggers once and stays. Default true. */
  once?: boolean;
  /** Viewport amount that must be visible to trigger. Default 0.2. */
  amount?: number;
  /** Disable animation entirely (useful inside hero where no scroll yet). */
  disabled?: boolean;
  as?: React.ElementType;
}

export function AnimatedSection({
  children,
  className,
  once = true,
  amount = 0.2,
  disabled = false,
  ...props
}: AnimatedSectionProps) {
  const reduced = useReducedMotion();

  if (disabled || reduced) {
    return (
      <div className={className} {...(props as React.HTMLAttributes<HTMLDivElement>)}>
        {children}
      </div>
    );
  }

  return (
    <motion.div
      className={cn(className)}
      variants={fadeUpStaggerVariants}
      initial="hidden"
      whileInView="visible"
      viewport={{ once, amount }}
      {...(props as React.ComponentProps<typeof motion.div>)}
    >
      {children}
    </motion.div>
  );
}

interface AnimatedItemProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Override the default fade-up — useful for left/right slides. */
  variants?: Variants;
  delay?: number;
}

export function AnimatedItem({
  children,
  className,
  variants = fadeUpVariants,
  delay,
  ...props
}: AnimatedItemProps) {
  const reduced = useReducedMotion();

  if (reduced) {
    return (
      <div className={className} {...(props as React.HTMLAttributes<HTMLDivElement>)}>
        {children}
      </div>
    );
  }

  // If a delay is requested, wrap variants with the custom transition
  const finalVariants: Variants = delay
    ? {
        ...variants,
        visible:
          typeof variants.visible === "object" && variants.visible !== null
            ? {
                ...variants.visible,
                transition: {
                  ...((variants.visible as { transition?: object }).transition ?? {}),
                  delay,
                },
              }
            : variants.visible,
      }
    : variants;

  return (
    <motion.div
      className={cn(className)}
      variants={finalVariants}
      {...(props as React.ComponentProps<typeof motion.div>)}
    >
      {children}
    </motion.div>
  );
}
