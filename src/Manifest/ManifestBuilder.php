<?php

namespace Riven\Manifest;

use Composer\InstalledVersions;
use Illuminate\Contracts\Foundation\Application;
use Illuminate\Support\Facades\Date;
use Riven\Scanner\RouteScanner;

/**
 * Wraps a scan in the manifest envelope (see schema/riven.schema.json).
 */
class ManifestBuilder
{
    public const SCHEMA_VERSION = 1;

    public const GENERATOR = 'faysal/riven';

    public function __construct(
        private Application $app,
        private RouteScanner $scanner,
    ) {}

    /**
     * @return array<string, mixed>
     */
    public function build(): array
    {
        ['routes' => $routes, 'warnings' => $warnings] = $this->scanner->scan();

        return [
            'schema_version' => self::SCHEMA_VERSION,
            'generator' => ['name' => self::GENERATOR, 'version' => $this->generatorVersion()],
            'generated_at' => Date::now()->utc()->format('Y-m-d\TH:i:s\Z'),
            'app' => [
                'name' => $this->app['config']->get('app.name'),
                'url' => $this->app['config']->get('app.url'),
                'laravel_version' => $this->app->version(),
                'php_version' => PHP_VERSION,
            ],
            'routes' => array_map($this->forJson(...), $routes),
            'warnings' => $warnings,
        ];
    }

    /**
     * PHP encodes an empty array as [], but the contract says rules is an object
     * (field => rules), so a FormRequest with no rules must encode as {}.
     */
    private function forJson(array $route): array
    {
        if (is_array($route['request']['rules'])) {
            $route['request']['rules'] = (object) $route['request']['rules'];
        }

        return $route;
    }

    private function generatorVersion(): ?string
    {
        return InstalledVersions::isInstalled(self::GENERATOR)
            ? InstalledVersions::getPrettyVersion(self::GENERATOR)
            : null;
    }
}
