# Backend Design Notes

## Scope

- Add a local HTTP web backend for workflow management.
- Keep existing CLI command behavior unchanged.
- Persist all domain state to local disk (`~/.imagegen` by default).

## Logical Components

- `HTTP Layer`
  - Template routes for main pages.
  - JSON routes for async UI interactions.
- `Application Services`
  - Brand service
  - Project service
  - Work item service
  - Generation service
  - Comparison service
- `Persistence Layer`
  - File-backed repositories (Markdown + JSON + image files)

## Storage Strategy

- Base dir: `~/.imagegen`.
- `brands/*.md` as source of truth for brand text.
- `projects/*/project.json` and nested `work-items/*/work-item.json` for metadata.
- candidate image files in `images/`.
- comparison and ranking state in `comparisons.json`.

## Route Shape (Initial)

Template routes:
- `GET /`
- `GET /brands`
- `GET /brands/{brandID}`
- `GET /projects`
- `GET /projects/{projectID}`
- `GET /projects/{projectID}/work-items/{workItemID}`

JSON routes:
- `POST /api/brands`
- `PUT /api/brands/{brandID}`
- `POST /api/projects`
- `POST /api/projects/{projectID}/work-items`
- `POST /api/work-items/{workItemID}/generate`
- `POST /api/work-items/{workItemID}/compare`
- `POST /api/work-items/{workItemID}/regenerate`
- `POST /api/work-items/{workItemID}/finalize`

## Generation Integration

- Reuse existing generation logic from internal packages where possible.
- Avoid changing CLI flags/commands.
- Capture prompt and settings snapshots with each candidate for reproducibility.

## Validation and Errors

- Centralized validation for slugs, required fields, supported formats/sizes.
- Return structured JSON errors for API routes (`code`, `message`, `field_errors`).
- Template routes render friendly error pages for unrecoverable failures.

## Concurrency Model

- Start with per-request synchronous writes and atomic file replacement for JSON updates.
- Guard conflicting writes with per-resource in-memory mutexes.
- Introduce queued background jobs only if generation latency requires it.

## Observability (Local)

- Structured logs with request id and work item id.
- Log disk write/read failures with concrete file paths.

## Security Posture (v1)

- Local single-user app, no auth in initial iteration.
- Sanitize slugs/paths to prevent traversal.
- Enforce max markdown/file sizes to avoid runaway memory use.
