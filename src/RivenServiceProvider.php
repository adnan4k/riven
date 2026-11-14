<?php

namespace Riven;

use Illuminate\Contracts\Http\Kernel as HttpKernel;
use Illuminate\Support\ServiceProvider;
use Riven\Commands\ExportManifestCommand;
use Riven\Commands\FakeBodyCommand;
use Riven\Commands\InstallEngineCommand;
use Riven\Commands\ScanRoutesCommand;
use Riven\Engine\EngineInstaller;
use Riven\Engine\LaravelHttpClient;
use Riven\Faking\BodyFaker;
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

        // Faker\Generator is bound by Laravel (respects app.faker_locale).
        $this->app->bind(BodyFaker::class, fn ($app) => new BodyFaker($app->make(\Faker\Generator::class)));

        $this->app->bind(EngineInstaller::class, fn ($app) => new EngineInstaller(
            target: $app['config']['riven.engine.path'] ?? $app->basePath('vendor/bin/riven-engine'.(PHP_OS_FAMILY === 'Windows' ? '.exe' : '')),
            http: new LaravelHttpClient,
            releasesUrl: $app['config']['riven.engine.releases_url'] ?? EngineInstaller::RELEASES_URL,
        ));
    }

    public function boot(): void
    {
        if ($this->app->runningInConsole()) {
            $this->publishes([self::CONFIG => config_path('riven.php')], 'riven-config');

            $this->commands([
                ScanRoutesCommand::class,
                ExportManifestCommand::class,
                InstallEngineCommand::class,
                FakeBodyCommand::class,
            ]);
        }
    }
}
