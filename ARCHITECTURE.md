# Architecture

This project follows the Go project layout guidance from:
https://github.com/golang-standards/project-layout

## Progressive Disclosure

Read only what is needed for the area you are changing.

1. Always start here (`ARCHITECTURE.md`) for system boundaries.
2. If working on workflow/domain behavior, read `docs/workflows/image-creation-workflow.md`.
3. If working on backend routes/services/storage, read `docs/backend/web-backend-design.md`.
4. If working on frontend UI/UX/interaction, read `docs/frontend/design-principles.md` first, then `docs/frontend/frontend-file-layout.md`, then `docs/frontend/typescript-migration-plan.md`, then `docs/frontend/typescript-development-guide.md`, then `docs/frontend/web-ui-design.md`.

## Current Components

- `cmd/imagegen` contains CLI command wiring (`generate`, `convert`).
- `internal/imageconv` contains format conversion utilities for CLI image encoding.
- `cmd/imagegen-web` contains local web server startup.
- `internal/webapp` contains web routing, templates integration, SQLite persistence, and background job processing.

## Web App Runtime Architecture

The web app is additive and does not modify existing CLI UX/flags.

- `HTTP Layer`
  - Server-rendered pages for dashboard, brands, projects, work items, jobs.
  - Form actions submit work and redirect to pages with status banners.
- `Persistence Layer (SQLite)`
  - SQLite file at `~/.imagegen/imagegen.db` is source of truth for:
    - brands
    - projects
    - work_items
    - jobs
    - runs
    - run_images metadata
  - Images remain files on local disk.
- `Job Worker`
  - Long-running operations are submitted as jobs (`queued -> running -> succeeded|failed`).
  - Background worker claims queued jobs and executes generation.
  - Failures are persisted (`jobs.error_message`, `runs.error_message`) and surfaced in UI.

## Async Generate Flow

1. User submits generate form on a work item page.
2. Server inserts a `jobs` row with status `queued` and payload snapshot.
3. Worker claims the job and marks it `running`.
4. Worker creates a `run` record, executes `./imagegen generate`, stores files on disk.
5. Worker inserts `run_images` metadata rows and marks run/job `succeeded`.
6. On errors, worker marks run/job `failed` with explicit error message.

## Data Placement

```text
~/.imagegen/
  imagegen.db
  images/
    <project-slug>/
      <work-item-slug>/
        run-<run-id>/
          <generated files>
```

## Query Performance Constraints

- Avoid N+1 query patterns in page rendering.
- Use joined list queries for jobs/projects/work-items views.
- Use indexed filters for common lookups.

Indexes used:
- unique: `brands.slug`
- unique: `projects.slug`
- unique: `work_items(project_id, slug)`
- `jobs(status, created_at)`
- `jobs(work_item_id, created_at DESC)`
- `runs(work_item_id, created_at DESC)`
- `run_images(run_id, created_at)`

## Frontend Build Strategy

- Use simple JavaScript for interactivity and loading-state UX.
- Bundle static assets with esbuild and hashed filenames + manifest mapping.
- Apply immutable cache headers only for hashed `/static/dist/*` assets.
