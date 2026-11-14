<?php

namespace Riven\Engine;

use Illuminate\Support\Facades\Http;
use Throwable;

/** Used by `php artisan riven:install-engine` (and faked with Http::fake in tests). */
final class LaravelHttpClient implements HttpClient
{
    public function get(string $url): HttpResponse
    {
        try {
            $response = Http::timeout(120)->withUserAgent('riven-installer')->get($url);
        } catch (Throwable $e) {
            throw new EngineInstallException("Couldn't reach {$url}: {$e->getMessage()}", previous: $e);
        }

        return new HttpResponse($response->status(), $response->body());
    }
}
