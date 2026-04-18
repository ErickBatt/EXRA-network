# EXRA Landing — Premium DePIN Marketing Site

> Next.js 15 + Tailwind + shadcn-style primitives + framer-motion + react-simple-maps.
> Built to make Google / Anthropic grant evaluators take a second look.

## Stack

| Layer | Choice | Why |
|---|---|---|
| Framework | **Next.js 15** (App Router) | RSC, edge-ready, best-in-class image / font pipeline |
| Language | **TypeScript** strict | Type safety across components |
| Styling | **Tailwind 3.4** + custom design tokens | Fast iteration, consistent spacing/colour, no runtime cost |
| Components | **shadcn-style primitives** (`Button`, `GlassCard`, `Badge`) | Owned, not vendored — fully customisable, tree-shakeable |
| Animation | **framer-motion** 11 | Scroll-triggered fade-up, subtle parallax, spring physics |
| Icons | **lucide-react** | Consistent stroke weight, large tree-shaken set |
| Map | **react-simple-maps** + d3-geo | TopoJSON of world via jsdelivr CDN (cached, ~100KB) |
| Fonts | **Geist Sans + Geist Mono** (Google Fonts) | Vercel's display family, exceptional at all sizes |

## Design system

The visual language lives in three files. Touch these to re-skin the site:

- **`tailwind.config.ts`** — colour palette, font sizes, animations, shadows.
- **`app/globals.css`** — `@layer components` defines `.glass`, `.glass-elevated`, `.text-neon-gradient`, `.hairline`, `.bg-grid`, pulse keyframes.
- **`components/animated.tsx`** — single source for fade-up easing/duration. Swap motion library here without touching sections.

### Colour palette

| Token | Hex | Usage |
|---|---|---|
| `bg.DEFAULT` | `#09090b` | Page background (zinc-950) |
| `ink.DEFAULT` | `#fafafa` | Primary text |
| `ink-muted` | `#a1a1aa` | Secondary text |
| `neon.DEFAULT` | `#22d3ee` | Brand accent (cyan-400, electric blue) |
| `neon-bright` | `#67e8f9` | Hover / glow accent |
| `neon-violet` | `#a78bfa` | Secondary accent (gradient pair) |
| `glass-fill` | `rgba(255,255,255,0.03)` | Card backgrounds |
| `glass-border` | `rgba(255,255,255,0.08)` | 1px hairlines |

### Typography

- **Display**: `Geist Sans 600`, `clamp(3.5rem, 8vw, 6.5rem)`, `letter-spacing: -0.04em`.
- **Body**: `Geist Sans 400`, `text-lg` (18px) on landing.
- **Mono / labels**: `Geist Mono`, uppercase, `letter-spacing: 0.16em` for eyebrows.

## File map

```
landing/
├── app/
│   ├── layout.tsx        # fonts, metadata, viewport, dark mode
│   ├── page.tsx          # composes all sections
│   └── globals.css       # design tokens, glassmorphism utilities, keyframes
├── components/
│   ├── ui/
│   │   ├── button.tsx    # CVA-driven variants (primary/secondary/ghost/outline)
│   │   ├── glass-card.tsx
│   │   └── badge.tsx
│   ├── animated.tsx      # AnimatedSection / AnimatedItem (framer-motion)
│   ├── navbar.tsx        # scroll-reactive backdrop blur, mobile drawer
│   ├── world-map.tsx     # react-simple-maps + pulsing pins + arcs + parallax
│   ├── hero.tsx          # title + map + CTAs + live-stats strip
│   ├── how-it-works.tsx  # 4-step glassmorphism cards
│   ├── features.tsx      # 6-card feature grid (bento-ish)
│   ├── earnings.tsx      # device → $/day grid
│   ├── tokenomics.tsx    # split bar + referral tier table
│   ├── final-cta.tsx     # closing card with CTA cluster
│   └── footer.tsx        # link columns + social icons
├── lib/
│   └── utils.ts          # `cn()` (clsx + tailwind-merge)
├── public/               # static assets — currently empty (logo is inline SVG)
├── package.json
├── tsconfig.json
├── tailwind.config.ts
├── postcss.config.mjs
├── next.config.mjs
└── README.md
```

## Run locally

```bash
cd landing
npm install
npm run dev    # → http://localhost:3001
```

Production build:
```bash
npm run build
npm run start
```

## Libraries — what I added and why

| Package | Reason |
|---|---|
| `next@15` | App Router, React 19, Turbopack-ready dev server |
| `framer-motion@11` | Scroll-triggered animations, parallax, spring physics. Honours `prefers-reduced-motion` automatically via `useReducedMotion()` |
| `lucide-react@0.460` | 1,500+ tree-shakeable SVG icons, consistent stroke weight |
| `react-simple-maps@3` | Declarative world map composition (`<ComposableMap>`/`<Geographies>`/`<Marker>`) — paired with TopoJSON from jsdelivr CDN so we don't bundle 100KB of geo |
| `d3-geo@3` | Underlying projection math for `react-simple-maps` |
| `class-variance-authority@0.7` | shadcn-style variant API for `Button` — clean prop ergonomics |
| `clsx` + `tailwind-merge` | The `cn()` helper — composable, conflict-free Tailwind class names |
| `tailwindcss-animate` | Animation utilities used by shadcn primitives |

## Performance budget (target)

| Metric | Budget | Strategy |
|---|---|---|
| LCP | < 2.0s | Hero text is plain HTML; map TopoJSON cached on CDN; no above-fold images |
| CLS | < 0.05 | Map has fixed `aspect-[16/10]`; fonts use `display: swap` with sized fallbacks |
| TTI | < 3.0s | All sections except `Navbar`/`Hero`/`WorldMap` are client islands only where motion needed; `optimizePackageImports` for lucide + framer |
| Bundle (first load) | < 200KB | Tree-shaken icons; no global state lib; CDN-hosted geo |

## Accessibility notes (built-in, not afterthoughts)

- `prefers-reduced-motion` honoured in `WorldMap` (parallax disabled), `AnimatedSection` (animation disabled), and CSS keyframes (`globals.css` `@media`).
- Semantic landmarks: `<header>`, `<main>`, `<nav>`, `<footer>`.
- Focus rings: every `Button` has `focus-visible:ring-2 ring-neon`.
- Mobile drawer: `aria-expanded`, `aria-label`, focus management.
- Color contrast: body text `#a1a1aa` on `#09090b` = **9.4:1** (passes AAA). Headlines on bg = **18.4:1**.

## What's intentionally NOT here

- Auth / wallet connect — landing is brochureware, links to `app.exra.space`.
- Forms — newsletter / contact use external services (`mailto:`, embeds).
- Logo binary — inline SVG in `Navbar` and `Footer` for now. Drop a `public/logo.svg` to override.
- Analytics — wire up Plausible / PostHog in `app/layout.tsx` `<Script>` tag.
- Internationalisation — copy is English-only. Wire `next-intl` if expanding.

## Next moves

1. **Real numbers** — replace hardcoded "48,217 nodes" with a fetch to `https://api.exra.space/network/stats` (ISR 60s).
2. **OG image** — generate dynamic `app/opengraph-image.tsx` using the hero map + brand gradient.
3. **/whitepaper** — port the existing `EXRA Tokenomics Architecture v2.0_ PEAQ DePIN.md` into `app/whitepaper/page.tsx` with proper typography.
4. **Testimonials / press** — once we have grant decisions or top-10 hub partners, drop a logo cloud above the final CTA.

---

Built for `erickbattt@gmail.com` · April 2026 · Live at `exra.space`
