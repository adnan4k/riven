<?php

namespace Riven\Dashboard;

use Illuminate\Http\Client\ConnectionException;
use Illuminate\Support\Facades\Http;

/**
 * Try it for targets on another origin, which the browser can't call
 * itself (CORS). Same-origin targets are sent by the browser directly.
 * Mirrors engine/server's POST /api/try.
 */
class TryProxy
{
    public const METHODS = ['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'];

    public const BODY_LIMIT = 1 << 20;

    /**
     * @param  array<string, string>  $headers
     * @return array{status: int, duration_ms: float, size_bytes: int, headers: array<string, string>, body: string, truncated: bool}
     */
    public function send(string $baseUrl, string $method, string $path, array $headers, string $body, ?string $selfHost = null): array
    {
        if (! in_array($method, self::METHODS, true)) {
            throw new DashboardException("Unsupported method {$method}.", 400);
        }
        // A path only, appended to the target: no scheme or host of its own.
        if (! str_starts_with($path, '/') || str_starts_with($path, '//') || str_contains($path, '\\')) {
            throw new DashboardException('path must start with a single /', 400);
        }

        $start = hrtime(true);
        try {
            $request = Http::withHeaders($headers)
                ->withOptions(['allow_redirects' => false, 'http_errors' => false])
                ->timeout(30);
            if ($body !== '') {
                $request = $request->withBody($body, $headers['Content-Type'] ?? 'application/json');
            }
            $response = $request->send($method, rtrim($baseUrl, '/').$path);
        } catch (ConnectionException $e) {
            $message = "The target didn't answer: {$e->getMessage()}";
            if ($selfHost !== null && strtolower((string) parse_url($baseUrl, PHP_URL_HOST)) === strtolower(explode(':', $selfHost)[0])) {
                // php artisan serve has one worker by default: this request holds it,
                // so the app can't also answer the proxied one.
                $message .= ' The target looks like this same app. If it runs on `php artisan serve`, start it with'
                    .' PHP_CLI_SERVER_WORKERS=4 php artisan serve --no-reload, or open the dashboard on the target\'s exact URL'
                    .' so your browser sends the request itself.';
            }
            throw new DashboardException($message, 502);
        }
        $elapsed = (hrtime(true) - $start) / 1e6;

        $data = $response->body();

        return [
            'status' => $response->status(),
            'duration_ms' => round($elapsed, 1),
            'size_bytes' => strlen($data),
            'headers' => array_map(fn ($values) => implode(', ', $values), $response->headers()),
            'body' => substr($data, 0, self::BODY_LIMIT),
            'truncated' => strlen($data) > self::BODY_LIMIT,
        ];
    }
}
