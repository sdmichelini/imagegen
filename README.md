# imagegen

Simple Go CLI for generating branded images.

## Models

- `google/gemini-2.5-flash-image`
- `openai/gpt-5-image-mini`

## Setup

1. Set your OpenRouter API key:

```bash
export OPEN_ROUTER_API_KEY=your_key_here
```

2. Install dependencies:

```bash
make install
```

3. Build binaries and web assets:

```bash
make build
```

Optional frontend lint + type-check:

```bash
make web-lint
make web-typecheck
```

## Usage

Generate with both models:

```bash
./imagegen -prompt "A clean launch announcement banner for a fintech app"
```

Generate with a single model and brand directory:

```bash
./imagegen \
  -prompt "Hero image for homepage, modern and friendly" \
  -brand-dir ./brand \
  -model openai \
  -image-size 2K \
  -aspect-ratio 3:2 \
  -out ./generated
```

## Flags

- `-prompt` (required): short image prompt
- `-brand-dir` (optional): directory with text files describing brand guidelines
- `-model`: `google`, `openai`, or `both` (default: `both`)
- `-out`: output directory (default: `output`)
- `-image-size`: image size `1K`, `2K`, or `4K` (default: `1K`, used by Gemini image model)
- `-aspect-ratio`: optional ratio: `1:1`, `2:3`, `3:2`, `3:4`, `4:3`, `4:5`, `5:4`, `9:16`, `16:9`, `21:9`
- `-n`: number of images per selected model (default: `1`)
- `-output-format`: output format `jpg`, `png`, `webp`, or `ico` (default: `jpg`)
- `-ico-sizes`: comma-separated ICO sizes from `16x16`, `32x32`, `48x48` (default: `16x16,32x32,48x48`)

## Notes

- The CLI reads non-binary UTF-8 files from `-brand-dir` (top-level only).
- Files larger than 512KB are skipped.
- Generated files are saved with model + UTC timestamp in the filename.
- When `-output-format ico` is used, generation is forced to `1:1` and encoded with the sizes selected in `-ico-sizes`.

## Web App (Scaffold)

Run the local web app server in dev mode (builds first):

```bash
make dev
```

Optional overrides:

```bash
make dev ADDR=:9090 DATA_DIR=/tmp/imagegen-dev
```

Direct run after `make build`:

```bash
./imagegen-web -addr :8080 -data-dir ~/.imagegen
```

Web app persistence:
- SQLite metadata database: `~/.imagegen/imagegen.db`
- Generated image files: `~/.imagegen/images/...`

Long-running generate actions are processed asynchronously:
- Submit from a work item page
- Track status in `/jobs`
- Inspect failures in `/jobs/{id}`
