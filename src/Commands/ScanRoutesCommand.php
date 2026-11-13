<?php

namespace Riven\Commands;

use Illuminate\Console\Command;
use Riven\Scanner\RouteScanner;

class ScanRoutesCommand extends Command
{
    protected $signature = 'riven:scan';

    protected $description = 'Scan the application routes';

    public function handle(RouteScanner $scanner): int
    {
        $routes = $scanner->scan();

        if ($routes === []) {
            $this->warn('No routes found.');

            return self::SUCCESS;
        }

        $this->table(
            ['URI', 'Methods', 'Name', 'Action'],
            array_map(fn (array $route) => [
                $route['uri'],
                implode('|', $route['methods']),
                $route['name'] ?? '',
                $route['action'],
            ], $routes),
        );

        return self::SUCCESS;
    }
}
