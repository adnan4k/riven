<?php

namespace Riven\Commands;

use Illuminate\Console\Command;
use Riven\Faking\RouteBodies;
use RuntimeException;

class FakeBodyCommand extends Command
{
    protected $signature = 'riven:fake-body
        {route : Route id as in the manifest, e.g. "POST /api/users"}
        {--count=1 : How many bodies to generate (1-500)}';

    protected $description = 'Generate request bodies that pass a route\'s FormRequest rules (prints JSON)';

    public const MAX = RouteBodies::MAX;

    public function handle(RouteBodies $bodies): int
    {
        try {
            $result = $bodies->generate((string) $this->argument('route'), (int) $this->option('count'));
        } catch (RuntimeException $e) {
            // Errors are JSON too, so the engine can show them.
            $this->output->writeln(json_encode(['error' => $e->getMessage()], JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE));

            return self::FAILURE;
        }

        $this->output->writeln(json_encode($result, JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE | JSON_THROW_ON_ERROR));

        return self::SUCCESS;
    }
}
