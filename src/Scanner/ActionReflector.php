<?php

namespace Riven\Scanner;

use Closure;
use Illuminate\Routing\Route;
use Illuminate\Support\Str;
use ReflectionFunction;
use ReflectionFunctionAbstract;
use ReflectionMethod;

/**
 * Reflects the code a route dispatches to: the closure, or the controller method.
 */
class ActionReflector
{
    /**
     * Null when the action can't be reflected (missing class/method, cached closure).
     */
    public function reflect(Route $route): ?ReflectionFunctionAbstract
    {
        $uses = $route->getAction('uses');

        if ($uses instanceof Closure) {
            return new ReflectionFunction($uses);
        }

        // Cached routes hold closures as serialized strings, which can't be reflected.
        if ($route->getActionName() === 'Closure' || ! is_string($uses)) {
            return null;
        }

        // 'uses' is normalized to Class@method; invokables are Class@__invoke.
        [$class, $method] = Str::parseCallback($uses);
        $class = ltrim($class, '\\');

        // method_exists() is false for a missing class too, so this guards both cases.
        if ($method === null || ! method_exists($class, $method)) {
            return null;
        }

        return new ReflectionMethod($class, $method);
    }
}
