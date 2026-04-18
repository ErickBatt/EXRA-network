import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: "class",
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
    "./lib/**/*.{ts,tsx}",
  ],
  theme: {
    container: {
      center: true,
      padding: "1.5rem",
      screens: {
        "2xl": "1400px",
      },
    },
    extend: {
      colors: {
        // Cyberpunk minimalism palette
        bg: {
          DEFAULT: "#09090b", // near-black, zinc-950
          elevated: "#0d0d10",
          subtle: "#111114",
        },
        ink: {
          DEFAULT: "#fafafa",
          muted: "#a1a1aa", // zinc-400
          dim: "#71717a", // zinc-500
          ghost: "#3f3f46", // zinc-700
        },
        // Brand neon — electric blue primary, violet secondary
        neon: {
          DEFAULT: "#22d3ee", // cyan-400, electric blue
          bright: "#67e8f9", // cyan-300
          deep: "#0891b2", // cyan-600
          violet: "#a78bfa", // violet-400
          "violet-deep": "#7c3aed", // violet-600
        },
        glass: {
          border: "rgba(255, 255, 255, 0.08)",
          borderBright: "rgba(255, 255, 255, 0.14)",
          fill: "rgba(255, 255, 255, 0.03)",
          fillHover: "rgba(255, 255, 255, 0.05)",
        },
        success: "#10b981",
        warning: "#f59e0b",
        danger: "#ef4444",
      },
      fontFamily: {
        sans: ["var(--font-geist-sans)", "Inter", "system-ui", "sans-serif"],
        mono: ["var(--font-geist-mono)", "ui-monospace", "monospace"],
        display: ["var(--font-geist-sans)", "Inter", "system-ui", "sans-serif"],
      },
      fontSize: {
        // Display sizes for hero
        "display-2xl": ["clamp(3.5rem, 8vw, 6.5rem)", { lineHeight: "0.95", letterSpacing: "-0.04em", fontWeight: "600" }],
        "display-xl": ["clamp(2.75rem, 6vw, 4.5rem)", { lineHeight: "1", letterSpacing: "-0.035em", fontWeight: "600" }],
        "display-lg": ["clamp(2rem, 4.5vw, 3.25rem)", { lineHeight: "1.05", letterSpacing: "-0.03em", fontWeight: "600" }],
      },
      backgroundImage: {
        "grid-pattern":
          "linear-gradient(to right, rgba(255,255,255,0.04) 1px, transparent 1px), linear-gradient(to bottom, rgba(255,255,255,0.04) 1px, transparent 1px)",
        "radial-glow":
          "radial-gradient(ellipse at center, rgba(34,211,238,0.18) 0%, rgba(167,139,250,0.08) 35%, transparent 70%)",
        "neon-gradient":
          "linear-gradient(135deg, #22d3ee 0%, #a78bfa 100%)",
      },
      backgroundSize: {
        grid: "60px 60px",
      },
      boxShadow: {
        "neon-sm": "0 0 12px rgba(34, 211, 238, 0.35)",
        neon: "0 0 28px rgba(34, 211, 238, 0.45), 0 0 60px rgba(34, 211, 238, 0.15)",
        "neon-violet": "0 0 28px rgba(167, 139, 250, 0.5), 0 0 60px rgba(167, 139, 250, 0.2)",
        "glass-inset":
          "inset 0 1px 0 0 rgba(255,255,255,0.08), 0 1px 2px 0 rgba(0,0,0,0.4)",
        "glass-elevated":
          "inset 0 1px 0 0 rgba(255,255,255,0.1), 0 8px 32px 0 rgba(0,0,0,0.6)",
      },
      animation: {
        "pulse-ring": "pulse-ring 2.4s cubic-bezier(0.215, 0.61, 0.355, 1) infinite",
        "pulse-dot": "pulse-dot 2.4s cubic-bezier(0.455, 0.03, 0.515, 0.955) infinite",
        "float-slow": "float 8s ease-in-out infinite",
        shimmer: "shimmer 2.5s linear infinite",
        "fade-up": "fade-up 0.7s cubic-bezier(0.22, 1, 0.36, 1) both",
        "gradient-x": "gradient-x 6s ease infinite",
      },
      keyframes: {
        "pulse-ring": {
          "0%": { transform: "scale(0.5)", opacity: "0.8" },
          "80%, 100%": { transform: "scale(2.6)", opacity: "0" },
        },
        "pulse-dot": {
          "0%, 100%": { transform: "scale(1)", opacity: "1" },
          "50%": { transform: "scale(1.15)", opacity: "0.85" },
        },
        float: {
          "0%, 100%": { transform: "translateY(0px)" },
          "50%": { transform: "translateY(-12px)" },
        },
        shimmer: {
          "0%": { backgroundPosition: "-200% 0" },
          "100%": { backgroundPosition: "200% 0" },
        },
        "fade-up": {
          "0%": { opacity: "0", transform: "translateY(24px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
        "gradient-x": {
          "0%, 100%": { backgroundPosition: "0% 50%" },
          "50%": { backgroundPosition: "100% 50%" },
        },
      },
    },
  },
  plugins: [require("tailwindcss-animate")],
};

export default config;
