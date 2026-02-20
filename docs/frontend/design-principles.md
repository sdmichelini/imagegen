# Frontend Design Principles

## Visual System

- UI is card-first across all screens.
- Use HSL tokens for all colors, including shadows.
- Keep backgrounds neutral (warm grays), not tinted with primary colors.
- Define exactly two primary brand colors and reuse them consistently.
- Keep the rest of the palette white + neutral ramps.

## Color Model (HSL)

```css
:root {
  /* Two primary colors */
  --primary-a-h: 24;
  --primary-a-s: 72%;
  --primary-a-l: 48%;
  --primary-a: hsl(var(--primary-a-h), var(--primary-a-s), var(--primary-a-l));
  --primary-a-strong: hsl(var(--primary-a-h), 74%, 40%);
  --primary-a-soft: hsl(var(--primary-a-h), 80%, 94%);

  --primary-b-h: 188;
  --primary-b-s: 58%;
  --primary-b-l: 42%;
  --primary-b: hsl(var(--primary-b-h), var(--primary-b-s), var(--primary-b-l));
  --primary-b-strong: hsl(var(--primary-b-h), 62%, 34%);
  --primary-b-soft: hsl(var(--primary-b-h), 70%, 94%);

  /* Neutral foundation */
  --neutral-h: 35;
  --neutral-s: 10%;
  --bg-page: hsl(var(--neutral-h), var(--neutral-s), 95%);
  --bg-surface: hsl(var(--neutral-h), 10%, 98%);
  --bg-card: hsl(0, 0%, 100%);
  --border: hsl(var(--neutral-h), 11%, 86%);
  --border-strong: hsl(var(--neutral-h), 12%, 76%);

  --text-1: hsl(28, 12%, 16%);
  --text-2: hsl(30, 8%, 36%);
  --text-3: hsl(30, 7%, 52%);
}
```

## Depth and Cards

- Every major container is a card with radius, border, and layered shadows.
- Depth is communicated by luminance delta and shadow spread, not saturated colors.
- Elevation scale:

```css
:root {
  --shadow-1: 0 1px 2px hsl(var(--neutral-h), 12%, 30%, 0.08);
  --shadow-2: 0 2px 6px hsl(var(--neutral-h), 12%, 30%, 0.10);
  --shadow-3: 0 6px 18px hsl(var(--neutral-h), 12%, 30%, 0.13);
  --shadow-4: 0 14px 34px hsl(var(--neutral-h), 12%, 30%, 0.16);
}

.card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 16px;
  box-shadow: var(--shadow-2);
}
```

## Buttons and Interactive States

- Buttons should increase depth on hover.
- Hover behavior combines slight upward transform + stronger shadow.
- Primary buttons use `--primary-a`; secondary accents use `--primary-b`.

```css
.btn {
  border-radius: 12px;
  border: 1px solid transparent;
  font-size: 1rem;
  font-weight: 700;
  line-height: 1.2;
  padding: 0.7rem 1rem;
  box-shadow: var(--shadow-1);
  transition: transform 140ms ease, box-shadow 140ms ease, background-color 140ms ease;
}

.btn:hover {
  transform: translateY(-1px);
  box-shadow: var(--shadow-3);
}

.btn:active {
  transform: translateY(0);
  box-shadow: var(--shadow-2);
}

.btn-primary {
  background: var(--primary-a);
  color: white;
}

.btn-secondary {
  background: var(--primary-b);
  color: white;
}
```

## Typography

- Avoid default AI-feeling stacks (`Inter`, `Roboto`, generic system-only look).
- Use a warm, friendly, broadly supported stack.

```css
:root {
  --font-ui: "Nunito Sans", "Avenir Next", "Trebuchet MS", "Segoe UI", sans-serif;
  --font-reading: "Source Serif 4", Georgia, serif;
}

body {
  font-family: var(--font-ui);
}
```

Notes:
- Prefer self-hosted font assets when available.
- If web fonts are not loaded, stack falls back to common platform fonts.

## Layout Principles

- Keep content in cards grouped by task (brand editor, project summary, generation controls, candidate review).
- Add more vertical breathing room between cards; default stack gap should be `1.25rem` to `1.5rem`.
- Use consistent spacing rhythm (8px baseline).
- Limit line length for text-heavy content.
- Preserve strong contrast and keyboard-visible focus states.

## About Page Requirement

- Include a dedicated `/about` page in the main nav.
- `/about` must clearly define:
  - `Brand`: reusable text guidance describing style, voice, and constraints.
  - `Project`: container for related image work with a default brand context.
  - `Work Item`: a specific asset task inside a project (for example, `icon` for `recipe-buddy`).
- Present these definitions as three separate cards for quick scanning.

## Implementation Guidance

- Centralize tokens in one base stylesheet.
- Build page styles on top of shared card/button/type tokens.
- Use esbuild to bundle CSS and produce hashed output + manifest entries.
