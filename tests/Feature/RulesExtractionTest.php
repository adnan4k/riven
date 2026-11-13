<?php

use Illuminate\Support\Facades\Route;
use Illuminate\Validation\Rules\Password;
use Riven\Scanner\RouteScanner;
use Riven\Scanner\RulesExtractor;
use Riven\Tests\Fixtures\App\NoRulesRequest;
use Riven\Tests\Fixtures\App\RuleObjectsRequest;
use Riven\Tests\Fixtures\App\StoreUserRequest;
use Riven\Tests\Fixtures\App\Uppercase;
use Riven\Tests\Fixtures\App\UpdateUserRequest;
use Riven\Tests\Fixtures\App\UserController;

it('normalizes pipe strings and arrays to a list of rule strings', function () {
    expect(app(RulesExtractor::class)->extract(StoreUserRequest::class))->toBe([
        'email' => ['required', 'email'],
        'name' => ['required', 'string', 'max:255'],
    ]);
});

it('stringifies rule objects, closures and custom rules', function () {
    expect(app(RulesExtractor::class)->extract(RuleObjectsRequest::class))->toBe([
        'role' => ['required', 'in:"admin","editor"'],
        'email' => ['required', 'unique:users,NULL,NULL,id'],
        'title' => ['string', 'min:3'],
        'password' => ['required', Password::class],
        'code' => ['required', Uppercase::class],
        'nickname' => ['nullable', 'Closure'],
        'items.*.id' => ['required', 'integer'],
        'empty_pipes' => ['required', 'string'],
    ]);
});

it('returns no rules for a FormRequest without a rules() method', function () {
    expect(app(RulesExtractor::class)->extract(NoRulesRequest::class))->toBe([]);
});

it('does not run validation while reading rules', function () {
    // Resolving a FormRequest through the container would validate it and throw.
    expect(fn () => app(RulesExtractor::class)->extract(StoreUserRequest::class))->not->toThrow(Throwable::class);
});

it('puts the rules on the route', function () {
    Route::post('users', [UserController::class, 'store']);

    $route = collect(app(RouteScanner::class)->scan()['routes'])->firstWhere('uri', 'users');

    expect($route['request'])->toBe([
        'form_request' => StoreUserRequest::class,
        'rules' => ['email' => ['required', 'email'], 'name' => ['required', 'string', 'max:255']],
    ]);
});

it('warns and leaves rules null when rules() needs a live request', function () {
    Route::put('users/{user}', fn (UpdateUserRequest $request) => '');

    $result = app(RouteScanner::class)->scan();
    $route = collect($result['routes'])->firstWhere('uri', 'users/{user}');

    expect($route['request'])->toBe(['form_request' => UpdateUserRequest::class, 'rules' => null])
        ->and($result['warnings'])->toHaveCount(1)
        ->and($result['warnings'][0])
        ->route_id->toBe('PUT /users/{user}')
        ->code->toBe('rules_need_request');
});

it('treats PHP warnings in rules() as failures even without Laravel\'s error handler', function () {
    // Tinker and plain scripts don't convert warnings to exceptions; the extractor must.
    set_error_handler(fn () => true);

    try {
        expect(fn () => app(RulesExtractor::class)->extract(UpdateUserRequest::class))
            ->toThrow(ErrorException::class, 'Attempt to read property "id" on null');
    } finally {
        restore_error_handler();
    }
});

it('has no rules when the route has no FormRequest', function () {
    Route::get('users', [UserController::class, 'index']);

    $route = collect(app(RouteScanner::class)->scan()['routes'])->firstWhere('uri', 'users');

    expect($route['request'])->toBe(['form_request' => null, 'rules' => null]);
});
