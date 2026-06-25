# Skill: frontend

> Frontend-first guidance for shipping web apps.
> Bundle size, a11y, perf budgets — frontend has different rules than backend.

## Decision tree

```
Project starts (or pivots to web)
        │
        ▼
[Step 1] Framework decision
        │
        ├── React + Next.js (or Vite + React Router)
        ├── Vue + Nuxt
        ├── Svelte + SvelteKit
        ├── Solid + SolidStart
        └── Vanilla (just HTML + CSS + minimal JS)
        │
        ▼
[Step 2] Rendering strategy
        │
        ├── SPA (single page app; auth-gated dashboards)
        ├── SSR (server-side render; SEO matters)
        ├── SSG (static site; blog, docs, marketing)
        └── ISR (incremental static regeneration; hybrid)
        │
        ▼
[Step 3] Performance budget
        │
        ├── LCP < 2.5s
        ├── INP < 200ms
        ├── CLS < 0.1
        ├── Bundle < 200KB (compressed)
        └── Lighthouse score > 90
        │
        ▼
[Step 4] Architecture: docs/frontend/architecture.md
        │
        ▼
[Step 5] Performance budget: docs/frontend/performance-budget.md
```

## Workflow

### Step 1: Framework decision

The framework choice cascades through everything:

| Framework | Bundle | Maturity | Best for |
|-----------|--------|----------|----------|
| React + Next.js | ~80KB | High | General web; team knows React |
| Vue + Nuxt | ~50KB | High | Lighter than React; Asia + EU teams |
| Svelte + SvelteKit | ~10KB | High | Performance-critical; small teams |
| Solid + SolidStart | ~10KB | Growing | React-like syntax, fine-grained reactivity |
| Vanilla | 0KB | High | Docs / blogs / static pages |

**Recommendation**: pick the framework your team knows best. The
ergonomics of "I already know it" outweighs micro-benchmark
differences. Migration cost is high.

### Step 2: Rendering strategy

| Strategy | When to use |
|----------|-------------|
| SPA | Auth-gated dashboards; SEO doesn't matter |
| SSR | Content sites; SEO matters; first paint must be fast |
| SSG | Pure static (docs, blogs, marketing) |
| ISR | Mix of static + dynamic; SSG with revalidation |

Most modern frameworks (Next.js, Nuxt, SvelteKit) support all four.
Pick one and document why.

### Step 3: Performance budget

**Core Web Vitals** (Google's ranking signal):

| Metric | Good | Needs improvement | Poor |
|--------|------|-------------------|------|
| LCP (Largest Contentful Paint) | ≤ 2.5s | ≤ 4s | > 4s |
| INP (Interaction to Next Paint) | ≤ 200ms | ≤ 500ms | > 500ms |
| CLS (Cumulative Layout Shift) | ≤ 0.1 | ≤ 0.25 | > 0.25 |

Plus project-specific:
- Bundle size (JS, CSS): < 200KB compressed
- Initial transfer: < 50KB compressed
- Image size: < 200KB per image
- Third-party JS: < 50KB total

**Enforce with CI**: lighthouse-ci / pa11y-ci / bundlewatch.

### Step 4: Architecture document

`docs/frontend/architecture.md`:

- **Framework**: <react/vue/svelte/vanilla>
- **Rendering**: <spa/ssr/ssg/isr>
- **State management**: <redux/zustand/pinia/none>
- **Data fetching**: <react-query/swr/RTK Query>
- **Styling**: <tailwind/css-modules/styled-components>
- **Component library**: <shadcn/mantine/chakra/none>
- **Testing**: <vitest/jest/playwright/cypress>
- **Build**: <vite/webpack/turbo>
- **CI/CD**: <github actions/vercel/netlify>

### Step 5: Performance budget

`docs/frontend/performance-budget.md`:

| Metric | Budget | Current | Action |
|--------|--------|---------|--------|
| LCP | < 2.5s | _ | Alert if > 2.5s |
| INP | < 200ms | _ | Alert if > 200ms |
| CLS | < 0.1 | _ | Alert if > 0.1 |
| JS bundle | < 200KB | _ | Block PR if > 250KB |
| CSS bundle | < 50KB | _ | Block PR if > 75KB |
| LCP transfer | < 50KB | _ | Alert if > 75KB |

## Frontend-specific gotchas

| Issue | Impact | Fix |
|-------|--------|-----|
| Bundle bloat | Slow first load | Bundle analyzer; lazy load; tree-shake |
| Layout shift (CLS) | Bad UX | Reserve space for images/ads; web fonts with size-adjust |
| Slow INP | Feels janky | Break up long tasks; defer non-critical work |
| Memory leak in SPA | Browser tab crashes | Profile with DevTools Memory; fix on every release |
| Hydration mismatch (SSR) | White flash, errors | Server-render + client-render the same data |
| SEO miss | No organic traffic | SSR/SSG; meta tags; structured data |

## Examples

### Example 1: SaaS dashboard (Next.js + React)

```
Framework: React + Next.js
Rendering: SSR (auth-gated dashboard; SEO doesn't matter but SSR for first-paint speed)
State: Zustand (lightweight; no Redux needed)
Data: React Query (cache + retry + optimistic updates)
Styling: Tailwind + shadcn/ui
Build: Vite (via Turbopack)
Testing: Vitest + Playwright
CI: GitHub Actions + Vercel
Bundle budget: < 250KB JS, < 50KB CSS
LCP target: < 1.5s (auth-gated; can preload user data)
```

### Example 2: marketing site + docs (Astro)

```
Framework: Astro (zero-JS by default)
Rendering: SSG (docs); ISR (blog with weekly updates)
State: none (or Alpine.js for tiny interactivity)
Data: fetch() at build time
Styling: Tailwind
Build: Vite (built into Astro)
Testing: Playwright (visual regression)
CI: GitHub Actions + Cloudflare Pages
Bundle budget: < 50KB JS total (most pages ship 0 JS)
LCP target: < 1s (static content, edge-cached)
```

## Anti-patterns

### ❌ Adding dependencies without checking bundle size

`npm install date-fns` adds 200KB+ to your bundle. Use `bundlephobia.com`
or `@bundlephobia/cli` before installing. Prefer native `Intl.DateTimeFormat`
or a small library (`date-fns/locale/en-US/format`) when you only need
formatting.

### ❌ "It's fast enough on my MacBook"

That's not a budget. A 3G user in rural Brazil sees your site 100×
slower. Set explicit budgets (LCP < 2.5s, INP < 200ms, bundle < 200KB)
and enforce them in CI.

### ❌ Skipping a11y until "later"

Adding keyboard navigation, ARIA labels, color contrast AFTER
launch is painful. Often incomplete (because it requires re-doing
component work). Bake a11y in from the first component: use semantic
HTML, label every input, test with a screen reader once per release.

### ❌ Using localStorage for sensitive data

localStorage is readable by any JavaScript on the page. XSS = full
data leak. Use HttpOnly cookies for auth tokens; IndexedDB only for
non-sensitive client state.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Bundle regression > 50KB | Bundle analyzer → identify culprit → lazy load or replace |
| LCP > 4s on mobile | Lighthouse CI; check images, fonts, JS parse time |
| a11y audit failure | Use axe DevTools; fix by priority (color contrast > aria > keyboard) |
| Cross-browser bug | Test on real devices (BrowserStack); polyfill if needed |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/kickoff` | Initial architecture decision (uses frontend inputs) |
| `/mobile` | If shipping a PWA alongside native apps |
| `/roadmap` | Track framework upgrades, deprecations |
| `/incident` | Production perf regression; cross-browser bug |