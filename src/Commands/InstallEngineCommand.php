<?php

namespace Riven\Commands;

use Illuminate\Console\Command;
use Riven\Engine\EngineInstaller;
use Riven\Engine\EngineInstallException;

class InstallEngineCommand extends Command
{
    // --version is taken by Symfony Console on every command, hence --engine-version.
    protected $signature = 'riven:install-engine
        {--engine-version= : Version to install (default: the installed faysal/riven version)}
        {--force : Download even if that version is already installed}';

    protected $description = 'Download the riven-engine binary for this platform and verify its checksum';

    public function handle(EngineInstaller $installer): int
    {
        try {
            $result = $installer->install($this->option('engine-version'), (bool) $this->option('force'));
        } catch (EngineInstallException $e) {
            $this->error($e->getMessage());

            return self::FAILURE;
        }

        $result->installed ? $this->info($result->message) : $this->line($result->message);

        return self::SUCCESS;
    }
}
