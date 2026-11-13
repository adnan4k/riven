<?php

use Illuminate\Support\Facades\Route;
use Riven\Scanner\RouteScanner;
use Riven\Tests\Fixtures\App\StoreUserAction;
use Riven\Tests\Fixtures\App\StoreUserRequest;
use Riven\Tests\Fixtures\App\UserController;

function formRequestFor(string $uri): ?string
{
    $route = collect(app(RouteScanner::class)->scan()['routes'])->firstWhere('uri', $uri);

    return $route['request']['form_request'];
}

it('finds the FormRequest type-hinted on a controller method', function () {
    Route::post('users', [UserController::class, 'store']);

    expect(formRequestFor('users'))->toBe(StoreUserRequest::class);
});

it('finds the FormRequest on an invokable controller', function () {
    Route::post('users', StoreUserAction::class);

    expect(formRequestFor('users'))->toBe(StoreUserRequest::class);
});

it('finds the FormRequest on a closure', function () {
    Route::post('users', fn (StoreUserRequest $request) => '');

    expect(formRequestFor('users'))->toBe(StoreUserRequest::class);
});

it('finds a nullable FormRequest that is not the first parameter', function () {
    Route::put('users/{id}', [UserController::class, 'update']);

    expect(formRequestFor('users/{id}'))->toBe(StoreUserRequest::class);
});

it('returns null for a plain Request and builtin types', function () {
    Route::get('users', [UserController::class, 'index']);

    expect(formRequestFor('users'))->toBeNull();
});

it('returns null when the controller method does not exist', function () {
    Route::get('users', [UserController::class, 'missing']);

    expect(formRequestFor('users'))->toBeNull();
});
