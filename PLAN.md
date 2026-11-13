# Riven Plan

Status key: `[ ]` todo · `[~]` in progress · `[x]` done (tests pass + checked in host app).
Each task has a **Done when** line: that is the acceptance check used in review.
Work top to bottom; don't start a milestone until the previous one's gate passes.

---

## M0 — Package foundation
- [x] Resolve dev deps for Laravel 13 (`orchestra/testbench`, `pestphp/pest`)
  - Done when: `composer install` succeeds and `vendor/bin/pest` runs (0 tests is fine)
- [x] Testbench base `TestCase` that loads `RivenServiceProvider`
  - Done when: one smoke test asserts `RouteScanner` resolves from the container as a singleton
- [x] Host app wiring (`riven-app` path repo, symlinked)
  - Done when: `php artisan list` in `riven-app` shows the `riven` namespace

**Gate:** tests run in the package and the host app sees the provider.

## M1 — PHP collector
- [x] `riven:scan` (`src/Commands/ScanRoutesCommand.php`) prints routes as a table
  - Done when: output shows uri, methods, name, action for `riven-app` routes
- [x] RouteScanner filtering: exclude framework/vendor routes (e.g. `_ignition`, `sanctum/*`, `storage/*`, closures from vendor providers); configurable
  - Done when: a test registers app + vendor-like routes and only app routes come back
- [x] RouteScanner enrichment: middleware (resolved, not aliases only), parameter names + optional flag + `where` patterns, domain, action type (controller / invokable / closure)
  - Done when: tests cover each field on a fixture route
- [x] Controller reflection: resolve the FormRequest type-hinted on the action
  - Done when: a fixture controller method with `StoreUserRequest $request` yields that class name
- [x] FormRequest rules extraction (`rules()` output, string and array rule formats normalized)
  - Done when: rules appear in manifest shape; rules needing a live request are skipped with a `warnings[]` entry, not a crash
- [x] Resource detection: return type / `new XResource` / `XResource::collection` (best effort, static)
  - Done when: fixture returns are detected; unknown → `null`, not a guess
- [x] Config file `config/riven.php` (publishable): include/exclude patterns, output path
  - Done when: `vendor:publish --tag=riven-config` works and options are honored
- [x] `riven:export` writes `riven.json` matching the Manifest contract below
  - Done when: output validates against `schema/riven.schema.json` in a test

**Gate:** `riven:export` on `riven-app` produces a valid manifest; schema frozen as v1. ✅ passed 2026-11-13

## M2 — Go engine skeleton (`engine/` in this repo)
- [x] Go module in `engine/`, CLI framework choice (cobra vs stdlib `flag`), decide with reasons
  - Done when: `go test ./...` passes and `go run ./cmd/riven-engine --version` works
- [x] Manifest structs + loader with `schema_version` check (`engine/manifest`)
  - Done when: loads the M1 output; rejects an unknown major version with a clear error
- [x] `riven-engine routes` prints routes
  - Done when: output matches `riven:scan` for the same app (cells identical on riven-app; artisan collapses cell padding, see log)
- [ ] Golden-file test using a committed sample manifest

## M3 — Stress tester
- [ ] Worker pool (goroutines), configurable concurrency, duration / request count
- [ ] Per-route targeting from manifest, param substitution, auth header injection
- [ ] Metrics: p50/p95/p99 latency, RPS, error rate, status histogram
- [ ] JSON report output (consumed later by the dashboard)
  - Done when: run against `riven-app` locally produces a report; a test uses `httptest.Server`

## M4 — Docs (static + AI)
- [ ] Static OpenAPI 3.1 builder from manifest (paths, params, request bodies from rules)
  - Done when: output passes an OpenAPI validator
- [ ] Rule → JSON Schema mapping table (required, string, email, integer, min/max, in, array, nullable…)
- [ ] Claude API pass: fill descriptions/examples only where static data is missing; cache by route hash
  - Done when: static output alone is valid; AI only adds fields; re-run with no changes makes 0 API calls
- [ ] Eval set: a few fixture routes with expected descriptions, scored, to compare prompts/models

## M5 — Dashboard
- [ ] React app (Vite) built into `go:embed`, served by `riven-engine serve`
- [ ] Views: routes list, route detail (docs), stress reports

## M6 — Ship
- [ ] AI test generator (Pest tests from manifest + docs), with an eval like M4
- [ ] Cross-platform binaries (goreleaser), checksums
- [ ] Composer post-install downloads the matching binary (verify checksum)
- [ ] README, LICENSE, tag v0.1.0, publish to Packagist

---

## Manifest contract (schema_version 1, frozen)

**Source of truth: `schema/riven.schema.json`** (JSON Schema 2020-12, strict: unknown fields
are rejected, so any new field must be added there first). The example below is illustrative.

Rules: additive changes = minor; renames/removals = bump `schema_version`.
Unknown values are `null`, never guessed. Non-fatal problems go in `warnings`.

```json
{
  "schema_version": 1,
  "generator": { "name": "faysal/riven", "version": "0.1.0" },  // version null if unknown
  "generated_at": "2026-11-13T12:00:00Z",
  "app": {
    "name": "riven-app",
    "url": "http://localhost:8000",
    "laravel_version": "13.42.0",
    "php_version": "8.4.24"
  },
  "routes": [
    {
      "id": "POST /api/users",
      "uri": "api/users",
      "methods": ["POST"],
      "name": "users.store",
      "domain": null,
      "action": {
        "type": "controller",
        "class": "App\\Http\\Controllers\\UserController",
        "method": "store"
      },
      "middleware": ["api", "auth:sanctum"],
      "middleware_resolved": ["Illuminate\\Routing\\Middleware\\SubstituteBindings", "Illuminate\\Auth\\Middleware\\Authenticate:sanctum"],
      "parameters": [
        { "name": "user", "required": true, "pattern": null }
      ],
      "request": {
        "form_request": "App\\Http\\Requests\\StoreUserRequest",
        "rules": { "email": ["required", "email"], "name": ["required", "string", "max:255"] }
      },
      "response": { "resource": "App\\Http\\Resources\\UserResource", "collection": false }
    }
  ],
  "warnings": [
    { "route_id": "POST /api/users", "code": "rules_need_request", "message": "..." }
  ]
}
```

Field notes:
- `id`: one per route, `"<first non-HEAD method> /<uri>"` (e.g. `PUT /users/{user}` for PUT|PATCH).
- `action.type`: `controller` | `invokable` | `closure`; `class`/`method` are `null` for closures.
- `middleware`: as declared (aliases, groups, controller middleware). `middleware_resolved`:
  class names after expanding aliases/groups and removing `withoutMiddleware()` exclusions.
- `parameters`: includes domain parameters; `pattern` comes from `where()` (incl. global patterns).
- `request.rules`: `field => [rule strings]`, `null` when there's no FormRequest or `rules()` needs a
  live request (then a `rules_need_request` warning). Strings are Laravel's own syntax (`max:255`,
  `in:"a","b"`); rule objects without a string form appear as their class name, closures as `Closure`.
  Wildcard keys (`items.*.id`) are kept as-is. A FormRequest with no rules gives `{}`.
- `response`: `{resource, collection}`, both `null` when unknown (no guessing). From the declared
  return type, else from every `return` in the action body (`new X`, `X::make`, `X::collection`,
  through chained calls); all returns must agree. Named collections report the item resource
  (`$collects`, else Laravel's `UserCollection` → `UserResource` naming rule).

- `warnings[].code`: `controller_middleware_unresolved` | `rules_need_request` (new codes are additive).

Deferred (additive, no version bump needed):
- Resource field shapes (`toArray()` keys/types): static keys + AI types in M4.
- Source locations (`action.file`/`start_line`/`end_line`, `response.resource_file`) and a
  top-level `resources[]` candidate list, for the M4 AI fallback.

---

## Decision log
| Date | Decision | Why |
|------|----------|-----|
| 2026-11-13 | PHP collects, Go processes, JSON manifest between them | Collector needs the booted app; everything else is faster/more portable in a single binary |
| 2026-11-13 | Route filtering by URI patterns + "action defined under vendor/" (reflection on closure/controller file); `Route::view`/`redirect` controllers allowlisted | No route carries its registering provider; the action's file is the most reliable ownership signal |
| 2026-11-13 | Route `id` = first non-HEAD method + uri, one entry per route | Keeps one manifest entry per Laravel route; methods stay in `methods[]` |
| 2026-11-13 | Store both declared and resolved middleware (additive, schema stays v1) | Declared names explain the route to humans/AI; resolved classes make auth detection reliable |
| 2026-11-13 | `RouteScanner::scan()` returns `{routes, warnings}` | Manifest-shaped; non-fatal problems (e.g. unbuildable controller) surface as warnings, not crashes |
| 2026-11-13 | `ActionReflector` reflects the action (closure / controller method) for FormRequest and Resource detection; vendor check keeps reflecting the controller *class* file | `ReflectionMethod::getFileName()` is the declaring file, so inherited vendor methods would misclassify app controllers |
| 2026-11-13 | Scanner binding resolves the HTTP kernel first | Middleware groups/aliases are synced to the router in the HTTP kernel constructor; Artisan never builds it, so `web` stayed unresolved |
| 2026-11-13 | Rules stay raw Laravel strings; the Go engine maps them to JSON Schema | Laravel's syntax is already a stable, documented format; mapping once in Go (M4) avoids two parsers |
| 2026-11-13 | FormRequests are instantiated with `new`, never resolved from the container; PHP warnings in `rules()` are thrown | The container validates FormRequests on resolve; warnings (e.g. `null->id`) would otherwise yield silently wrong rules depending on the caller's error handler |
| 2026-11-13 | Resource detection uses `nikic/php-parser` (^5, new runtime dep) on the action's own `return`s; ambiguity → `null` | Already installed in every Laravel app (via Tinker/psysh); an AST with `use` resolution is reliable where token matching isn't; limited patterns keep it far from a full inference engine (Scramble) |
| 2026-11-13 | Config: `include` / `exclude` URI patterns (exclude wins), `exclude_vendor` toggle, relative `output` path; read only in the provider's scanner binding | The scanner stays config-free and testable with plain arguments; one place maps config → constructor (named args) |
| 2026-11-13 | AI fallback (M4, Go engine): providers `none` (default) / `ollama` / `anthropic`; choose-from-candidates schema; cached by source hash | Keeps Riven deterministic by default, free+private option via local models, constrained output prevents invented class names |
| 2026-11-13 | Manifest v1 frozen; `schema/riven.schema.json` is the source of truth, strict (`additionalProperties: false`); Resource shapes deferred | Strict schema makes contract drift fail tests instead of silently reaching the engine; deferred fields are additive |
| 2026-11-13 | Monorepo: Go engine in `engine/` (module `github.com/faysal/riven/engine`), not a separate repo; `.gitattributes` export-ignores it | One place for both halves and the schema they share; a subdirectory because Go reads a root `vendor/` as Go vendoring, which Composer's `vendor/` breaks; Composer installs stay PHP-only (the binary is downloaded, M6) |
| 2026-11-13 | Engine CLI uses Cobra; `internal/cli` only parses flags and prints, logic in its own packages | Several subcommands are planned (`routes`, `stress`, `docs`, `serve`) and Cobra gives per-command flags, help and completion; stdlib `flag` has no subcommands. Thin CLI layer keeps logic testable and the framework swappable |
| 2026-11-13 | Go manifest types: JSON null → nil (pointers for scalars, nil map for `rules`); `schema_version` checked before full decode; unknown fields ignored | nil keeps "unknown" distinct from zero values (`{}` rules decode to an empty map, `null` to nil); version-first gives a clear error instead of a type error on an incompatible file; additive fields don't bump the version, so the engine must tolerate them (strictness stays in the PHP schema test) |
| 2026-11-13 | `riven-engine routes` renders Symfony's default table style itself (`internal/table`, no dependency); warnings go to stderr | Same look as artisan with ~60 lines instead of a table library; stderr keeps `routes > file` clean. Open oddity: `riven:scan` in riven-app prints correct borders but unpadded cells, while plain Symfony `Table` pads in the same app; cell content is identical, cause not found yet |
