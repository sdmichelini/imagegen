# TypeScript Migration Plan

## Objective

Migrate frontend JavaScript to TypeScript with minimal risk, preserving the current server-rendered architecture, esbuild pipeline, and hashed dist manifest workflow.

## Scope

- In scope:
  - `web/static/js/src/**/*.js` -> `*.ts`
  - shared DOM/api/event helpers
  - page scripts for brand/project/work-item flows
  - build and typecheck integration in Make and npm scripts
- Out of scope for initial migration:
  - frontend framework adoption
  - SSR template engine changes
  - CSS architecture rewrite

## Current Baseline

- JS is modular and page-scoped.
- Bundling uses esbuild with hashed outputs and `manifest.json`.
- Pages are server rendered and progressively enhanced.

## Migration Strategy

### Phase 0: Tooling foundation

1. Add TypeScript compiler config (`web/tsconfig.json`) with strict settings.
2. Update esbuild to accept `.ts` entry points.
3. Add `npm run typecheck` (`tsc --noEmit`).
4. Add `npm run lint` for TypeScript (`eslint` with `typescript-eslint`).
5. Keep output format and manifest contract unchanged.

Exit criteria:
- `npm run build` and `npm run typecheck` succeed with zero TS errors.

### Phase 1: Core modules first

1. Migrate:
- `core/events.js` -> `events.ts`
- `core/dom.js` -> `dom.ts`
- `core/api.js` -> `api.ts`
2. Define shared utility types:
- `Nullable<T>`
- `SubmitState`
- `ApiError`

Exit criteria:
- All imports resolve through `.ts` modules.
- No `any` in core utilities unless explicitly justified.

### Phase 2: Page modules

1. Migrate page entry points one-by-one:
- `app.ts`
- `pages/brands.ts`
- `pages/brand-edit.ts`
- `pages/project-detail.ts`
- `pages/work-item-detail.ts`
2. Add typed DOM element narrowing guards.
3. Standardize form loading-state handlers and typed event wiring.

Exit criteria:
- All page entrypoints compile in strict mode.
- No runtime behavior changes from baseline.

### Phase 3: Type contracts and hardening

1. Add frontend domain types (`BrandSummary`, `ProjectSummary`, `JobSummary`) where data is read from DOM `data-*` or JSON endpoints.
2. Add explicit parse/validation helpers for numeric/select inputs.
3. Add linting conventions for TypeScript hygiene.

Exit criteria:
- Strict mode remains enabled.
- TS prevents common class of DOM/null and shape errors.

## Build and CI/Make Integration

- `web/package.json` scripts:
  - `build`: esbuild bundles TS entrypoints
  - `watch`: esbuild watch mode
  - `lint`: biome lint check for TS modules
  - `typecheck`: `tsc --noEmit`
- `Makefile`:
  - keep `make build` invoking web build
  - include `web-lint` and `web-typecheck` in CI-quality path

## Performance Requirements (unchanged)

- Keep hashed JS/CSS outputs and manifest lookup.
- Keep immutable cache headers for hashed assets only.
- Keep no sourcemaps in production build by default.

## Risk Management

- Migrate in small commits by module group.
- Preserve entrypoint names (`app`, `brands`, etc.) so template includes and manifest keys do not change.
- Validate each phase with:
  - `npm run lint`
  - `npm run typecheck`
  - `npm run build`
  - `make web-lint`
  - `make web-typecheck`
  - `make build`

## Definition of Done

- All `web/static/js/src` modules are TypeScript.
- Typecheck is part of normal developer workflow.
- Build output format, cache behavior, and runtime UX remain consistent.
