<?php

namespace Riven\Commands;

use Illuminate\Console\Command;
use Illuminate\Filesystem\Filesystem;
use Riven\Manifest\ManifestBuilder;

class ExportManifestCommand extends Command
{
    protected $signature = 'riven:export
        {--output= : Where to write the manifest (default: config riven.output)}
        {--compact : Write minified JSON instead of pretty-printed}';

    protected $description = 'Write the riven.json manifest for the Riven engine';

    public function handle(ManifestBuilder $builder, Filesystem $files): int
    {
        $manifest = $builder->build();

        $flags = JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE | JSON_THROW_ON_ERROR;
        $json = json_encode($manifest, $this->option('compact') ? $flags : $flags | JSON_PRETTY_PRINT);

        $path = $this->outputPath();
        $files->ensureDirectoryExists(dirname($path));
        $files->put($path, $json.PHP_EOL);

        foreach ($manifest['warnings'] as $warning) {
            $this->warn("{$warning['route_id']}: {$warning['message']}");
        }

        $this->components->info(sprintf(
            'Wrote %s (%d %s, %d %s).',
            $path,
            count($manifest['routes']), str('route')->plural(count($manifest['routes'])),
            count($manifest['warnings']), str('warning')->plural(count($manifest['warnings'])),
        ));

        return self::SUCCESS;
    }

    /**
     * Relative paths are resolved from the app's base path, not the shell's cwd.
     */
    private function outputPath(): string
    {
        $path = $this->option('output') ?: config('riven.output');

        return str_starts_with($path, DIRECTORY_SEPARATOR) ? $path : $this->laravel->basePath($path);
    }
}
