<?php

use Composer\IO\BufferIO;
use Composer\Script\ScriptEvents;
use Riven\Composer\Plugin;
use Riven\Engine\EngineInstaller;
use Riven\Engine\EngineInstallException;
use Riven\Engine\HttpClient;
use Riven\Engine\HttpResponse;

/** Serves fixed responses by URL, 404 otherwise. No Laravel involved, like inside Composer. */
function staticHttp(array $responses): HttpClient
{
    return new class($responses) implements HttpClient
    {
        public function __construct(private array $responses) {}

        public function get(string $url): HttpResponse
        {
            return $this->responses[$url] ?? new HttpResponse(404, 'not found');
        }
    };
}

beforeEach(function () {
    $this->dir = sys_get_temp_dir().'/riven-plugin-'.uniqid();
    mkdir($this->dir);
    $this->target = "{$this->dir}/bin/riven-engine";
    $this->io = new BufferIO;
});

afterEach(fn () => exec('rm -rf '.escapeshellarg($this->dir)));

it('is declared as the package Composer plugin', function () {
    $composer = json_decode(file_get_contents(__DIR__.'/../../composer.json'), true);

    expect($composer['type'])->toBe('composer-plugin')
        ->and($composer['extra']['class'])->toBe(Plugin::class)
        ->and($composer['require'])->toHaveKey('composer-plugin-api');
});

it('runs after composer install and update', function () {
    expect(Plugin::getSubscribedEvents())->toBe([
        ScriptEvents::POST_INSTALL_CMD => 'installEngine',
        ScriptEvents::POST_UPDATE_CMD => 'installEngine',
    ]);
});

it('installs the engine without a Laravel app', function () {
    [$name, $bytes] = fakeArchive($this->dir, 'linux', 'amd64');
    $http = staticHttp([
        RELEASES."/v0.1.0/{$name}" => new HttpResponse(200, $bytes),
        RELEASES.'/v0.1.0/checksums.txt' => new HttpResponse(200, hash('sha256', $bytes)."  {$name}\n"),
    ]);

    Plugin::run(fn () => new EngineInstaller($this->target, $http, RELEASES, 'linux', 'amd64', fn () => null), 'v0.1.0', $this->io);

    expect(file_get_contents($this->target))->toBe('fake engine 0.1.0')
        ->and($this->io->getOutput())->toContain('Installed riven-engine 0.1.0');
});

it('warns instead of failing composer when the download fails', function () {
    Plugin::run(fn () => new EngineInstaller($this->target, staticHttp([]), RELEASES, 'linux', 'amd64', fn () => null), '0.1.0', $this->io);

    expect($this->io->getOutput())
        ->toContain("couldn't install riven-engine")
        ->toContain('still a draft')
        ->toContain('php artisan riven:install-engine')
        ->and(file_exists($this->target))->toBeFalse();
});

it('warns when the platform has no build', function () {
    Plugin::run(fn () => throw new EngineInstallException('No riven-engine build for the riscv64 CPU'), '0.1.0', $this->io);

    expect($this->io->getOutput())->toContain('riscv64');
});

it('only explains for dev versions', function () {
    Plugin::run(fn () => new EngineInstaller($this->target, staticHttp([]), RELEASES, 'linux', 'amd64', fn () => null), 'dev-main', $this->io);

    expect($this->io->getOutput())->toContain('go build')->not->toContain('warning');
});
