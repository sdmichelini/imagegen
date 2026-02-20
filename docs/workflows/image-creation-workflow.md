# Image Creation Workflow Plan

## Goals

- Provide an interactive local web app workflow for creating and iterating images.
- Support `Brand` creation and editing (brands are text content stored as Markdown).
- Organize work as `Project -> Work Item -> Image Candidates`.
- Persist everything on local disk with a default data root of `~/.imagegen`.
- Keep the current CLI unchanged; web app is additive.

## Domain Model

- `Brand`
  - Purpose: reusable style/system prompt context.
  - Fields: `id`, `name`, `slug`, `description`, `content_markdown`, `created_at`, `updated_at`.
- `Project`
  - Purpose: container for related creative work.
  - Fields: `id`, `name`, `slug`, `default_brand_id`, `created_at`, `updated_at`.
- `WorkItem`
  - Purpose: specific asset request inside a project (example: `icon` in project `recipe-buddy`).
  - Fields: `id`, `project_id`, `name`, `slug`, `type`, `prompt`, `constraints`, `status`, `created_at`, `updated_at`.
- `ImageCandidate`
  - Purpose: generated output versions for a work item.
  - Fields: `id`, `work_item_id`, `model`, `prompt_snapshot`, `settings_snapshot`, `path`, `format`, `created_at`, `score`.

## Local Storage Layout (Default: `~/.imagegen`)

```text
~/.imagegen/
  brands/
    acme.md
    recipebuddy.md
  projects/
    recipe-buddy/
      project.json
      work-items/
        icon/
          work-item.json
          prompts.md
          images/
            cand-001-openai.png
            cand-002-google.png
          comparisons.json
          final/
            icon-final.png
```

Notes:
- Brand files are Markdown (`.md`) and are the source of truth for brand text.
- Images are plain files on disk.
- JSON sidecars store metadata for projects, work items, generation parameters, and comparisons.

## Primary User Flows

1. Brand management
- Create brand: name + markdown content.
- Edit brand: update markdown and save version timestamp.
- List brands and preview markdown.

2. Project setup
- Create project with name/slug.
- Choose default brand (optional).
- View project dashboard with work items.

3. Work item creation
- Create work item (name/type like `icon`, `hero`, `thumbnail`).
- Enter base prompt and generation constraints.
- Optionally override project default brand.

4. Generate candidates
- Submit generation request with model, output format, size, aspect ratio, count.
- Backend creates candidate image files under the work item folder.
- UI updates candidate grid as images become available.

5. Compare and iterate
- Show pairwise comparisons (two at a time).
- Persist user picks to `comparisons.json`.
- Support regenerate with adjustment text while preserving history.

6. Finalize
- Mark one candidate as final and copy to `final/`.
- Allow direct download and quick open-in-folder action.

## Web App Delivery Plan

### Phase 1: Foundational local app
- Go HTTP server with template rendering + minimal JSON endpoints.
- Disk repositories for brands/projects/work-items/images.
- Basic JS UI for forms, candidate list updates, and compare actions.

### Phase 2: Generation orchestration
- Web backend calls existing internal generation path (without changing CLI behavior/flags).
- Track generation jobs per work item.
- Better retry + error surfaces in UI.

### Phase 3: Iteration quality
- Pairwise ranking helper and quick regenerate loop.
- Work item history timeline (prompt/settings/candidates).
- Export bundle (final image + metadata).

## Technical Decisions

- Storage: local disk only for now (`~/.imagegen` configurable later).
- API style: mixed templates + JSON APIs for interactive actions.
- Frontend: server-rendered pages, progressive enhancement with vanilla JS bundles via esbuild.
- Auth: none initially (single-user local environment).

## Non-Goals (Current Scope)

- No cloud sync or multi-user collaboration.
- No database dependency in the initial implementation.
- No modification of current CLI UX.

## Open Questions for Iteration

- Should brand files allow frontmatter metadata or remain pure markdown text?
- Should long-running generation be synchronous (simple) or queued background jobs first?
- Do we need soft-delete/archive semantics for projects and work items in v1?
