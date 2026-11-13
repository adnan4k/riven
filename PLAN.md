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
- [x] Golden-file test using a committed sample manifest
  - Done when: `routes` output on `engine/testdata/riven-app.json` matches `internal/cli/testdata/*.golden`; `-update` regenerates

**Gate:** engine loads the M1 manifest and lists routes; tests green in both halves. ✅ passed 2026-11-13

## M3 — Stress tester
- [x] Worker pool (goroutines), configurable concurrency, duration / request count (`engine/stress`)
  - Done when: `httptest` tests prove exact request counts, the concurrency cap, connection reuse, duration stop and cancel; race detector clean
- [x] Per-route targeting from manifest, param substitution, auth header injection (`engine/targets`, `riven-engine stress`)
  - Done when: `--dry-run` lists the right URLs for riven-app; params escaped and checked against `where()`; headers reach the server in an `httptest` run
- [x] Metrics: p50/p95/p99 latency, RPS, error rate, status histogram (`stress.Summarize`, table in `riven-engine stress`)
  - Done when: exact percentiles/RPS/error rate on fixed results in tests; real timeouts and refused connections classified; table printed for riven-app
- [x] JSON report output (consumed later by the dashboard) (`engine/report`, `riven-engine stress -o report.json`)
  - Done when: run against `riven-app` locally produces a report; a test uses `httptest.Server`
- [ ] Request bodies for routes with rules (generate from rules or `--body`), after M4's rule mapping; so POST/PUT don't just measure 422s

**Gate:** `riven-engine stress -o` on riven-app prints metrics and writes a versioned report; the request-body task is deferred until M4's rule mapping. ✅ passed 2026-11-13

## M4 — Docs (static + AI)
- [x] Static OpenAPI 3.1 builder from manifest (paths, params, request bodies from rules) (`engine/openapi`, `riven-engine docs`)
  - Done when: output passes an OpenAPI validator (official OAS 3.1 JSON Schema + every embedded schema compiled as 2020-12 + path-param/operationId checks, in `go test`)
- [x] Rule → JSON Schema mapping table (required, string, email, integer, min/max, in, array, nullable…) (`engine/rules`)
- [ ] (deferred: AI later, decided 2026-11-13) Claude API pass: fill descriptions/examples only where static data is missing; cache by route hash
  - Done when: static output alone is valid; AI only adds fields; re-run with no changes makes 0 API calls
- [ ] (deferred with the AI pass) Eval set: a few fixture routes with expected descriptions, scored, to compare prompts/models

## M5 — Dashboard
- [x] React app (Vite) built into `go:embed`, served by `riven-engine serve` (`engine/server`, `engine/web`)
  - Done when: API tests (incl. path traversal) pass; built UI is served from the binary for riven-app
- [x] Views: routes list, route detail (docs), stress reports (shadcn/ui)

**Gate:** `riven-engine serve` in riven-app shows routes, docs and saved stress reports. ✅ passed 2026-11-13
- [x] Endpoint explorer layout (user's mockups): top bar (search ⌘K, target, Rescan), grouped route sidebar with status dots, endpoint page with tabs: Docs & Try it, Stress test (run from the UI, live), History
  - Done when: Try it, Rescan and a UI-started stress run work against riven-app; server tests cover the guard (non-JSON, foreign Origin, rebinding Host) and one-run-at-a-time
- [ ] Later (needs Laravel-side instrumentation or AI): AI summary, Generated tests, Laravel insights (N+1, deadlocks, query count), fill from factory, DB transaction rollback, throttle bypass, ramp-up profiles
- [ ] Color pass (dark mode is the default for now)

## M6 — Ship
- [ ] (deferred with the AI work) AI test generator (Pest tests from manifest + docs), with an eval like M4
- [x] Cross-platform binaries (goreleaser), checksums (`.goreleaser.yaml`, `.github/workflows/release.yml`, CI in `ci.yml`)
  - Done when: all 6 targets cross-compile static with the version stamped, archives match the installer's naming, `checksums.txt` verifies, release binary serves the embedded UI (checked locally by hand; goreleaser itself first runs in CI)
- [x] Composer post-install downloads the matching binary (verify checksum) (`php artisan riven:install-engine`, `src/Engine/`; apps add it to `post-autoload-dump`)
  - Done when: tests cover naming per OS/arch, checksum mismatch/missing entry refused, old binary kept on failure, dev versions skipped, same version skipped; a real install from a local release mirror runs in riven-app
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
| 2026-11-13 | One shared sample manifest (`engine/testdata/riven-app.json`, a real export), validated against the schema by a Pest test; golden files for CLI output with a `-update` flag | Both halves test against the same file, so schema drift fails on the PHP side before Go tests silently pass on stale data; golden files make any output change show up as a reviewable diff |
| 2026-11-13 | Stress pool: N workers pull request numbers from an atomic counter (round-robin over targets), per-worker result slices merged at the end, every `Result` kept; client sized to `MaxIdleConnsPerHost = concurrency`, no redirects followed; duration stops issuing but lets in-flight requests finish, Ctrl-C drops cut-off requests | No locks/channels on the hot path; keeping raw results gives exact percentiles (memory is ~100 B/request, fine for runs up to millions; switch to a histogram if needed); Go's default 2 idle conns/host would measure TCP handshakes instead of the app |
| 2026-11-13 | Stress targeting: GET routes only by default, others only via `--route` (id or name, `*` wildcard); missing required params skip the route with a reason; `where()` patterns checked as a full match (RE2, PCRE-only patterns skipped); `Accept: application/json` by default; warnings for likely 401 (resolved `Authenticate` middleware, no credentials) and 422 (FormRequest with rules or unknown rules, no body yet) | Load-testing writes data, so it must be opt-in; a skipped route with a reason beats sending a literal `{user}`; JSON Accept makes Laravel return 401/422 JSON instead of redirects or HTML, so statuses mean what they say |
| 2026-11-13 | Metrics: nearest-rank percentiles on all samples (no interpolation); latency only for requests that got a response; failed = transport errors + 5xx (4xx counted, not failed); RPS from first start to last response end; transport errors grouped by kind | Nearest-rank always reports a real request, so p99 of a small run is the slowest one, not an invented value; timeouts would otherwise show up as latency = the timeout setting; 401/422 are often expected here (no token or body yet), so counting them as failures would hide real 5xx |
| 2026-11-13 | Stress report: `report_version` 1 + `kind`, durations as float ms in `*_ms` fields, `latency_ms: null` when nothing responded, `statuses`/`errors`/`routes` always objects/arrays (never null), `route_id` links to the manifest; Go structs + golden file are the contract until M5 adds a JSON Schema | Units in names avoid nanosecond/second mix-ups in JS; null vs 0 keeps "no data" from looking fast; stable shapes keep dashboard code free of null checks; a schema is cheap to add once the dashboard is the first consumer |
| 2026-11-13 | Rules → JSON Schema maps only certain rules (types, formats, size by type like Laravel's `getSize()`, in/not_in, regex without flags, nested `a.b` / `a.*.b`, `confirmed`); every property keeps `x-laravel-rules`, unmappable rules listed in `x-riven-unmapped-rules`; `date` stays a plain string | A schema that is too strict rejects valid requests, so uncertain rules are kept as data instead of guessed; the raw rules and the unmapped list are exactly what the AI pass needs |
| 2026-11-13 | OpenAPI built from generic maps (no typed model); optional params expand into separate paths; domain routes become path-level `servers` with variables; GET/DELETE rules become query params; POST+Resource answers 201; 401/422 documented from middleware/FormRequest; Resource schemas are open objects until shapes are analyzed | Maps serialise x- extensions exactly with no big dependency; OpenAPI path params must be required; each choice mirrors what Laravel does at runtime |
| 2026-11-13 | OpenAPI validation in `go test` with `santhosh-tekuri/jsonschema/v6` (test-only, not linked into the binary) against the committed official OAS 3.1 schema | Offline and deterministic; the OAS schema skips Schema Objects, so each one is also compiled as 2020-12; a mutation check confirmed it rejects bad documents |
| 2026-11-13 | Dashboard: `riven-engine serve` (127.0.0.1 by default) with a JSON API that re-reads files per request; reports opened via `os.Root`; React app in `engine/web`, built into `web/dist` (not committed) and embedded with `go:embed`; UI uses shadcn/ui (radix, Tailwind v4, default neutral theme, light mode) with its Sidebar | Local tool showing app internals, so local-only by default; `os.Root` blocks path traversal; embedding keeps one binary; shadcn gives accessible, consistent components we own as source |
| 2026-11-13 | Releases: GoReleaser on a `v*` tag builds linux/darwin/windows × amd64/arm64, `CGO_ENABLED=0`, version from the tag via ldflags, archives `riven-engine_<ver>_<os>_<arch>.tar.gz` (.zip on Windows), sha256 `checksums.txt`, draft release; one tag versions both halves | Static binaries run on any distro; a predictable name + checksum file is all the Composer installer needs; draft lets a human check assets before publishing; one version number means the installer can derive the binary version from the Composer package |
| 2026-11-13 | Package identity is `faysal/riven` (Composer name, Go module `github.com/faysal/riven/engine`, schema `$id`, manifest `generator.name`); releases are downloaded from github.com/faysal/riven | Owner's choice; Composer name, Go module and release URL must agree, so the GitHub repo has to live at faysal/riven (the current git remote is adnan4k/riven) |
| 2026-11-13 | Dashboard can act: `POST /api/try` (proxy, app base URL only), `POST /api/stress` + polling `GET /api/stress` (one run at a time, saved as a report), `POST /api/rescan` (runs `php artisan riven:export`); all POSTs require JSON, a same-origin Origin and a loopback Host | The browser can't call the app (CORS), so the engine proxies; JSON-only + Origin + Host checks stop other websites and DNS rebinding from driving a localhost tool that sends load and runs artisan |
| 2026-11-13 | Stress results get a per-second `timeline` (report v1, additive) and a thread-safe `stress.Live` recorder for live views | Latency-over-time charts for live and saved runs from the same data |
| 2026-11-13 | Dashboard on React 19, dark mode by default | shadcn components are written for React 19 (ref as a prop); on 18 refs were silently dropped |
| 2026-11-13 | Engine install = `riven:install-engine` artisan command run from the app's `post-autoload-dump` script, not a Composer plugin; downloads `riven-engine_<ver>_<os>_<arch>` + `checksums.txt` from the GitHub release matching the installed package version, verifies sha256 before unpacking, installs atomically to `vendor/bin/riven-engine`; dev versions are skipped with build instructions | Composer 2.2+ asks users to trust plugins and they run before the app exists; an artisan command is testable with `Http::fake`/`Process::fake`, rerunnable by hand, and a script hook is one line in the README; never installing an unverified file is the point of the checksum |
