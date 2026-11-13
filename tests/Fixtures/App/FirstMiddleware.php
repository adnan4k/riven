<?php

namespace Riven\Tests\Fixtures\App;

class FirstMiddleware
{
    public function handle($request, $next)
    {
        return $next($request);
    }
}
