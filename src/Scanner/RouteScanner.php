<?php

namespace Riven\Scanner;

use Closure;
use Illuminate\Routing\RedirectController;
use Illuminate\Routing\Route;
use Illuminate\Routing\Router;
use Illuminate\Routing\ViewController;
use Illuminate\Support\Str;
use ReflectionClass;
use ReflectionFunction;
use Throwable;

class RouteScanner
{
    /**
     * URIs registered by the framework or common packages.
     */
    public const DEFAULT_EXCLUDED_URIS = [
        'up',
        '_ignition/*',
        'sanctum/*',
        'storage/*',
    ];

    /**
     * Framework controllers that back app-defined routes (Route::view, Route::redirect),
     * so living under vendor/ doesn't make them vendor routes.
     */
    private const APP_ROUTE_CONTROLLERS = [
        ViewController::class,
        RedirectController::class,
    ];

    private ?string $vendorPath;

    /** @var array<int, array{route_id: string, code: string, message: string}> */
    private array $warnings = [];

    /**
     * @param  string[]  $excludedUris  Wildcard patterns matched with Str::is()
     * @param  ?string  $vendorPath  Routes whose action is defined under this path are skipped
     */
    public function __construct(
        private Router $router,
        private array $excludedUris = self::DEFAULT_EXCLUDED_URIS,
        ?string $vendorPath = null,
    ) {
        // Reflection reports resolved paths, so compare against a resolved path too.
        $this->vendorPath = $vendorPath === null ? null : (realpath($vendorPath) ?: $vendorPath);
    }

    /**
     * @return array{routes: array<int, array<string, mixed>>, warnings: array<int, array{route_id: string, code: string, message: string}>}
     */
    public function scan(): array
    {
        $this->warnings = [];

        $routes = collect($this->router->getRoutes()->getRoutes())
            ->reject(fn (Route $route) => $this->isExcludedUri($route) || $this->isVendorAction($route))
            ->map(fn (Route $route) => $this->describe($route))
            ->values()
            ->all();

        return ['routes' => $routes, 'warnings' => $this->warnings];
    }

    /**
     * @return array<string, mixed>
     */
    private function describe(Route $route): array
    {
        $id = $this->id($route);

        return [
            'id' => $id,
            'uri' => $route->uri(),
            'methods' => $route->methods(),
            'name' => $route->getName(),
            'domain' => $route->getDomain(),
            'action' => $this->action($route),
            ...$this->middleware($route, $id),
            'parameters' => $this->parameters($route),
        ];
    }

    /**
     * One id per route: the first non-HEAD method plus the URI, e.g. "GET /users".
     */
    private function id(Route $route): string
    {
        $methods = array_values(array_diff($route->methods(), ['HEAD']));
        $method = $methods[0] ?? $route->methods()[0];

        return $method.' /'.ltrim($route->uri(), '/');
    }

    /**
     * @return array{type: string, class: ?string, method: ?string}
     */
    private function action(Route $route): array
    {
        // getActionName() is 'Closure' for both live and cached (serialized) closures.
        if ($route->getActionName() === 'Closure') {
            return ['type' => 'closure', 'class' => null, 'method' => null];
        }

        // 'uses' is normalized to Class@method (invokables get @__invoke); getActionMethod()
        // reads the un-normalized name instead and returns the class for invokables.
        [, $method] = Str::parseCallback($route->getAction('uses'));

        return [
            'type' => $method === '__invoke' ? 'invokable' : 'controller',
            'class' => ltrim($route->getControllerClass(), '\\'),
            'method' => $method,
        ];
    }

    /**
     * Declared names (aliases, groups, controller middleware) and the classes they resolve to.
     *
     * @return array{middleware: string[], middleware_resolved: string[]}
     */
    private function middleware(Route $route, string $id): array
    {
        try {
            // Includes controller middleware; for controllers using the legacy
            // getMiddleware() style this instantiates the controller.
            $declared = $route->gatherMiddleware();
            $excluded = $route->excludedMiddleware();
        } catch (Throwable $e) {
            $declared = $route->middleware();
            $excluded = (array) $route->getAction('excluded_middleware');

            $this->warnings[] = [
                'route_id' => $id,
                'code' => 'controller_middleware_unresolved',
                'message' => 'Could not instantiate the controller to read its middleware: '.$e->getMessage(),
            ];
        }

        return [
            'middleware' => array_values($declared),
            'middleware_resolved' => array_values($this->router->resolveMiddleware($declared, $excluded)),
        ];
    }

    /**
     * @return array<int, array{name: string, required: bool, pattern: ?string}>
     */
    private function parameters(Route $route): array
    {
        $path = $route->getDomain().'/'.$route->uri();

        return array_map(fn (string $name) => [
            'name' => $name,
            'required' => ! str_contains($path, '{'.$name.'?}'),
            'pattern' => $route->wheres[$name] ?? null,
        ], $route->parameterNames());
    }

    private function isExcludedUri(Route $route): bool
    {
        return $this->excludedUris !== [] && Str::is($this->excludedUris, $route->uri());
    }

    private function isVendorAction(Route $route): bool
    {
        if ($this->vendorPath === null) {
            return false;
        }

        $file = $this->actionFile($route);

        return $file !== null && str_starts_with($file, $this->vendorPath.DIRECTORY_SEPARATOR);
    }

    /**
     * The file the route's action is defined in, or null if it can't be determined
     * (or the action is a framework controller that backs app routes).
     */
    private function actionFile(Route $route): ?string
    {
        $uses = $route->getAction('uses');

        // Cached routes store closures serialized, not as Closure instances.
        if ($uses instanceof Closure) {
            return (new ReflectionFunction($uses))->getFileName() ?: null;
        }

        $class = $route->getControllerClass();
        $class = $class === null ? null : ltrim($class, '\\');

        if ($class === null || in_array($class, self::APP_ROUTE_CONTROLLERS, true) || ! class_exists($class)) {
            return null;
        }

        return (new ReflectionClass($class))->getFileName() ?: null;
    }
}
