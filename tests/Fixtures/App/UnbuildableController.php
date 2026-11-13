<?php

namespace Riven\Tests\Fixtures\App;

/**
 * Legacy-style controller (has getMiddleware()), so Laravel instantiates it to
 * read middleware. The container can't resolve the string argument, so that fails.
 */
class UnbuildableController
{
    public function __construct(string $secret) {}

    public function getMiddleware(): array
    {
        return [];
    }

    public function index() {}
}
