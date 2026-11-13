<?php

namespace Riven\Scanner;

use Closure;
use ErrorException;
use Illuminate\Container\Container;
use Illuminate\Validation\Rules\ArrayRule;
use Illuminate\Validation\Rules\Date;
use Illuminate\Validation\Rules\Numeric;
use Illuminate\Validation\Rules\StringRule;
use Stringable;
use Throwable;

/**
 * Reads a FormRequest's rules() and normalizes them to: field => list of rule strings.
 */
class RulesExtractor
{
    /**
     * Fluent rules whose string form is several pipe-joined rules ("string|min:3").
     * Laravel's ValidationRuleParser splits exactly these.
     */
    private const MULTI_RULE_OBJECTS = [ArrayRule::class, Date::class, Numeric::class, StringRule::class];

    private Container $container;

    public function __construct(?Container $container = null)
    {
        $this->container = $container ?? Container::getInstance();
    }

    /**
     * @param  class-string  $formRequest
     * @return array<string, string[]>
     *
     * @throws Throwable when rules() can't run outside a live request
     */
    public function extract(string $formRequest): array
    {
        // Not resolved through the container on purpose: Laravel validates a
        // FormRequest as soon as the container resolves it.
        $request = new $formRequest;
        $request->setContainer($this->container);

        if (! method_exists($request, 'rules')) {
            return [];
        }

        // Reading request state that doesn't exist here ($this->route('user')->id) is only
        // a PHP warning. Make it throw regardless of the caller's error handler, so we
        // never return rules built from missing data.
        set_error_handler(function (int $severity, string $message, string $file, int $line): never {
            throw new ErrorException($message, 0, $severity, $file, $line);
        });

        try {
            // Same call Laravel makes, so rules() can type-hint dependencies.
            $rules = $this->container->call([$request, 'rules']);
        } finally {
            restore_error_handler();
        }

        return array_map(fn ($rule) => $this->normalize($rule), $rules);
    }

    /**
     * @return string[]
     */
    private function normalize(mixed $rule): array
    {
        // A whole-field string is pipe-delimited; strings inside an array are one rule each.
        $parts = is_string($rule) ? explode('|', $rule) : (is_array($rule) ? $rule : [$rule]);

        $rules = [];

        foreach ($parts as $part) {
            array_push($rules, ...$this->stringify($part));
        }

        return array_values(array_filter($rules, fn (string $r) => $r !== ''));
    }

    /**
     * @return string[]
     */
    private function stringify(mixed $rule): array
    {
        if (is_string($rule)) {
            return [$rule];
        }

        if ($rule instanceof Closure) {
            return ['Closure'];
        }

        if (is_object($rule) && in_array($rule::class, self::MULTI_RULE_OBJECTS, true)) {
            return explode('|', (string) $rule);
        }

        if ($rule instanceof Stringable) {
            try {
                return [(string) $rule]; // Rule::in() → in:"a","b", Rule::unique() → unique:users,…
            } catch (Throwable) {
                // e.g. RequiredIf with a callback that needs request data.
            }
        }

        // Custom rule objects, Password, File, ConditionalRules…: the engine maps known classes.
        return is_object($rule) ? [$rule::class] : [(string) $rule];
    }
}
