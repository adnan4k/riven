<?php

use Illuminate\Support\Facades\Route;
use Riven\Scanner\RouteScanner;

function configuredUris(): array
{
    // Resolve after config() changes: the singleton reads config when first built.
    return array_column(app(RouteScanner::class)->scan()['routes'], 'uri');
}

it('ships defaults that match the scanner defaults', function () {
    expect(config('riven.include'))->toBe([])
        ->and(config('riven.exclude'))->toBe(RouteScanner::DEFAULT_EXCLUDED_URIS)
        ->and(config('riven.exclude_vendor'))->toBeTrue()
        ->and(config('riven.output'))->toBe('riven.json');
});

it('only scans included URIs when include is set', function () {
    config(['riven.include' => ['api/*']]);
    Route::get('api/users', fn () => '');
    Route::get('dashboard', fn () => '');

    expect(configuredUris())->toBe(['api/users']);
});

it('applies exclude on top of include', function () {
    config(['riven.include' => ['api/*'], 'riven.exclude' => ['api/internal/*']]);
    Route::get('api/users', fn () => '');
    Route::get('api/internal/metrics', fn () => '');

    expect(configuredUris())->toBe(['api/users']);
});

it('uses the configured exclude list instead of the defaults', function () {
    config(['riven.exclude' => []]);
    Route::get('up', fn () => '');

    expect(configuredUris())->toContain('up');
});

it('publishes the config file with the riven-config tag', function () {
    $target = config_path('riven.php');
    @unlink($target);

    try {
        $this->artisan('vendor:publish', ['--tag' => 'riven-config'])->assertSuccessful();

        expect($target)->toBeFile()
            ->and(require $target)->toBe(require __DIR__.'/../../config/riven.php');
    } finally {
        @unlink($target);
    }
});
