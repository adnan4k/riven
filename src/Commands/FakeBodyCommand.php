<?php

namespace Riven\Commands;

use Illuminate\Console\Command;
use Riven\Faking\BodyFaker;
use Riven\Scanner\RouteScanner;
use Riven\Scanner\RulesExtractor;
use Throwable;

class FakeBodyCommand extends Command
{
    protected $signature = 'riven:fake-body
        {route : Route id as in the manifest, e.g. "POST /api/users"}
        {--count=1 : How many bodies to generate (1-500)}';

    protected $description = 'Generate request bodies that pass a route\'s FormRequest rules (prints JSON)';

    public const MAX = 500;

    public function handle(RouteScanner $scanner, RulesExtractor $rules, BodyFaker $faker): int
    {
        $id = (string) $this->argument('route');
        $count = (int) $this->option('count');

        if ($count < 1 || $count > self::MAX) {
            return $this->fail_("--count must be between 1 and ".self::MAX);
        }

        $route = collect($scanner->scan()['routes'])->firstWhere('id', $id);
        if ($route === null) {
            return $this->fail_("No route with id \"{$id}\" (see php artisan riven:scan).");
        }
        $formRequest = $route['request']['form_request'];
        if ($formRequest === null) {
            return $this->fail_("{$id} has no FormRequest, so there are no rules to generate a body from.");
        }

        try {
            $raw = $rules->raw($formRequest);
        } catch (Throwable $e) {
            return $this->fail_("{$formRequest}::rules() needs a live request: {$e->getMessage()}");
        }

        $bodies = [];
        $invalid = 0;
        $errors = [];
        $warnings = [];
        for ($i = 0; $i < $count; $i++) {
            $result = $faker->make($raw);
            $bodies[] = (object) $result['body']; // {} rather than [] when empty
            if ($result['errors'] !== []) {
                $invalid++;
                $errors += $result['errors'];
            }
            $warnings = [...$warnings, ...$result['warnings']];
        }

        $this->output->writeln(json_encode([
            'route_id' => $id,
            'form_request' => $formRequest,
            'bodies' => $bodies,
            'invalid' => $invalid,   // bodies that still fail validation
            'errors' => (object) $errors, // field => messages, from those bodies
            'warnings' => array_values(array_unique($warnings)),
        ], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE | JSON_THROW_ON_ERROR));

        return self::SUCCESS;
    }

    // Errors are JSON too, so the engine can show them.
    private function fail_(string $message): int
    {
        $this->output->writeln(json_encode(['error' => $message], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE));

        return self::FAILURE;
    }
}
