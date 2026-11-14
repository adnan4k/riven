<p align="center">
  <img src="https://img.shields.io/packagist/v/faysal/riven?style=flat-square" alt="Packagist Version">
  <img src="https://img.shields.io/packagist/l/faysal/riven?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/Laravel-11%20%7C%2012%20%7C%2013-FF2D20?style=flat-square" alt="Laravel 11, 12, 13">
</p>

# Riven

**API docs, a request playground and load tests for your Laravel app, generated from the code you already have.**

Riven reads your routes, FormRequest rules and API Resources, then gives you a local dashboard to browse every endpoint, send real requests with generated test data, and stress test them, without writing a spec by hand.

![Riven dashboard: docs and Try it for POST /api/users, with a generated body and the 201 response](docs/images/dashboard-docs.png)

## How it works

1. **The package** (`faysal/riven`) runs inside your app. `php artisan riven:export` writes `riven.json`, which lists every route with its middleware, parameters, validation rules and response Resource.
2. **The engine** (`riven-engine`, one Go binary) reads that file. It prints the routes, builds an OpenAPI 3.1 document, runs load tests, and serves the dashboard.
3. **The dashboard** sends Try it and stress requests to your app, and updates by itself when you add or change a route.

## Install

```bash
composer require faysal/riven --dev
```

That's the only setup step. Riven is a Composer plugin: when Composer installs or updates it, it also downloads the `riven-engine` binary for your platform. The first time, Composer asks whether you trust `faysal/riven` to run code; answer `y` (it saves that under `allow-plugins` in your `composer.json`).

The binary comes from this repository's GitHub release for your installed version and is checked against its SHA-256 checksum before it's installed to `vendor/bin/riven-engine`. If the download fails (offline, a platform without a build), Composer still finishes and prints a warning; run `php artisan riven:install-engine` to retry.

## Use it

Open **`APP_URL/riven`** (for example http://localhost:8000/riven) while your app runs. Nothing else to start: your app serves the dashboard and reads your routes live on every page load. It's only there when `APP_ENV=local`; set `RIVEN_DASHBOARD=true` or `false` to override, and `RIVEN_PATH` to mount it elsewhere.

**Target** in the top bar is where requests go. It starts as `APP_URL`; click it to send requests elsewhere (another port, a staging URL, a Docker service name like `http://nginx`). The choice stays in your browser, and the reset arrow goes back to `APP_URL`. When the target is the dashboard's own address, your browser sends Try it requests itself, with your session cookie and XSRF token, so `web` routes work while you're logged in.

> On `php artisan serve` with another target on the same app, start it with several workers, or the app can't answer a request while it's busy with another: `PHP_CLI_SERVER_WORKERS=4 php artisan serve --no-reload`.

Prefer a standalone dashboard (CI box, no app running)? `vendor/bin/riven-engine serve` serves it on http://127.0.0.1:7600 from `riven.json` and re-runs `riven:export` whenever your routes or controllers change.

On each endpoint:

- **Docs & Try it**: body fields and rules, responses, and a request panel. **Generate test data** fills the body with values that pass your rules, using real rows for `exists:` and new values for `unique:`.
- **Stress test**: pick users and a duration, watch latency and errors live, and fail the run when p95 goes over a limit. Every run is saved to History.

![Stress test results for GET /api/users: 122 req/s, p95 36 ms, 0% errors, latency over time](docs/images/dashboard-stress.png)

> Stress tests on POST, PUT and DELETE routes write to your database. Point them at a local or throwaway database.

## Commands

| Command | What it does |
|---|---|
| `php artisan riven:scan` | Print the route table |
| `php artisan riven:export` | Write `riven.json` |
| `php artisan riven:fake-body "POST /api/users"` | Print request bodies that pass the route's rules |
| `php artisan riven:install-engine` | Download and verify `riven-engine` again (Composer does this for you) |
| `riven-engine serve` | Open the dashboard (`--base-url`, `--watch=false`) |
| `riven-engine docs -o openapi.json` | Write an OpenAPI 3.1 document |
| `riven-engine stress -d 30s -c 20 --save` | Load test GET routes from the terminal (`--route`, `--param`, `--token`) |
| `riven-engine routes` | Print the route table from `riven.json` |

## Configuration

`php artisan vendor:publish --tag=riven-config` creates `config/riven.php`, where you can `include` or `exclude` routes by URI pattern (`api/*`) and change where `riven.json` is written.

## Requirements

PHP 8.2+ and Laravel 11, 12 or 13. Prebuilt engine binaries cover Linux, macOS and Windows on amd64 and arm64.

## Development

```bash
composer install && vendor/bin/pest              # PHP package
cd engine/web && npm ci && npm run build         # dashboard (embedded in the binary)
cd .. && go test ./... && go build ./cmd/riven-engine
```

The manifest format is defined in [`schema/riven.schema.json`](schema/riven.schema.json).

## License

MIT. See [LICENSE](LICENSE).
