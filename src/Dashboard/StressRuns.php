<?php

namespace Riven\Dashboard;

use Illuminate\Support\Facades\File;

/**
 * Stress runs started from the Laravel-hosted dashboard.
 *
 * A run outlives any PHP request, so it runs as a background
 * `riven-engine stress` process that keeps its live state in live.json
 * (same shape as engine/server's GET /api/stress) and stops when the stop
 * file appears. One run at a time, like `riven-engine serve`.
 */
class StressRuns
{
    /** A running state not refreshed for this long means the engine died. */
    public const STALE_SECONDS = 10;

    public function __construct(
        private Engine $engine,
        private string $storage,
        private Reports $reports,
    ) {}

    /**
     * @return array<string, mixed>
     */
    public function state(): array
    {
        $live = $this->path('live.json');
        $state = is_file($live) ? json_decode((string) file_get_contents($live), true) : null;
        if (! is_array($state)) {
            return ['state' => 'idle', 'elapsed_ms' => 0, 'duration_ms' => 0, 'interrupted' => false];
        }

        clearstatcache(true, $live);
        if ($state['state'] === 'running' && time() - filemtime($live) > self::STALE_SECONDS) {
            $state['state'] = 'failed';
            $state['error'] = 'riven-engine stopped without finishing the run. '.$this->logTail();
        }

        return $state;
    }

    /**
     * @param  array<string, mixed>  $manifest
     * @param  array{base_url: string, routes: list<string>, params: array<string, string>, token: string, concurrency: int, duration_s: int, requests: int}  $run
     * @param  array<string, list<mixed>>  $bodies  generated bodies by route id
     * @return array<string, mixed>
     */
    public function start(array $manifest, array $run, array $bodies = []): array
    {
        if ($this->state()['state'] === 'running') {
            throw new DashboardException('A stress run is already in progress.', 409);
        }
        if (! $this->engine->exists()) {
            throw new DashboardException("riven-engine isn't installed at {$this->engine->path()}. Run composer install (or php artisan riven:install-engine).");
        }

        File::ensureDirectoryExists($this->storage);
        File::ensureDirectoryExists($this->reports->directory());
        @unlink($this->path('stop'));

        $started = now()->utc();
        $args = [
            'stress',
            '--manifest', $this->engine->tempManifest($manifest, $this->path('manifest.json')),
            '--base-url', $run['base_url'],
            '--concurrency', (string) $run['concurrency'],
            '--output', $this->reports->directory().'/stress-'.$started->format('Ymd-His').'.json',
            '--live-file', $this->path('live.json'),
            '--stop-file', $this->path('stop'),
        ];
        foreach ($run['routes'] as $route) {
            array_push($args, '--route', $route);
        }
        foreach ($run['params'] as $name => $value) {
            array_push($args, '--param', "{$name}={$value}");
        }
        if ($run['token'] !== '') {
            array_push($args, '--token', $run['token']);
        }
        if ($run['duration_s'] > 0) {
            array_push($args, '--duration', $run['duration_s'].'s');
        }
        if ($run['requests'] > 0) {
            array_push($args, '--requests', (string) $run['requests']);
        }
        if ($bodies !== []) {
            file_put_contents($this->path('bodies.json'), json_encode($bodies, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE | JSON_THROW_ON_ERROR));
            array_push($args, '--bodies-file', $this->path('bodies.json'));
        }

        // Shown until the engine writes its first state (within ~1s).
        $state = [
            'state' => 'running', 'id' => (string) $started->getTimestampMs(), 'elapsed_ms' => 0,
            'duration_ms' => $run['duration_s'] * 1000, 'interrupted' => false,
        ];
        file_put_contents($this->path('live.json'), json_encode($state));

        $this->engine->startInBackground($args, $this->path('engine.log'));

        return $state;
    }

    public function stop(): void
    {
        if ($this->state()['state'] !== 'running') {
            throw new DashboardException('No stress run in progress.', 409);
        }
        touch($this->path('stop'));
    }

    private function path(string $name): string
    {
        return $this->storage.'/'.$name;
    }

    private function logTail(): string
    {
        $log = $this->path('engine.log');
        $tail = is_file($log) ? trim(substr((string) file_get_contents($log), -1000)) : '';

        return $tail === '' ? '' : "Its output: {$tail}";
    }
}
