<?php

namespace Riven\Composer;

use Composer\Composer;
use Composer\EventDispatcher\EventSubscriberInterface;
use Composer\IO\IOInterface;
use Composer\Plugin\PluginInterface;
use Composer\Script\ScriptEvents;
use Riven\Engine\EngineInstaller;
use Throwable;

/**
 * Installs the riven-engine binary whenever Composer installs or updates the
 * app, so `composer require faysal/riven` is the only setup step.
 *
 * Runs inside Composer's process, before the Laravel app can boot, so it only
 * uses Composer's API and the framework-free EngineInstaller. A failure is a
 * warning, never a failed `composer install`.
 */
class Plugin implements PluginInterface, EventSubscriberInterface
{
    private Composer $composer;

    private IOInterface $io;

    public function activate(Composer $composer, IOInterface $io): void
    {
        $this->composer = $composer;
        $this->io = $io;
    }

    public function deactivate(Composer $composer, IOInterface $io): void {}

    /** Called when faysal/riven is removed: take the binary with it. */
    public function uninstall(Composer $composer, IOInterface $io): void
    {
        $target = self::target($composer);
        if (is_file($target) && @unlink($target)) {
            $io->write("<info>Removed {$target}</info>");
        }
    }

    public static function getSubscribedEvents(): array
    {
        return [
            ScriptEvents::POST_INSTALL_CMD => 'installEngine',
            ScriptEvents::POST_UPDATE_CMD => 'installEngine',
        ];
    }

    public function installEngine(): void
    {
        $package = $this->composer->getRepositoryManager()->getLocalRepository()->findPackage(EngineInstaller::PACKAGE, '*');
        if ($package === null) {
            return; // being removed in this run
        }

        $make = fn () => new EngineInstaller(
            self::target($this->composer),
            new ComposerHttpClient($this->composer->getLoop()->getHttpDownloader()),
        );

        self::run($make, $package->getPrettyVersion(), $this->io);
    }

    /**
     * @param  callable(): EngineInstaller  $makeInstaller  a factory, so platform detection errors are caught too
     */
    public static function run(callable $makeInstaller, string $version, IOInterface $io): void
    {
        try {
            $result = $makeInstaller()->install($version);
            $io->write($result->installed ? "<info>{$result->message}</info>" : $result->message);
        } catch (Throwable $e) {
            $io->writeError("<warning>Riven: couldn't install riven-engine: {$e->getMessage()}</warning>");
            $io->writeError('<warning>Everything else is installed. To retry: php artisan riven:install-engine</warning>');
        }
    }

    private static function target(Composer $composer): string
    {
        return $composer->getConfig()->get('bin-dir').'/riven-engine'.(PHP_OS_FAMILY === 'Windows' ? '.exe' : '');
    }
}
