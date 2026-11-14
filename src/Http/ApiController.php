<?php

namespace Riven\Http;

use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;
use Riven\Dashboard\DashboardException;
use Riven\Dashboard\Engine;
use Riven\Dashboard\Reports;
use Riven\Dashboard\StressRuns;
use Riven\Dashboard\TryProxy;
use Riven\Faking\RouteBodies;
use Riven\Manifest\ManifestBuilder;
use RuntimeException;

/**
 * The dashboard's JSON API when Laravel hosts it: the same endpoints as
 * engine/server, answered in the app. The manifest is built on every
 * request, so it's always current: no riven.json, watcher or rescan needed.
 * Errors are {"error": "..."}, which the UI shows.
 */
class ApiController
{
    public function __construct(private ManifestBuilder $manifest) {}

    public function config(): JsonResponse
    {
        return $this->json([
            'base_url' => (string) config('app.url'),
            'can_rescan' => true,       // Rescan just reloads: the manifest is always fresh
            'live_updates' => false,    // no /api/events: a PHP worker can't hold a stream open cheaply
        ]);
    }

    public function manifest(): JsonResponse
    {
        return $this->json($this->manifest->build());
    }

    public function openapi(Engine $engine): JsonResponse
    {
        return $this->attempt(fn () => $engine->openapi($this->manifest->build()));
    }

    public function reports(Reports $reports): JsonResponse
    {
        return $this->json($reports->list());
    }

    public function report(Reports $reports, string $name): JsonResponse
    {
        $report = $reports->find($name);

        return $report === null ? $this->error("No stress report named {$name}.", 404) : $this->json($report);
    }

    public function rescan(): JsonResponse
    {
        return $this->json(['output' => 'Routes are read live on every page load.']);
    }

    public function try(Request $request, TryProxy $proxy): JsonResponse
    {
        return $this->attempt(fn () => $proxy->send(
            $this->target($request),
            (string) $request->input('method'),
            (string) $request->input('path'),
            array_map('strval', (array) $request->input('headers', [])),
            (string) $request->input('body', ''),
            $request->getHttpHost(),
        ));
    }

    public function fake(Request $request, RouteBodies $bodies): JsonResponse
    {
        return $this->attempt(fn () => $bodies->generate((string) $request->input('route_id'), (int) $request->input('count', 1)), 422);
    }

    public function stressState(StressRuns $runs): JsonResponse
    {
        return $this->json($runs->state());
    }

    public function startStress(Request $request, StressRuns $runs, RouteBodies $bodies): JsonResponse
    {
        return $this->attempt(function () use ($request, $runs, $bodies) {
            $routes = array_values(array_map('strval', (array) $request->input('routes', [])));
            $concurrency = (int) $request->input('concurrency');
            $duration = (int) $request->input('duration_s');
            $requests = (int) $request->input('requests');
            $fake = (int) $request->input('fake_bodies');

            if ($routes === []) {
                throw new DashboardException('pick at least one route', 400);
            }
            if ($concurrency < 1 || $concurrency > 1000) {
                throw new DashboardException('virtual users must be between 1 and 1000', 400);
            }
            if ($duration < 0 || $duration > 3600 || $requests < 0 || ($duration === 0 && $requests === 0)) {
                throw new DashboardException('set a duration (up to 1h) or a request count', 400);
            }
            if ($fake < 0 || $fake > RouteBodies::MAX) {
                throw new DashboardException('generated bodies must be between 0 and '.RouteBodies::MAX, 400);
            }

            $manifest = $this->manifest->build();
            $generated = [];
            if ($fake > 0) {
                foreach ($manifest['routes'] as $route) {
                    $writes = array_diff($route['methods'], ['GET', 'HEAD', 'DELETE']) !== [];
                    if (in_array($route['id'], $routes, true) && $writes && $route['request']['form_request'] !== null) {
                        try {
                            $generated[$route['id']] = $bodies->generate($route['id'], $fake)['bodies'];
                        } catch (RuntimeException $e) {
                            throw new DashboardException("test data for {$route['id']}: {$e->getMessage()}", 422);
                        }
                    }
                }
            }

            return $runs->start($manifest, [
                'base_url' => $this->target($request),
                'routes' => $routes,
                'params' => array_map('strval', (array) $request->input('params', [])),
                'token' => (string) $request->input('token', ''),
                'concurrency' => $concurrency,
                'duration_s' => $duration,
                'requests' => $requests,
            ], $generated);
        });
    }

    public function stopStress(StressRuns $runs): JsonResponse
    {
        return $this->attempt(function () use ($runs) {
            $runs->stop();

            return ['stopping' => true];
        });
    }

    public function notFound(): JsonResponse
    {
        return $this->error('no such API endpoint', 404);
    }

    /**
     * The target chosen in the dashboard, else APP_URL.
     */
    private function target(Request $request): string
    {
        $chosen = (string) $request->input('base_url', '');
        if ($chosen === '') {
            $chosen = (string) config('app.url');
        }
        $parts = parse_url($chosen);
        if (! in_array($parts['scheme'] ?? null, ['http', 'https'], true) || empty($parts['host'])) {
            throw new DashboardException("target \"{$chosen}\" must be an absolute http:// or https:// URL", 400);
        }

        return $chosen;
    }

    private function attempt(callable $fn, int $status = 500): JsonResponse
    {
        try {
            return $this->json($fn());
        } catch (DashboardException $e) {
            return $this->error($e->getMessage(), $e->getCode() ?: $status);
        } catch (RuntimeException $e) {
            return $this->error($e->getMessage(), $status);
        }
    }

    private function json(mixed $data): JsonResponse
    {
        return response()->json($data, 200, ['Cache-Control' => 'no-store'], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
    }

    private function error(string $message, int $status): JsonResponse
    {
        return response()->json(['error' => $message], $status);
    }
}
