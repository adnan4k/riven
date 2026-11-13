<?php

use Illuminate\Support\Facades\Route;
use Riven\Scanner\RouteScanner;

class FakeUserController
{
    public function index() {}
}

function scannerWith(array $excludedUris = RouteScanner::DEFAULT_EXCLUDED_URIS): RouteScanner
{
    return new RouteScanner(app('router'), $excludedUris, __DIR__.'/../Fixtures/Vendor');
}

function scannedUris(RouteScanner $scanner): array
{
    return array_column($scanner->scan()['routes'], 'uri');
}

it('returns uri, methods, name and action for each route', function () {
    Route::get('users', [FakeUserController::class, 'index'])->name('users.index');
    Route::post('ping', fn () => 'pong');

    $routes = collect(app(RouteScanner::class)->scan()['routes'])->keyBy('uri');

    expect($routes['users'])
        ->uri->toBe('users')
        ->methods->toBe(['GET', 'HEAD'])
        ->name->toBe('users.index')
        ->action->toBe(['type' => 'controller', 'class' => 'FakeUserController', 'method' => 'index']);

    expect($routes['ping'])
        ->methods->toBe(['POST'])
        ->name->toBeNull()
        ->action->toBe(['type' => 'closure', 'class' => null, 'method' => null]);
});

it('skips routes whose action is defined in the vendor path', function () {
    Route::get('users', [FakeUserController::class, 'index']);
    Route::post('ping', fn () => 'pong');
    require __DIR__.'/../Fixtures/Vendor/routes.php';

    expect(scannedUris(scannerWith()))
        ->toContain('users', 'ping')
        ->not->toContain('vendor-closure')
        ->not->toContain('vendor-controller');
});

it('keeps Route::view and Route::redirect routes even though their controllers are in vendor', function () {
    Route::view('about', 'about');
    Route::redirect('old', 'new');

    // The real vendor dir, where ViewController and RedirectController live.
    $scanner = new RouteScanner(app('router'), [], dirname(__DIR__, 2).'/vendor');

    expect(scannedUris($scanner))->toContain('about', 'old');
});

it('skips routes matching the default excluded URIs', function () {
    Route::get('users', [FakeUserController::class, 'index']);
    Route::get('up', fn () => 'ok');
    Route::get('sanctum/csrf-cookie', fn () => '');
    Route::get('_ignition/health-check', fn () => '');
    Route::get('storage/{path}', fn () => '');

    expect(scannedUris(scannerWith()))
        ->toContain('users')
        ->not->toContain('up')
        ->not->toContain('sanctum/csrf-cookie')
        ->not->toContain('_ignition/health-check')
        ->not->toContain('storage/{path}');
});

it('uses custom excluded URIs instead of the defaults', function () {
    Route::get('admin/stats', fn () => '');
    Route::get('sanctum/csrf-cookie', fn () => '');

    expect(scannedUris(scannerWith(['admin/*'])))
        ->toContain('sanctum/csrf-cookie')
        ->not->toContain('admin/stats');
});

it('keeps routes whose controller class does not exist', function () {
    Route::get('ghost', ['App\\Http\\Controllers\\MissingController', 'index']);

    expect(scannedUris(scannerWith()))->toContain('ghost');
});

it('prints the routes as a table', function () {
    Route::get('users', [FakeUserController::class, 'index'])->name('users.index');

    // Each expectation consumes the line it matches, so check one string per table row.
    $this->artisan('riven:scan')
        ->expectsOutputToContain('users.index | FakeUserController@index')
        ->assertSuccessful();
});
