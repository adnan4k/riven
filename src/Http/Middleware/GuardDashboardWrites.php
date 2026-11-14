<?php

namespace Riven\Http\Middleware;

use Closure;
use Illuminate\Http\Request;
use Symfony\Component\HttpFoundation\Response;

/**
 * Protects the dashboard endpoints that act (send requests, run load), the
 * way engine/server's guard does. These routes skip the "web" group, so
 * there's no session or CSRF token; instead:
 *  - only JSON bodies: a plain HTML form on another site can't post JSON
 *    without the browser asking first (CORS preflight), which we never allow;
 *  - a browser request (Origin header) must come from this app's own origin.
 */
class GuardDashboardWrites
{
    public function handle(Request $request, Closure $next): Response
    {
        if (! str_starts_with((string) $request->header('Content-Type'), 'application/json')) {
            return response()->json(['error' => 'send JSON (Content-Type: application/json)'], 415);
        }

        $origin = $request->header('Origin');
        if ($origin !== null && $this->hostOf($origin) !== strtolower($request->getHttpHost())) {
            return response()->json(['error' => 'requests must come from the Riven dashboard itself'], 403);
        }

        return $next($request);
    }

    private function hostOf(string $origin): ?string
    {
        $parts = parse_url($origin);
        if (! isset($parts['host'])) {
            return null;
        }
        $default = ($parts['scheme'] ?? 'http') === 'https' ? 443 : 80;
        $port = $parts['port'] ?? $default;

        // getHttpHost() leaves the default port out, so do the same.
        return strtolower($parts['host']).($port === $default ? '' : ':'.$port);
    }
}
