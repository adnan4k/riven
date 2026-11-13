<?php

namespace Riven\Tests\Fixtures\App;

use Illuminate\Routing\Controllers\HasMiddleware;

class PostController implements HasMiddleware
{
    public static function middleware(): array
    {
        return ['controller-mw'];
    }

    public function show() {}
}
