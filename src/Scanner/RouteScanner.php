<?php

namespace Riven\Scanner;

use Illuminate\Routing\Route;
use Illuminate\Routing\Router;

class RouteScanner
{
    public function __construct(private Router $router)
    {
    }

    /**
     * @return array<int, array{uri: string, methods: string[], name: ?string, action: string}>
     */
    public function scan(): array
    {
        return collect($this->router->getRoutes()->getRoutes())
            ->map(fn (Route $route) => [
                'uri' => $route->uri(),
                'methods' => $route->methods(),
                'name' => $route->getName(),
                'action' => $route->getActionName(),
            ])
            ->values()
            ->all();
    }
}
