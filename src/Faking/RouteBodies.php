<?php

namespace Riven\Faking;

use Riven\Scanner\RouteScanner;
use Riven\Scanner\RulesExtractor;
use RuntimeException;
use Throwable;

/**
 * Request bodies for one route, generated from its FormRequest rules.
 * Shared by `php artisan riven:fake-body` and the dashboard.
 */
class RouteBodies
{
    public const MAX = 500;

    public function __construct(
        private RouteScanner $scanner,
        private RulesExtractor $rules,
        private BodyFaker $faker,
    ) {}

    /**
     * @return array{route_id: string, form_request: string, bodies: list<object>, invalid: int, errors: object, warnings: list<string>}
     *
     * @throws RuntimeException with a message meant for the user
     */
    public function generate(string $id, int $count): array
    {
        if ($count < 1 || $count > self::MAX) {
            throw new RuntimeException('--count must be between 1 and '.self::MAX);
        }

        $route = collect($this->scanner->scan()['routes'])->firstWhere('id', $id);
        if ($route === null) {
            throw new RuntimeException("No route with id \"{$id}\" (see php artisan riven:scan).");
        }
        $formRequest = $route['request']['form_request'];
        if ($formRequest === null) {
            throw new RuntimeException("{$id} has no FormRequest, so there are no rules to generate a body from.");
        }

        try {
            $raw = $this->rules->raw($formRequest);
        } catch (Throwable $e) {
            throw new RuntimeException("{$formRequest}::rules() needs a live request: {$e->getMessage()}", previous: $e);
        }

        $bodies = [];
        $invalid = 0;
        $errors = [];
        $warnings = [];
        for ($i = 0; $i < $count; $i++) {
            $result = $this->faker->make($raw);
            $bodies[] = (object) $result['body']; // {} rather than [] when empty
            if ($result['errors'] !== []) {
                $invalid++;
                $errors += $result['errors'];
            }
            $warnings = [...$warnings, ...$result['warnings']];
        }

        return [
            'route_id' => $id,
            'form_request' => $formRequest,
            'bodies' => $bodies,
            'invalid' => $invalid,   // bodies that still fail validation
            'errors' => (object) $errors, // field => messages, from those bodies
            'warnings' => array_values(array_unique($warnings)),
        ];
    }
}
