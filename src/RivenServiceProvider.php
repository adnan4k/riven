<?php

namespace Riven;

use Illuminate\Contracts\Http\Kernel as HttpKernel;
use Illuminate\Support\ServiceProvider;
use Riven\Commands\ScanRoutesCommand;
use Riven\Scanner\ActionReflector;
use Riven\Scanner\RouteScanner;
use Riven\Scanner\RulesExtractor;

class RivenServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->app->singleton(RouteScanner::class, function ($app) {
            // Middleware groups and aliases live on the HTTP kernel, which copies them
            // into the router in its constructor. Artisan only boots the console kernel,
            // so build the HTTP one or groups like "web" can't be resolved.
            $app->make(HttpKernel::class);

            return new RouteScanner(
                $app['router'],
                RouteScanner::DEFAULT_EXCLUDED_URIS,
                $app->basePath('vendor'),
                $app->make(ActionReflector::class),
                $app->make(RulesExtractor::class),
            );
        });
    }

    public function boot(): void
    {
        if ($this->app->runningInConsole()) {
            $this->commands([
                ScanRoutesCommand::class,
            ]);
        }
    }
}
