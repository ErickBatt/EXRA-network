import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

/**
 * Button — shadcn-style API with EXRA brand variants.
 *
 * - `primary`: solid neon-gradient CTA, used for the main hero action.
 * - `secondary`: glass surface with bright border, used for "secondary" CTAs.
 * - `ghost`: transparent, hover only — used inside navbars / inline.
 * - `outline`: 1px hairline border, no fill — used in dense areas.
 *
 * Use `asChild` to render the button styles on a child element (e.g. <Link>),
 * which avoids the invalid <button><a/></button> nesting.
 */
const buttonVariants = cva(
  [
    "relative inline-flex items-center justify-center gap-2",
    "rounded-full font-medium tracking-tight whitespace-nowrap",
    "transition-all duration-200 ease-out",
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-neon focus-visible:ring-offset-2 focus-visible:ring-offset-bg",
    "disabled:opacity-50 disabled:pointer-events-none",
    "select-none",
  ].join(" "),
  {
    variants: {
      variant: {
        primary: [
          "text-bg font-semibold",
          "bg-gradient-to-r from-neon-bright via-neon to-neon-violet",
          "shadow-[0_0_24px_rgba(34,211,238,0.4),0_0_60px_rgba(167,139,250,0.2)]",
          "hover:shadow-[0_0_32px_rgba(34,211,238,0.6),0_0_80px_rgba(167,139,250,0.35)]",
          "hover:-translate-y-0.5",
        ].join(" "),
        secondary: [
          "text-ink",
          "bg-glass-fill backdrop-blur-md border border-glass-borderBright",
          "hover:bg-glass-fillHover hover:border-neon/40",
          "hover:shadow-[0_0_20px_rgba(34,211,238,0.18)]",
        ].join(" "),
        ghost: "text-ink-muted hover:text-ink hover:bg-glass-fill",
        outline:
          "text-ink border border-glass-borderBright hover:border-neon/50 hover:text-neon-bright",
      },
      size: {
        sm: "h-9 px-4 text-sm",
        md: "h-11 px-6 text-[15px]",
        lg: "h-14 px-8 text-base",
        icon: "h-10 w-10",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "md",
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp
        ref={ref}
        className={cn(buttonVariants({ variant, size, className }))}
        {...props}
      />
    );
  }
);
Button.displayName = "Button";

export { Button, buttonVariants };
