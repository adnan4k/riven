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
        // TODO (M1): call $scanner->scan() and render the routes with $this->table().
        $this->info('riven:scan is not implemented yet.');

        return self::SUCCESS;
    }
}
