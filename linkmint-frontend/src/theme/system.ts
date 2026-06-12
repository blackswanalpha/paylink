/**
 * "Ivory Premium" — the LinkMint design system as a Chakra UI v3 system.
 *
 * Light-first: a warm ivory canvas, ink type, a single emerald jewel accent, and restrained
 * champagne gold for celebratory moments (a settlement). Built by extending Chakra's `defaultConfig`
 * so every existing component keeps working — we override the base tokens + the built-in semantic
 * tokens (`bg`, `fg`, `border`) so screens that already use `fg.muted` / `bg.panel` inherit the
 * palette automatically.
 *
 * Spec: see `frontendfeature.md` §2 (Design System). Status colors here back `StatusPill`.
 * Fonts resolve to the families loaded via <link> in `app/layout.tsx` (display=swap), with robust
 * fallbacks so a build/render without network still renders correctly.
 */

import { createSystem, defaultConfig, defineConfig } from '@chakra-ui/react';

const config = defineConfig({
  theme: {
    tokens: {
      fonts: {
        heading: { value: "'Fraunces', Georgia, 'Times New Roman', serif" },
        body: { value: "'Inter', system-ui, -apple-system, Segoe UI, Roboto, sans-serif" },
        mono: { value: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace" },
      },
      colors: {
        // Ivory neutrals
        canvas: { value: '#FAF7F0' },
        surface: { value: '#FFFFFF' },
        surfaceSubtle: { value: '#F4F0E7' },
        hairline: { value: '#E7E1D5' },
        ink: { value: '#1C1A17' },
        inkMuted: { value: '#6B655C' },
        // Emerald accent ramp (primary = emerald.600 #0F6E4E)
        emerald: {
          50: { value: '#E9F3EC' },
          100: { value: '#CBE3D3' },
          200: { value: '#A3CEB2' },
          300: { value: '#72B690' },
          400: { value: '#3E9A6C' },
          500: { value: '#1A8055' },
          600: { value: '#0F6E4E' },
          700: { value: '#0B5840' },
          800: { value: '#084231' },
          900: { value: '#052D22' },
          950: { value: '#031C16' },
        },
        // Champagne / gold highlight — full 50–950 ramp so `colorPalette="champagne"` resolves
        champagne: {
          50: { value: '#FBF6E9' },
          100: { value: '#F3E7C4' },
          200: { value: '#E6CF8F' },
          300: { value: '#D8B65C' },
          400: { value: '#C8A24B' },
          500: { value: '#B08C37' },
          600: { value: '#8E6F2A' },
          700: { value: '#6F571F' },
          800: { value: '#514018' },
          900: { value: '#362A0F' },
          950: { value: '#221A09' },
        },
        // Status hues
        statusPending: { value: '#B8860B' },
        statusDanger: { value: '#B4452F' },
        statusExpired: { value: '#8A7E6A' },
      },
      radii: {
        sm: { value: '6px' },
        md: { value: '10px' },
        lg: { value: '14px' },
        xl: { value: '20px' },
        '2xl': { value: '28px' },
      },
      shadows: {
        xs: { value: '0 1px 2px rgba(28, 26, 23, 0.05)' },
        sm: { value: '0 2px 8px rgba(28, 26, 23, 0.06)' },
        md: { value: '0 8px 24px rgba(28, 26, 23, 0.08)' },
        lg: { value: '0 20px 48px rgba(28, 26, 23, 0.10)' },
      },
      // Motion (frontendfeature.md §2.4 / ADR-012) — distinct `lm*` names so Chakra's own
      // durations.fast/slow (referenced by the built-in overlay recipes) stay intact.
      durations: {
        lmFast: { value: '120ms' },
        lmBase: { value: '200ms' },
        lmSlow: { value: '320ms' },
      },
      easings: {
        lmStandard: { value: 'cubic-bezier(0.2, 0.8, 0.2, 1)' },
      },
    },
    semanticTokens: {
      colors: {
        // Re-map Chakra's built-in surface/text/border semantics to the Ivory palette so existing
        // components (Card, Text color="fg.muted", etc.) pick it up with no code change.
        bg: {
          DEFAULT: { value: '{colors.canvas}' },
          subtle: { value: '{colors.surfaceSubtle}' },
          muted: { value: '{colors.surfaceSubtle}' },
          panel: { value: '{colors.surface}' },
        },
        fg: {
          DEFAULT: { value: '{colors.ink}' },
          muted: { value: '{colors.inkMuted}' },
          subtle: { value: '{colors.inkMuted}' },
        },
        border: {
          DEFAULT: { value: '{colors.hairline}' },
          muted: { value: '{colors.hairline}' },
        },
        // Brand accents
        accent: {
          solid: { value: '{colors.emerald.600}' },
          emphasized: { value: '{colors.emerald.700}' },
          subtle: { value: '{colors.emerald.50}' },
          fg: { value: '{colors.emerald.700}' },
        },
        gold: {
          solid: { value: '{colors.champagne.400}' },
          subtle: { value: '{colors.champagne.50}' },
          fg: { value: '{colors.champagne.600}' },
        },
        // Palette-group semantics: Chakra v3 does NOT auto-generate `<palette>.solid/contrast/…`
        // for custom ramps, and the built-in recipes (Button solid, Badge, …) resolve exactly
        // these names — without them `colorPalette="emerald"` paints an unresolved var.
        emerald: {
          solid: { value: '{colors.emerald.600}' },
          contrast: { value: 'white' },
          fg: { value: '{colors.emerald.700}' },
          muted: { value: '{colors.emerald.100}' },
          subtle: { value: '{colors.emerald.50}' },
          emphasized: { value: '{colors.emerald.200}' },
          focusRing: { value: '{colors.emerald.600}' },
        },
        champagne: {
          solid: { value: '{colors.champagne.400}' },
          contrast: { value: '{colors.ink}' },
          fg: { value: '{colors.champagne.600}' },
          muted: { value: '{colors.champagne.100}' },
          subtle: { value: '{colors.champagne.50}' },
          emphasized: { value: '{colors.champagne.200}' },
          focusRing: { value: '{colors.champagne.500}' },
        },
        // Status semantics — the single source consumed by StatusPill (see §2.6)
        status: {
          success: { value: '{colors.emerald.600}' },
          successSubtle: { value: '{colors.emerald.50}' },
          pending: { value: '{colors.statusPending}' },
          pendingSubtle: { value: '#FBF1D6' },
          neutral: { value: '{colors.inkMuted}' },
          neutralSubtle: { value: '{colors.surfaceSubtle}' },
          danger: { value: '{colors.statusDanger}' },
          dangerSubtle: { value: '#F6E4DF' },
          expired: { value: '{colors.statusExpired}' },
          expiredSubtle: { value: '#EFEAE0' },
        },
      },
    },
    recipes: {
      // Press micro-interaction on every button (ADR-012). The transform animates via Chakra's
      // built-in `transitionProperty: "common"` (which includes transform); the global
      // prefers-reduced-motion rule (globals.css) makes it instant when motion is reduced (F.6).
      button: {
        base: {
          _active: { transform: 'scale(0.97)' },
        },
      },
    },
  },
  globalCss: {
    'html, body': {
      bg: 'canvas',
      color: 'fg',
      fontFamily: 'body',
    },
    '::selection': {
      bg: 'emerald.100',
      color: 'emerald.900',
    },
    '*:focus-visible': {
      outline: '2px solid',
      outlineColor: 'emerald.600',
      outlineOffset: '2px',
    },
  },
});

export const system = createSystem(defaultConfig, config);
