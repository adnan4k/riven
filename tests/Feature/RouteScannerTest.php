<?php

use Illuminate\Support\Facades\Route;
use Riven\Scanner\RouteScanner;

class FakeUserController
{
    public function index() {}
}

it('returns uri, methods, name and action for each route', function () {
    Route::get('users', [FakeUserController::class, 'index'])->name('users.index');
    Route::post('ping', fn () => 'pong');

    $routes = collect(app(RouteScanner::class)->scan())->keyBy('uri');

    expect($routes['users'])->toBe([
        'uri' => 'users',
        'methods' => ['GET', 'HEAD'],
        'name' => 'users.index',
        'action' => 'FakeUserController@index',
    ]);

    expect($routes['ping'])
        ->methods->toBe(['POST'])
        ->name->toBeNull()
        ->action->toBe('Closure');
});

it('prints the routes as a table', function () {
    Route::get('users', [FakeUserController::class, 'index'])->name('users.index');

    $this->artisan('riven:scan')
        ->expectsOutputToContain('users.index')
        ->assertSuccessful();
});
