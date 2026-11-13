# Riven Plan

Status key: `[ ]` todo · `[~]` in progress · `[x]` done (reviewed).
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
- [ ] `riven:scan` (`src/Commands/ScanRoutesCommand.php`) prints routes as a table
  - Done when: output shows uri, methods, name, action for `riven-app` routes
- [ ] RouteScanner filtering: exclude framework/vendor routes (e.g. `_ignition`, `sanctum/*`, `storage/*`, closures from vendor providers); configurable
  - Done when: a test registers app + vendor-like routes and only app routes come back
- [ ] RouteScanner enrichment: middleware (resolved, not aliases only), parameter names + optional flag + `where` patterns, domain, action type (controller / invokable / closure)
  - Done when: tests cover each field on a fixture route
- [ ] Controller reflection: resolve the FormRequest type-hinted on the action
  - Done when: a fixture controller method with `StoreUserRequest $request` yields that class name
- [ ] FormRequest rules extraction (`rules()` output, string and array rule formats normalized)
  - Done when: rules appear in manifest shape; rules needing a live request are skipped with a `warnings[]` entry, not a crash
- [ ] Resource detection: return type / `new XResource` / `XResource::collection` (best effort, static)
  - Done when: fixture returns are detected; unknown → `null`, not a guess
- [ ] Config file `config/riven.php` (publishable): include/exclude patterns, output path
  - Done when: `vendor:publish --tag=riven-config` works and options are honored
- [ ] `riven:export` writes `riven.json` matching the Manifest contract below
  - Done when: output validates against `schema/riven.schema.json` in a test

**Gate:** `riven:export` on `riven-app` produces a valid manifest; schema frozen as v1.

## M2 — Go engine skeleton (`riven-engine` repo)
- [ ] Repo, Go module, CLI framework choice (cobra vs stdlib `flag`), decide with reasons
- [ ] Manifest structs + loader with `schema_version` check
  - Done when: loads the M1 output; rejects an unknown major version with a clear error
- [ ] `riven-engine routes` prints routes
  - Done when: output matches `riven:scan` for the same app
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

## Manifest contract (draft, schema_version 1)

Rules: additive changes = minor; renames/removals = bump `schema_version`.
Unknown values are `null`, never guessed. Non-fatal problems go in `warnings`.

```json
{
  "schema_version": 1,
  "generator": { "name": "faysal/riven", "version": "0.1.0" },
  "generated_at": "2026-11-13T12:00:00Z",
  "app": {
    "name": "riven-app",
    "url": "http://localhost:8000",
    "laravel_version": "13.x",
    "php_version": "8.4.x"
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

Open questions (decide before freezing v1):
- `id` format: `METHOD uri` per method, or one entry per route with multiple methods?
- Rules as raw Laravel strings (engine maps them) vs pre-normalized objects?
- Include Resource field shapes now, or leave for a later schema version?

---

## Decision log
| Date | Decision | Why |
|------|----------|-----|
| 2026-11-13 | PHP collects, Go processes, JSON manifest between them | Collector needs the booted app; everything else is faster/more portable in a single binary |
