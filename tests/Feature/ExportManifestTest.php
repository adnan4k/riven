<?php

use Illuminate\Support\Facades\Route;
use Opis\JsonSchema\Errors\ErrorFormatter;
use Opis\JsonSchema\Validator;
use Riven\Tests\Fixtures\App\NoRulesRequest;
use Riven\Tests\Fixtures\App\PostController;
use Riven\Tests\Fixtures\App\ResourceController;
use Riven\Tests\Fixtures\App\StoreUserAction;
use Riven\Tests\Fixtures\App\UpdateUserRequest;
use Riven\Tests\Fixtures\App\UserController;

const SCHEMA_ID = 'https://github.com/faysal/riven/schema/riven.schema.json';

/**
 * Validation errors as readable text, or [] when the data matches the schema.
 */
function schemaErrors(mixed $data): array
{
    $validator = new Validator;
    $validator->resolver()->registerFile(SCHEMA_ID, __DIR__.'/../../schema/riven.schema.json');

    $result = $validator->validate($data, SCHEMA_ID);

    return $result->isValid() ? [] : (new ErrorFormatter)->format($result->error());
}

function exportTo(string $path): object
{
    test()->artisan('riven:export', ['--output' => $path])->assertSuccessful();

    return json_decode(file_get_contents($path), false, flags: JSON_THROW_ON_ERROR);
}

beforeEach(function () {
    $this->dir = sys_get_temp_dir().'/riven-export-'.uniqid();
});

afterEach(function () {
    (new Illuminate\Filesystem\Filesystem)->deleteDirectory($this->dir);
});

it('exports a manifest that validates against the schema', function () {
    Route::post('users', [UserController::class, 'store'])->name('users.store');
    Route::get('users', [UserController::class, 'index']);
    Route::post('actions/users', StoreUserAction::class);
    Route::domain('{team}.example.com')->get('posts/{post}/{comment?}', [PostController::class, 'show'])->where('post', '[0-9]+');
    Route::put('users/{user}', fn (UpdateUserRequest $request) => '');
    Route::post('empty', fn (NoRulesRequest $request) => '');
    Route::get('resources/new', [ResourceController::class, 'newResource']);
    Route::get('resources/collection', [ResourceController::class, 'collection']);

    $manifest = exportTo($this->dir.'/riven.json');

    expect(schemaErrors($manifest))->toBe([])
        ->and($manifest->routes)->toHaveCount(8)
        ->and($manifest->warnings)->toHaveCount(1);
});

it('fills the envelope', function () {
    $this->travelTo(now()->parse('2026-11-13 12:30:00', 'UTC'));
    config(['app.name' => 'Demo', 'app.url' => 'http://demo.test']);

    $manifest = exportTo($this->dir.'/riven.json');

    expect($manifest->schema_version)->toBe(1)
        ->and($manifest->generator->name)->toBe('faysal/riven')
        ->and($manifest->generated_at)->toBe('2026-11-13T12:30:00Z')
        ->and($manifest->app->name)->toBe('Demo')
        ->and($manifest->app->url)->toBe('http://demo.test')
        ->and($manifest->app->laravel_version)->toBe(app()->version())
        ->and($manifest->app->php_version)->toBe(PHP_VERSION);
});

it('encodes a FormRequest without rules as an empty object, not a list', function () {
    Route::post('empty', fn (NoRulesRequest $request) => '');

    exportTo($path = $this->dir.'/riven.json');

    expect(file_get_contents($path))->toContain('"rules": {}');
});

it('writes to the configured output path, relative to the base path', function () {
    config(['riven.output' => 'build/riven-test-'.uniqid().'.json']);
    $path = base_path(config('riven.output'));

    try {
        $this->artisan('riven:export')
            ->expectsOutputToContain('Wrote')
            ->assertSuccessful();

        expect($path)->toBeFile();
    } finally {
        @unlink($path);
    }
});

it('can write compact JSON', function () {
    $this->artisan('riven:export', ['--output' => $path = $this->dir.'/riven.json', '--compact' => true])->assertSuccessful();

    expect(file_get_contents($path))->not->toContain("\n    ");
});

it('prints warnings and a summary', function () {
    Route::put('users/{user}', fn (UpdateUserRequest $request) => '');

    $this->artisan('riven:export', ['--output' => $this->dir.'/riven.json'])
        ->expectsOutputToContain('PUT /users/{user}: Could not read')
        ->expectsOutputToContain('1 route, 1 warning')
        ->assertSuccessful();
});

it('has a strict schema that rejects fields outside the contract', function () {
    $manifest = exportTo($this->dir.'/riven.json');
    $manifest->unexpected = true;

    expect(schemaErrors($manifest))->not->toBe([]);
});
