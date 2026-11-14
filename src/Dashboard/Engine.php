<?php

namespace Riven\Dashboard;

use Illuminate\Support\Facades\Process;

/**
 * The riven-engine binary, as the dashboard uses it: OpenAPI building,
 * the UI files, and stress runs started in the background.
 */
class Engine
{
    public function __construct(private string $binary) {}

    public function path(): string
    {
        return $this->binary;
    }

    public function exists(): bool
    {
        return is_file($this->binary) && is_executable($this->binary);
    }

    /**
     * Changes whenever the binary does, so cached UI files follow upgrades.
     */
    public function fingerprint(): string
    {
        clearstatcache(true, $this->binary);

        return substr(md5($this->binary.'|'.filesize($this->binary).'|'.filemtime($this->binary)), 0, 12);
    }

    /**
     * @param  array<string, mixed>  $manifest
     * @return array<string, mixed>
     */
    public function openapi(array $manifest): array
    {
        $file = $this->tempManifest($manifest);
        try {
            $result = $this->run(['docs', '--manifest', $file]);
        } finally {
            @unlink($file);
        }

        return json_decode($result, true, flags: JSON_THROW_ON_ERROR);
    }

    public function extractUi(string $dir): void
    {
        $this->run(['ui', $dir]);
    }

    /**
     * Starts the engine detached from this request: it keeps running after
     * the response is sent (a stress run lasts longer than any PHP request).
     *
     * @param  list<string>  $args
     */
    public function startInBackground(array $args, string $log): void
    {
        $command = implode(' ', array_map('escapeshellarg', [$this->binary, ...$args]));

        if (PHP_OS_FAMILY === 'Windows') {
            pclose(popen('start "" /B '.$command.' > '.escapeshellarg($log).' 2>&1', 'r'));

            return;
        }

        // nohup: survive the PHP worker; output to the log so exec() returns at once.
        exec('nohup '.$command.' > '.escapeshellarg($log).' 2>&1 &');
    }

    /**
     * @param  list<string>  $args
     */
    private function run(array $args): string
    {
        if (! $this->exists()) {
            throw new DashboardException("riven-engine isn't installed at {$this->binary}. Run composer install (or php artisan riven:install-engine).");
        }

        $result = Process::timeout(60)->run([$this->binary, ...$args]);
        if ($result->failed()) {
            throw new DashboardException('riven-engine '.$args[0].' failed: '.trim($result->errorOutput() ?: $result->output()));
        }

        return $result->output();
    }

    /**
     * @param  array<string, mixed>  $manifest
     */
    public function tempManifest(array $manifest, ?string $path = null): string
    {
        $path ??= tempnam(sys_get_temp_dir(), 'riven-manifest-');
        file_put_contents($path, json_encode($manifest, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE | JSON_THROW_ON_ERROR));

        return $path;
    }
}
