<?php

namespace Riven\Tests\Fixtures\App;

class SecondMiddleware
{
    public function handle($request, $next)
    {
        return $next($request);
    }
}
