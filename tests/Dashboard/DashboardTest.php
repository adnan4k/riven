<?php

use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Route;
use Riven\Tests\Fixtures\App\StoreUserRequest;
use Riven\Tests\Fixtures\App\UserController;

const ORIGIN = 'http://app.test:8080';

/** A POST the way the dashboard sends it (JSON, from its own page). */
function dashboardPost($test, string $uri, array $body, array $headers = [])
{
    return $test->withHeaders(['Origin' => ORIGIN, ...$headers])->postJson('http://app.test:8080/riven/api/'.$uri, $body);
}

function waitForRun($test): array
{
    for ($i = 0; $i < 50; $i++) {
        $state = $test->getJson('/riven/api/stress')->json();
        if ($state['state'] !== 'running') {
            return $state;
        }
        usleep(100_000);
    }

    return $state;
}

beforeEach(function () {
    Route::post('api/users', [UserController::class, 'store'])->name('users.store');
    Route::get('api/ping', fn () => 'pong');
});

it('serves the dashboard at /riven, told where it is mounted', function () {
    $this->get('/riven')->assertOk()
        ->assertHeader('Content-Type', 'text/html; charset=utf-8')
        ->assertSee('<base href="/riven/" />', false);

    // Client-side routes get index.html too, so reloads work.
    $this->get('/riven/routes/GET%20%2Fapi%2Fping/stress')->assertOk()->assertSee('riven ui');
});

it('serves the assets with their types, cached for good', function () {
    $response = $this->get('/riven/assets/app-abc.js')->assertOk();

    expect($response->headers->get('Content-Type'))->toStartWith('text/javascript')
        ->and($response->headers->get('Cache-Control'))->toContain('immutable');
});

it('never serves files outside the dashboard folder', function () {
    file_put_contents($this->dir.'/secret.txt', 'secret');

    $this->get('/riven/..%2F..%2Fsecret.txt')->assertOk()->assertDontSee('secret')->assertSee('riven ui');
});

it('explains when the engine is missing', function () {
    config(['riven.engine.path' => $this->dir.'/nope']);

    $this->get('/riven')->assertStatus(503)->assertSee('composer install');
});

it('defaults the target to APP_URL and has no event stream', function () {
    $this->getJson('/riven/api/config')->assertOk()->assertExactJson([
        'base_url' => 'http://app.test:8080', 'can_rescan' => true, 'live_updates' => false,
    ]);
});

it('builds the manifest live, without its own routes', function () {
    $ids = collect($this->getJson('/riven/api/manifest')->assertOk()->json('routes'))->pluck('id');

    expect($ids)->toContain('POST /api/users', 'GET /api/ping')
        ->and($ids->filter(fn ($id) => str_contains($id, 'riven')))->toBeEmpty();
});

it('builds the OpenAPI document with the engine', function () {
    $this->getJson('/riven/api/openapi')->assertOk()->assertJson(['openapi' => '3.1.0', 'x-route-count' => 2]);
});

it('refuses writes that are not JSON or come from another site', function () {
    $this->withHeaders(['Origin' => ORIGIN])->post('http://app.test:8080/riven/api/rescan', ['x' => 1])->assertStatus(415);
    dashboardPost($this, 'rescan', [], ['Origin' => 'https://evil.example'])->assertStatus(403);
    dashboardPost($this, 'rescan', [], ['Origin' => 'http://app.test:9999'])->assertStatus(403);
    dashboardPost($this, 'rescan', [])->assertOk();
});

it('proxies Try it to the chosen target', function () {
    Http::fake(['other.test/*' => Http::response('{"ok":true}', 201, ['X-From' => 'other'])]);

    dashboardPost($this, 'try', [
        'base_url' => 'http://other.test/', 'method' => 'POST', 'path' => '/api/users',
        'headers' => ['Accept' => 'application/json', 'Content-Type' => 'application/json'], 'body' => '{"name":"a"}',
    ])->assertOk()->assertJson(['status' => 201, 'body' => '{"ok":true}', 'headers' => ['X-From' => 'other']]);

    Http::assertSent(fn ($r) => $r->url() === 'http://other.test/api/users' && $r->body() === '{"name":"a"}');
});

it('refuses a target that is not an http URL, or a path with a host', function () {
    dashboardPost($this, 'try', ['base_url' => 'file:///etc', 'method' => 'GET', 'path' => '/'])->assertStatus(400);
    dashboardPost($this, 'try', ['method' => 'GET', 'path' => '//evil.example/'])->assertStatus(400);
});

it('generates bodies from the route rules', function () {
    dashboardPost($this, 'fake', ['route_id' => 'POST /api/users', 'count' => 2])->assertOk()
        ->assertJson(['route_id' => 'POST /api/users', 'form_request' => StoreUserRequest::class])
        ->assertJsonCount(2, 'bodies');

    dashboardPost($this, 'fake', ['route_id' => 'GET /api/ping'])->assertStatus(422)->assertJsonPath('error', fn ($e) => str_contains($e, 'no FormRequest'));
});

it('runs stress in the background against the chosen target and lists the report', function () {
    dashboardPost($this, 'stress', [
        'base_url' => 'http://nginx', 'routes' => ['POST /api/users'], 'params' => ['user' => '1'],
        'token' => 'secret', 'concurrency' => 3, 'duration_s' => 5, 'fake_bodies' => 2,
    ])->assertOk()->assertJson(['state' => 'running', 'duration_ms' => 5000]);

    $state = waitForRun($this);
    $args = json_decode(file_get_contents($this->dir.'/storage/stress/args.json'), true);

    expect($state['state'])->toBe('done')
        ->and(implode(' ', $args))->toContain('--base-url http://nginx', '--route POST /api/users', '--param user=1', '--token secret', '--concurrency 3', '--duration 5s', '--bodies-file')
        ->and(json_decode(file_get_contents($this->dir.'/storage/stress/bodies.json'), true)['POST /api/users'])->toHaveCount(2);

    $this->getJson('/riven/api/reports')->assertOk()->assertJsonPath('0.name', $state['report'])->assertJsonPath('0.routes', 1);
    $this->getJson('/riven/api/reports/'.$state['report'])->assertOk()->assertJsonPath('config.base_url', 'http://nginx');
});

it('allows one stress run at a time and notices a dead engine', function () {
    @mkdir($this->dir.'/storage/stress', 0755, true);
    $live = $this->dir.'/storage/stress/live.json';
    file_put_contents($live, json_encode(['state' => 'running', 'id' => '1', 'elapsed_ms' => 0, 'duration_ms' => 0, 'interrupted' => false]));

    dashboardPost($this, 'stress', ['routes' => ['GET /api/ping'], 'concurrency' => 1, 'requests' => 1])->assertStatus(409);
    dashboardPost($this, 'stress/stop', [])->assertOk();
    expect(file_exists($this->dir.'/storage/stress/stop'))->toBeTrue();

    touch($live, time() - 60);
    $this->getJson('/riven/api/stress')->assertOk()->assertJson(['state' => 'failed'])
        ->assertJsonPath('error', fn ($e) => str_contains($e, 'stopped without finishing'));
});

it('validates stress settings', function () {
    dashboardPost($this, 'stress', ['routes' => [], 'concurrency' => 1, 'requests' => 1])->assertStatus(400);
    dashboardPost($this, 'stress', ['routes' => ['GET /api/ping'], 'concurrency' => 0, 'requests' => 1])->assertStatus(400);
    dashboardPost($this, 'stress', ['routes' => ['GET /api/ping'], 'concurrency' => 1])->assertStatus(400);
});

it('refuses report names that leave the reports folder', function () {
    $this->getJson('/riven/api/reports/..%2Fsecret.json')->assertNotFound();
    $this->getJson('/riven/api/nope')->assertNotFound()->assertJson(['error' => 'no such API endpoint']);
});
