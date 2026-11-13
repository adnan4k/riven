<?php

use Riven\Scanner\RouteScanner;

it('registers RouteScanner as a singleton', function () {
    expect(app(RouteScanner::class))
        ->toBeInstanceOf(RouteScanner::class)
        ->toBe(app(RouteScanner::class));
});

it('registers the riven:scan command', function () {
    $this->artisan('riven:scan')->assertSuccessful();
});
