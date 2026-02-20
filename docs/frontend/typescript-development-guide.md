# TypeScript Development Guide

## Progressive Disclosure

Read this in order when implementing frontend code:

1. `docs/frontend/design-principles.md`
2. `docs/frontend/frontend-file-layout.md`
3. `docs/frontend/typescript-migration-plan.md`
4. `docs/frontend/typescript-development-guide.md`
5. `docs/frontend/web-ui-design.md`

## Directory Layout for TS

```text
web/static/js/src/
  core/
    api.ts
    dom.ts
    events.ts
  pages/
    brands.ts
    brand-edit.ts
    project-detail.ts
    work-item-detail.ts
  types/
    domain.ts
    ui.ts
  app.ts
```

Rules:
- `core/`: framework-agnostic helpers.
- `pages/`: page entrypoints with behavior bound to template structure.
- `types/`: shared type contracts and small data-shape helpers.

## TS Compiler Configuration

Recommended defaults in `web/tsconfig.json`:

- `strict: true`
- `noUncheckedIndexedAccess: true`
- `exactOptionalPropertyTypes: true`
- `noImplicitOverride: true`
- `noEmit: true` (for `typecheck` script)
- `module: ESNext`, `target: ES2022`
- `moduleResolution: Bundler`

## Coding Standards

- Avoid `any`; use `unknown` then narrow.
- Prefer explicit return types for exported functions.
- Keep page modules thin: delegate shared logic to `core/`.
- Use small, typed guard helpers for DOM narrowing.
- Use discriminated unions for async UI state where relevant.

## DOM and Event Patterns

- Always null-check queried elements.
- Provide typed helpers:
  - `queryRequired<T extends Element>(selector): T`
  - `isHTMLFormElement(node): node is HTMLFormElement`
- Bind handlers with explicit event types (`SubmitEvent`, `MouseEvent`).

## Form and Async UX Pattern

Every async form should implement:

- default state
- loading state (disable submit + `aria-busy=true`)
- success state (flash/toast)
- error state (inline + page-level alert)

Use shared utility types:

```ts
type AsyncState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'success'; message: string }
  | { status: 'error'; message: string };
```

## Data Contracts

Keep frontend contracts explicit even for server-rendered pages:

- Parse `data-*` attributes through typed parsers.
- Validate external JSON payloads before use.
- Avoid assuming optional fields are present.

## Build Integration

- esbuild entry points should reference `.ts` files.
- Output artifact names must stay stable logically:
  - `app.ts` -> `app-<hash>.js`
  - `brands.ts` -> `brands-<hash>.js`
- `manifest.json` keys must remain unchanged from current template usage.

## Review Checklist

Before merging TS changes:

1. `npm run lint` passes.
2. `npm run typecheck` passes.
3. `npm run build` passes.
4. `make web-lint` passes.
5. `make web-typecheck` passes.
6. `make build` passes.
7. No template asset key changes without explicit migration.
8. No newly introduced N+1-like frontend fetch behavior.
