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

it('keeps the dashboard off outside the local environment', function () {
    expect(app()->environment())->toBe('testing');

    $this->get('/riven')->assertNotFound();
});
