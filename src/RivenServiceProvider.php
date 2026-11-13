<?php

namespace Riven;
use  Illuminate\Support\ServiceProvider;
use Riven\Scanner\RouteScanner;
class RivenServiceProvider extends ServiceProvider
{
    public function register()
    {
        $this->app->singleton(RouteScanner::class, function () {
            return new \Riven\Scanner\RouteScanner();
        });
    }


    public function boot()
    {
        //
         if($this->app->runningInConsole()){
            $this->commands([
                \Riven\Scanner\ScanRouteCommands::class
            ]);
    }
    }
}