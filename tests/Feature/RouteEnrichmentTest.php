<?php

use Illuminate\Support\Facades\Route;
use Riven\Scanner\RouteScanner;
use Riven\Tests\Fixtures\App\FirstMiddleware;
use Riven\Tests\Fixtures\App\InvokableController;
use Riven\Tests\Fixtures\App\PostController;
use Riven\Tests\Fixtures\App\SecondMiddleware;
use Riven\Tests\Fixtures\App\UnbuildableController;

beforeEach(function () {
    $router = app('router');
    $router->aliasMiddleware('first', FirstMiddleware::class);
    $router->aliasMiddleware('controller-mw', SecondMiddleware::class);
    $router->middlewareGroup('custom-group', ['first:admin', SecondMiddleware::class]);
});

function scanRoute(string $uri): array
{
    $result = app(RouteScanner::class)->scan();

    return collect($result['routes'])->firstWhere('uri', $uri);
}

it('builds an id from the first non-HEAD method and the uri', function () {
    Route::get('users', fn () => '');
    Route::match(['PUT', 'PATCH'], 'users/{user}', fn () => '');

    expect(scanRoute('users')['id'])->toBe('GET /users')
        ->and(scanRoute('users/{user}')['id'])->toBe('PUT /users/{user}');
});

it('describes controller, invokable and closure actions', function () {
    Route::get('posts/{post}', [PostController::class, 'show']);
    Route::get('report', InvokableController::class);
    Route::get('ping', fn () => 'pong');

    expect(scanRoute('posts/{post}')['action'])
        ->toBe(['type' => 'controller', 'class' => PostController::class, 'method' => 'show'])
        ->and(scanRoute('report')['action'])
        ->toBe(['type' => 'invokable', 'class' => InvokableController::class, 'method' => '__invoke'])
        ->and(scanRoute('ping')['action'])
        ->toBe(['type' => 'closure', 'class' => null, 'method' => null]);
});

it('records the domain, or null when there is none', function () {
    Route::domain('{team}.example.com')->get('dashboard', fn () => '');
    Route::get('home', fn () => '');

    expect(scanRoute('dashboard')['domain'])->toBe('{team}.example.com')
        ->and(scanRoute('home')['domain'])->toBeNull();
});

it('lists parameters with optional flags and where patterns, including domain parameters', function () {
    Route::domain('{team}.example.com')
        ->get('posts/{post}/comments/{comment?}', fn () => '')
        ->where('post', '[0-9]+');

    expect(scanRoute('posts/{post}/comments/{comment?}')['parameters'])->toBe([
        ['name' => 'team', 'required' => true, 'pattern' => null],
        ['name' => 'post', 'required' => true, 'pattern' => '[0-9]+'],
        ['name' => 'comment', 'required' => false, 'pattern' => null],
    ]);
});

it('returns declared middleware and the classes they resolve to', function () {
    Route::get('posts/{post}', [PostController::class, 'show'])->middleware('custom-group');

    expect(scanRoute('posts/{post}'))
        ->middleware->toBe(['custom-group', 'controller-mw'])
        ->middleware_resolved->toBe([
            FirstMiddleware::class.':admin',
            SecondMiddleware::class,
        ]);
});

it('drops middleware excluded with withoutMiddleware from the resolved list', function () {
    Route::get('open', fn () => '')
        ->middleware('custom-group')
        ->withoutMiddleware(SecondMiddleware::class);

    expect(scanRoute('open')['middleware_resolved'])->toBe([FirstMiddleware::class.':admin']);
});

it('warns instead of crashing when controller middleware cannot be read', function () {
    Route::get('broken', [UnbuildableController::class, 'index'])->middleware('first');

    $result = app(RouteScanner::class)->scan();
    $route = collect($result['routes'])->firstWhere('uri', 'broken');

    expect($route['middleware'])->toBe(['first'])
        ->and($result['warnings'])->toHaveCount(1)
        ->and($result['warnings'][0])
        ->route_id->toBe('GET /broken')
        ->code->toBe('controller_middleware_unresolved');
});
