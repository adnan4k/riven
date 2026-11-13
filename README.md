<p align="center">
  <img src="https://img.shields.io/packagist/v/faysal/riven?style=flat-square" alt="Packagist Version">
  <img src="https://img.shields.io/packagist/l/faysal/riven?style=flat-square" alt="License">
  <img src="https://img.shields.io/packagist/php-v/faysal/riven?style=flat-square" alt="PHP Version">
  <img src="https://img.shields.io/badge/Laravel-11%20%7C%2012%20%7C%2013-FF2D20?style=flat-square" alt="Laravel">
</p>

# Riven

**The all-in-one Laravel developer toolkit.** Riven scans your application's routes, extracts validation rules, detects API resources, generates OpenAPI documentation, stress-tests endpoints, and serves an interactive dashboard — all from a single `composer require`.

```
composer require faysal/riven --dev
```

---

## What It Does

| Feature | Description |
|---------|-------------|
| **Route Scanning** | Discovers every route, resolves middleware (aliases → classes), detects parameters, `where` patterns, and action types |
| **Request Extraction** | Finds the `FormRequest` on each action via reflection and pulls out validation rules |
| **Resource Detection** | Statically detects `JsonResource` / `ResourceCollection` return types from controller bodies using `nikic/php-parser` |
| **JSON Manifest** | Exports a strict, schema-validated `riven.json` — the single contract between the PHP collector and the Go engine |
| **OpenAPI Generation** | Builds a full OpenAPI 3.1 spec from the manifest — paths, parameters, request bodies from Laravel rules, response schemas |
| **AI-Enhanced Docs** | Optionally fills in descriptions and examples using Anthropic or local Ollama models — only where static data is missing |
| **Stress Testing** | Concurrent load testing with per-route targeting, param substitution, auth injection, and percentile latency reports |
| **Interactive Dashboard** | A React UI served by the engine — browse routes, read docs, and review stress-test reports in the browser |
| **Test Generation** | Generates Pest test stubs from the manifest and docs so your API coverage matches your routes |

---

## Quick Start

### 1. Install the package

```bash
composer require faysal/riven --dev
```

The service provider auto-registers via Laravel's package discovery.

### 2. Publish the config (optional)

```bash
php artisan vendor:publish --tag=riven-config
```

This creates `config/riven.php` where you can set include/exclude URI patterns and the manifest output path.

### 3. Scan & export

```bash
# Preview routes in the terminal
php artisan riven:scan

# Export the JSON manifest
php artisan riven:export
```

`riven:export` writes `riven.json` (configurable) validated against the [schema](schema/riven.schema.json).

### 4. Use the engine

The Go engine reads the manifest for everything else:

```bash
# Print routes from the manifest
riven-engine routes

# Generate OpenAPI 3.1 spec
riven-engine docs --format openapi

# Run stress tests
riven-engine stress --concurrency 50 --duration 30s --auth-header "Bearer $TOKEN"

# Launch the dashboard
riven-engine serve --port 8080
```

---

## Architecture

Riven is a monorepo with two halves connected by a strict JSON contract:

```
┌──────────────────────────┐       riven.json        ┌──────────────────────────┐
│       PHP Collector      │ ──────────────────────▶  │       Go Engine          │
│                          │   (schema_version: 1)    │                          │
│  • RouteScanner          │                          │  • Routes viewer         │
│  • ActionReflector       │                          │  • OpenAPI builder       │
│  • RulesExtractor        │                          │  • AI doc enhancement    │
│  • ResourceDetector      │                          │  • Stress tester         │
│  • ManifestBuilder       │                          │  • Dashboard (React)     │
│                          │                          │  • Test generator        │
└──────────────────────────┘                          └──────────────────────────┘
        Laravel app                                         Single binary
```

**Why two languages?** The collector needs a booted Laravel application to reflect routes, middleware, FormRequests, and resources. Everything else — docs, stress testing, the dashboard — is faster and more portable as a single, cross-platform Go binary.

---

## Configuration

```php
// config/riven.php

return [
    // Only scan routes matching these URI patterns (empty = all)
    'include' => [],

    // Always skip these URIs (takes precedence over include)
    'exclude' => [
        'up',
        '_ignition/*',
        'sanctum/*',
        'storage/*',
    ],

    // Skip routes whose action is defined in vendor/
    'exclude_vendor' => true,

    // Where riven:export writes the manifest (relative to base_path)
    'output' => 'riven.json',
];
```

Patterns use `Str::is()` — `*` is a wildcard, so `api/*` matches all API routes.

---

## Manifest Contract

The manifest (`riven.json`) is the single source of truth between the PHP collector and the Go engine. Its schema is frozen as v1 and strictly validated — unknown fields are rejected.

<details>
<summary>Example manifest</summary>

```json
{
  "schema_version": 1,
  "generator": { "name": "faysal/riven", "version": "0.1.0" },
  "generated_at": "2026-11-13T12:00:00Z",
  "app": {
    "name": "my-app",
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
      "middleware_resolved": [
        "Illuminate\\Routing\\Middleware\\SubstituteBindings",
        "Illuminate\\Auth\\Middleware\\Authenticate:sanctum"
      ],
      "parameters": [
        { "name": "user", "required": true, "pattern": null }
      ],
      "request": {
        "form_request": "App\\Http\\Requests\\StoreUserRequest",
        "rules": {
          "email": ["required", "email"],
          "name": ["required", "string", "max:255"]
        }
      },
      "response": {
        "resource": "App\\Http\\Resources\\UserResource",
        "collection": false
      }
    }
  ],
  "warnings": [
    { "route_id": "POST /api/users", "code": "rules_need_request", "message": "..." }
  ]
}
```

</details>

### Key rules

- **`id`** — `"<first non-HEAD method> /<uri>"`, one entry per Laravel route
- **`action.type`** — `controller` | `invokable` | `closure` (class/method are `null` for closures)
- **`middleware`** — as declared; **`middleware_resolved`** — fully expanded class names
- **`request.rules`** — `null` when no FormRequest exists or `rules()` needs a live HTTP request (triggers a `rules_need_request` warning)
- **`response`** — `null` when unknown; Riven never guesses, only detects explicit patterns
- **`warnings`** — non-fatal issues surface here, never as crashes

Schema versioning: additive changes are minor bumps; renames or removals bump `schema_version`.

---

## Engine Commands

| Command | Description |
|---------|-------------|
| `riven-engine routes` | Pretty-print routes from the manifest |
| `riven-engine docs` | Generate OpenAPI 3.1 spec. `--ai=ollama\|anthropic` for AI-enhanced descriptions |
| `riven-engine stress` | Load test endpoints. `--concurrency`, `--duration`, `--count`, `--auth-header` |
| `riven-engine serve` | Launch the dashboard UI on `--port` (default 8080) |
| `riven-engine test-gen` | Generate Pest test stubs from manifest + docs |
| `riven-engine --version` | Print the engine version |

### AI Documentation

By default, Riven generates docs from static analysis alone (always deterministic). Optionally enable AI to fill in descriptions and examples:

```bash
# Free & private — local Ollama
riven-engine docs --ai=ollama

# Anthropic Claude
ANTHROPIC_API_KEY=sk-... riven-engine docs --ai=anthropic
```

AI results are cached by a hash of the route's source, so re-runs with no code changes make zero API calls.

### Stress Testing

```bash
# 50 concurrent workers, 30 seconds
riven-engine stress --concurrency 50 --duration 30s

# Target specific routes
riven-engine stress --route "GET /api/users" --route "POST /api/users"

# With authentication
riven-engine stress --auth-header "Bearer $TOKEN"
```

Reports include p50/p95/p99 latency, RPS, error rates, and status code histograms. Results export as JSON for the dashboard.

---

## Installation — Engine Binary

The Go engine binary is downloaded automatically via Composer's `post-install` hook. The binary matches your platform and is verified with a SHA-256 checksum.

To install manually:

```bash
# macOS (Apple Silicon)
curl -fsSL https://github.com/faysal/riven/releases/latest/download/riven-engine-darwin-arm64.tar.gz | tar xz

# Linux (x86_64)
curl -fsSL https://github.com/faysal/riven/releases/latest/download/riven-engine-linux-amd64.tar.gz | tar xz

# Move to PATH
mv riven-engine /usr/local/bin/
```

---

## Requirements

- **PHP** ≥ 8.2
- **Laravel** 11, 12, or 13
- **Go** 1.22+ (only if building the engine from source)

---

## Development

```bash
# Clone the repo
git clone https://github.com/faysal/riven.git
cd riven

# PHP — install deps & run tests
composer install
vendor/bin/pest

# Go engine — build & test
cd engine
go build ./cmd/riven-engine
go test ./...
```

The repo uses `.gitattributes` to keep Packagist dist archives PHP-only — the engine source, tests, and dev files are excluded. End users get the prebuilt binary via Composer's post-install hook.

---

## License

Riven is open-source software licensed under the [MIT License](LICENSE).
