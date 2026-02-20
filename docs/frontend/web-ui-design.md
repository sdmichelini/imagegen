# Frontend Design Notes

## Direction

- Build a pragmatic server-rendered web UI with interactive enhancements.
- Use plain TypeScript modules (no framework) for now.
- Bundle assets with esbuild, following the style used in `~/dev/recipebuddy`.
- Apply the design system from `docs/frontend/design-principles.md`.
- Use the file structure and build pipeline in `docs/frontend/frontend-file-layout.md`.
- Follow TypeScript migration and coding standards in `docs/frontend/typescript-migration-plan.md` and `docs/frontend/typescript-development-guide.md`.

## Information Architecture

- `/` Dashboard: recent projects, quick actions.
- `/about` Concept page that defines Brand, Project, and Work Item.
- `/brands` Brand list + create action.
- `/brands/:id` Brand editor (markdown textarea + preview pane).
- `/projects` Project list + create action.
- `/projects/:id` Project detail + work items.
- `/projects/:id/work-items/:wid` Work item detail with candidate gallery + compare/regenerate controls.

## Interaction Model

- Forms post to server for create/update actions.
- TS/JS enhances UX for:
  - inline validation and disabled states
  - optimistic candidate list refresh after generate
  - pairwise comparison voting without full page reload
  - markdown preview rendering in brand editor

## UI Components

- `BrandEditor` textarea + preview + save status.
- `ProjectForm` name/slug/default-brand selector.
- `WorkItemForm` prompt + constraints + model settings.
- `CandidateGrid` image cards with metadata.
- `ComparePanel` two-up chooser and skip/regenerate controls.
- `AboutConceptCards` three cards for Brand/Project/Work Item definitions.

## Visual Rules

- All feature sections render as cards with layered depth.
- Neutral background surfaces + white cards are the default canvas.
- Two primaries only (plus neutrals/white): one main CTA color and one accent color.
- Hover states increase visual depth for buttons and actionable cards.
- Button text is intentionally larger (`1rem` minimum) for stronger affordance.
- Increase vertical spacing between card sections (target `1.25rem` to `1.5rem`).

## Build and Asset Pipeline

- Add a local `package.json` with scripts:
  - `build`: node esbuild config
  - `watch`: node esbuild config with watch mode
- esbuild config patterns (matching recipebuddy approach):
  - multiple JS entrypoints
  - optional CSS entrypoints
  - hashed output filenames
  - generated asset manifest for template helpers

## Responsive Behavior

- Desktop: split panes for editor/preview and compare views.
- Mobile: stacked sections, fixed bottom action bar for voting and regenerate.

## Accessibility Baseline

- Proper labels and focus order for all forms.
- Keyboard-selectable compare actions.
- Alt text from work item name + candidate id.
