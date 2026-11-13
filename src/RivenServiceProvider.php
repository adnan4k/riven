<?php

namespace Riven;

use Illuminate\Contracts\Http\Kernel as HttpKernel;
use Illuminate\Support\ServiceProvider;
use Riven\Commands\ExportManifestCommand;
use Riven\Commands\ScanRoutesCommand;
use Riven\Scanner\ActionReflector;
use Riven\Scanner\ResourceDetector;
use Riven\Scanner\RouteScanner;
use Riven\Scanner\RulesExtractor;

class RivenServiceProvider extends ServiceProvider
{
    private const CONFIG = __DIR__.'/../config/riven.php';

    public function register(): void
    {
        // Package defaults; a published config/riven.php in the app overrides them key by key.
        $this->mergeConfigFrom(self::CONFIG, 'riven');

        $this->app->singleton(RouteScanner::class, function ($app) {
            // Middleware groups and aliases live on the HTTP kernel, which copies them
            // into the router in its constructor. Artisan only boots the console kernel,
            // so build the HTTP one or groups like "web" can't be resolved.
            $app->make(HttpKernel::class);

            $config = $app['config']['riven'];

            return new RouteScanner(
                router: $app['router'],
                excludedUris: $config['exclude'],
                vendorPath: $config['exclude_vendor'] ? $app->basePath('vendor') : null,
                actions: $app->make(ActionReflector::class),
                rules: $app->make(RulesExtractor::class),
                resources: $app->make(ResourceDetector::class),
                includedUris: $config['include'],
            );
        });
    }

    public function boot(): void
    {
        if ($this->app->runningInConsole()) {
            $this->publishes([self::CONFIG => config_path('riven.php')], 'riven-config');

            $this->commands([
                ScanRoutesCommand::class,
                ExportManifestCommand::class,
            ]);
        }
    }
}
