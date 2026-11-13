<?php

namespace Riven;

use Illuminate\Support\ServiceProvider;
use Riven\Commands\ScanRoutesCommand;
use Riven\Scanner\RouteScanner;

class RivenServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->app->singleton(RouteScanner::class);
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
