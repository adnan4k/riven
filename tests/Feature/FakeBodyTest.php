<?php

use Illuminate\Database\Schema\Blueprint;
use Illuminate\Support\Facades\Artisan;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Route;
use Illuminate\Support\Facades\Schema;
use Illuminate\Support\Facades\Validator;
use Riven\Faking\BodyFaker;
use Riven\Scanner\RulesExtractor;
use Riven\Tests\Fixtures\App\StoreOrderRequest;
use Riven\Tests\Fixtures\App\StoreUserRequest;

beforeEach(function () {
    config(['database.default' => 'testing']);
    Schema::create('customers', function (Blueprint $t) {
        $t->id();
        $t->string('email')->unique();
    });
    Schema::create('products', function (Blueprint $t) {
        $t->id();
        $t->string('name');
    });
    DB::table('customers')->insert([['email' => 'taken@example.com'], ['email' => 'also@example.com']]);
    DB::table('products')->insert([['name' => 'a'], ['name' => 'b'], ['name' => 'c']]);
});

function fakeOrder(): array
{
    return app(BodyFaker::class)->make(app(RulesExtractor::class)->raw(StoreOrderRequest::class));
}

it('generates a body that passes the real rules', function () {
    $result = fakeOrder();

    expect($result['errors'])->toBe([]);
    expect(Validator::make($result['body'], (new StoreOrderRequest)->rules())->passes())->toBeTrue();
});

it('shapes values by rule', function () {
    $body = fakeOrder()['body'];

    expect($body['customer_id'])->toBeIn(DB::table('customers')->pluck('id')->all())  // exists: a real row
        ->and($body['email'])->not->toBeIn(['taken@example.com', 'also@example.com']) // unique: not taken
        ->and($body['status'])->toBeIn(['pending', 'paid'])                            // enum cases
        ->and($body['items'])->toHaveCount(2)                                          // min:2
        ->and($body['items'][0]['product_id'])->toBeIn([1, 2, 3])
        ->and($body['items'][1]['qty'])->toBeGreaterThanOrEqual(1)->toBeLessThanOrEqual(5)
        ->and(strlen($body['shipping']['city']))->toBeLessThanOrEqual(40)              // nested object, max length
        ->and($body['priority'])->toBeIn(['low', 'normal', 'high'])
        ->and($body['reference'])->toMatch('/^ORD-[0-9]{4}$/')                         // regex
        ->and((string) $body['pin'])->toMatch('/^\d{4}$/')                             // digits:4
        ->and(strtotime($body['deliver_on']))->toBeGreaterThan(strtotime('today'))     // after:today
        ->and($body['password_confirmation'])->toBe($body['password'])                 // confirmed
        ->and($body['terms'])->toBeTrue();                                             // accepted
});

it('varies values between bodies', function () {
    $emails = collect(range(1, 5))->map(fn () => fakeOrder()['body']['email']);

    expect($emails->unique())->toHaveCount(5);
});

it('skips file uploads with a warning', function () {
    $result = fakeOrder();

    expect($result['body']['avatar'] ?? null)->toBeNull() // nullable, so null is valid
        ->and($result['warnings'])->toContain("avatar: file uploads can't be generated in a JSON body.");
});

it('warns when an exists: table is empty instead of guessing', function () {
    DB::table('products')->delete();

    $result = fakeOrder();

    expect($result['warnings'])->toContain('items.*.product_id: products has no rows, so exists:products,id can\'t be satisfied. Seed it first.')
        ->and($result['errors'])->toHaveKey('items.0.product_id');
});

it('never writes to the database', function () {
    $before = [DB::table('customers')->count(), DB::table('products')->count()];

    foreach (range(1, 3) as $_) {
        fakeOrder();
    }

    expect([DB::table('customers')->count(), DB::table('products')->count()])->toBe($before);
});

it('prints bodies as JSON from artisan', function () {
    Route::post('/orders', fn (StoreOrderRequest $request) => 'ok');

    expect(Artisan::call('riven:fake-body', ['route' => 'POST /orders', '--count' => 3]))->toBe(0);
    $out = json_decode(trim(Artisan::output()), true);

    expect($out['bodies'])->toHaveCount(3)
        ->and($out['invalid'])->toBe(0)
        ->and($out['form_request'])->toBe(StoreOrderRequest::class);
});

it('explains routes it cannot generate for', function (string $route, string $error) {
    Route::get('/plain', fn () => 'ok');
    Route::post('/users', fn (StoreUserRequest $request) => 'ok');

    expect(Artisan::call('riven:fake-body', ['route' => $route]))->toBe(1)
        ->and(json_decode(trim(Artisan::output()), true)['error'])->toContain($error);
})->with([
    ['GET /nope', 'No route with id'],
    ['GET /plain', 'has no FormRequest'],
]);
