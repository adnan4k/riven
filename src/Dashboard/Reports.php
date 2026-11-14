<?php

namespace Riven\Dashboard;

/**
 * Saved stress reports (riven-reports/*.json), the same files
 * `riven-engine serve` reads. Mirrors engine/server's /api/reports.
 */
class Reports
{
    public function __construct(private string $dir) {}

    public function directory(): string
    {
        return $this->dir;
    }

    /**
     * @return list<array<string, mixed>> newest first
     */
    public function list(): array
    {
        $list = [];
        foreach (glob($this->dir.'/*.json') ?: [] as $file) {
            $report = $this->read($file);
            if ($report === null) {
                continue; // not a stress report: skip it
            }
            $list[] = [
                'name' => basename($file),
                'started_at' => $report['started_at'],
                'interrupted' => $report['interrupted'] ?? false,
                'config' => $report['config'],
                'summary' => $report['summary'],
                'routes' => count($report['routes'] ?? []),
            ];
        }
        usort($list, fn ($a, $b) => strcmp($b['started_at'], $a['started_at']));

        return $list;
    }

    /**
     * @return ?array<string, mixed>
     */
    public function find(string $name): ?array
    {
        // A bare file name only: no directories, so nothing outside reports/.
        if (basename($name) !== $name || ! str_ends_with($name, '.json')) {
            return null;
        }

        return $this->read($this->dir.'/'.$name);
    }

    /**
     * @return ?array<string, mixed>
     */
    private function read(string $file): ?array
    {
        if (! is_file($file)) {
            return null;
        }
        $report = json_decode((string) file_get_contents($file), true);

        return is_array($report) && ($report['kind'] ?? null) === 'stress' && ($report['report_version'] ?? null) === 1
            ? $report
            : null;
    }
}
