# Riven

Open-source Laravel developer toolkit, distributed as a Composer package.
Features: hybrid API doc generation (static parsing + AI), stress testing,
route analysis, and AI test generation.

**Roadmap and task status live in `PLAN.md`.** Read it before starting work.
Use the `riven-mentor` skill (`/riven-mentor`) for the next-step / review loop.

## Architecture
Two halves in this one repo (monorepo), connected by a JSON manifest:

- **PHP collector** (repo root, `faysal/riven`, `src/`): runs inside the Laravel app.
  Does only what requires the app: scans routes, reflects controllers, reads
  FormRequest rules, Resource shapes, and middleware. Exports `riven.json`.
- **Go engine** (`engine/`, module `github.com/adnan4k/riven/engine`, binary
  `riven-engine`): reads the manifest. Handles
  stress testing (goroutines), Claude API analysis, OpenAPI building, test
  generation, and a React dashboard embedded via `go:embed`. Shipped as
  cross-platform binaries downloaded on `composer install`. Lives in `engine/`,
  not the root, because Go treats a root `vendor/` as Go vendoring and
  Composer's `vendor/` would break the build. `.gitattributes` keeps `engine/`
  out of Composer dist installs.

The manifest (`riven.json`) is the contract between the two halves. Its schema
is drafted in `PLAN.md` (Manifest contract). Any change to it must bump
`schema_version` and be recorded there.

## Environment
- PHP 8.4, Composer 2.9, Ubuntu
- Package: `~/Desktop/projects/riven` (namespace `Riven\` → `src/`, PSR-4)
- Host app: `~/Desktop/projects/riven-app` (Laravel 13), loads the package via
  a Composer path repository (symlinked, `"faysal/riven": "@dev"`)
- `illuminate/support`: `^11.0|^12.0|^13.0`
- Dev deps (testbench, pest) not installed yet; versions must match Laravel 13
- Intelephense shows a false "Undefined type ServiceProvider" warning; ignore it

## Conventions
- Commands live in `src/Commands/`, named `<Verb><Noun>Command` (e.g. `ScanRoutesCommand`)
- Collectors/scanners live in `src/Scanner/`, return plain arrays (manifest-shaped), no output
- Commands only format/print; logic stays in scanners (so it's testable)
- Tests: Pest + Orchestra Testbench in `tests/`
- Go: `cmd/riven-engine` (main only), `internal/cli` (Cobra commands: flags +
  output only), logic in its own packages (e.g. `manifest/`); stdlib `testing`,
  run `go test ./...` from `engine/`

## How I want to work
- I'm learning: I've used Laravel before, but mostly with AI-written code
- Claude writes the implementation and tests; I read along on my own and
  won't necessarily report back. Treat a task as reviewed once its tests pass
  and it's checked in the host app
- Give a short strategy before each task, and point out what's worth reading
  in the result (Laravel concepts, surprises, decisions made)
- Explain the why behind Laravel concepts, briefly
- Choose architecture on technical merit, not for learning's sake
- Keep answers concise
- When a task is done, mark it `[x]` in `PLAN.md` and log design decisions
- Never run `git commit`; leave changes for me to review and commit
