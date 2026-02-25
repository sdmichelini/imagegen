# ImageGen

ImageGen is an application to generate images from different providers.

## Architecture

Read `ARCHITECTURE.md` first before making structural changes.

## Important Files

- `cmd/imagegen/main.go` - main CLI application logic

## Build Commands

- `make install` - download Go module dependencies
- `make build` - build the binary at `./imagegen`
- `make web-lint` - lint TypeScript frontend code
- `make web-typecheck` - strict TypeScript type checking

## TypeScript Workflow

- After editing any `web/static/js/src/**/*.ts` file, run:
  - `make web-lint`
  - `make web-typecheck`
  - `make build`

## Build Quality

- Compile/build changes without warnings.
