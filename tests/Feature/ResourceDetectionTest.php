<?php

use Illuminate\Support\Facades\Route;
use Riven\Scanner\RouteScanner;
use Riven\Tests\Fixtures\App\ResourceController;
use Riven\Tests\Fixtures\App\UserCollection;
use Riven\Tests\Fixtures\App\UserResource;

function responseFor(string $uri): array
{
    $route = collect(app(RouteScanner::class)->scan()['routes'])->firstWhere('uri', $uri);

    return $route['response'];
}

function single(string $class): array
{
    return ['resource' => $class, 'collection' => false];
}

function many(string $class): array
{
    return ['resource' => $class, 'collection' => true];
}

$unknown = ['resource' => null, 'collection' => null];

it('detects the resource from a controller method', function (string $method, array $expected) {
    Route::get($method, [ResourceController::class, $method]);

    expect(responseFor($method))->toBe($expected);
})->with([
    'declared return type' => ['typed', single(UserResource::class)],
    'new X()' => ['newResource', single(UserResource::class)],
    'X::make()' => ['make', single(UserResource::class)],
    'X::collection()' => ['collection', many(UserResource::class)],
    'AnonymousResourceCollection type falls back to the body' => ['anonymousCollectionType', many(UserResource::class)],
    'named collection, item inferred from its name' => ['namedCollection', many(UserResource::class)],
    'named collection with $collects' => ['explicitCollects', many(UserResource::class)],
    'chained ->additional()->response()' => ['chained', single(UserResource::class)],
    'several returns that agree' => ['sameEverywhere', single(UserResource::class)],
]);

it('returns unknown instead of guessing', function (string $method) use ($unknown) {
    Route::get($method, [ResourceController::class, $method]);

    expect(responseFor($method))->toBe($unknown);
})->with([
    'returns disagree' => ['conflicting'],
    'one return is not a resource' => ['partlyUnknown'],
    'only a nested closure returns a resource' => ['nestedClosureOnly'],
    'new of a non-resource class' => ['notAResource'],
    'plain array' => ['plainArray'],
    'missing method' => ['missing'],
]);

it('detects resources returned from closures', function () {
    Route::get('arrow', fn () => new UserResource([]));
    Route::get('closure', function () {
        return UserResource::collection([]);
    });
    Route::get('typed', fn (): UserCollection => new UserCollection([]));

    expect(responseFor('arrow'))->toBe(single(UserResource::class))
        ->and(responseFor('closure'))->toBe(many(UserResource::class))
        ->and(responseFor('typed'))->toBe(many(UserResource::class));
});
