<?php

use Illuminate\Support\Facades\File;
use Illuminate\Support\Facades\Http;
use Illuminate\Support\Facades\Process;
use Riven\Engine\EngineInstaller;
use Riven\Engine\EngineInstallException;

const RELEASES = 'https://releases.test/download';

/**
 * Builds a release archive the way GoReleaser does, with a fake binary inside.
 * Returns [file name, bytes].
 */
function fakeArchive(string $dir, string $os, string $arch, string $version = '0.1.0'): array
{
    $binary = $os === 'windows' ? 'riven-engine.exe' : 'riven-engine';
    $base = "{$dir}/riven-engine_{$version}_{$os}_{$arch}";

    if ($os === 'windows') {
        $zip = new PharData("{$base}.zip", 0, null, Phar::ZIP);
        $zip->addFromString($binary, "fake engine {$version}");
        $path = "{$base}.zip";
    } else {
        $tar = new PharData("{$base}.tar");
        $tar->addFromString($binary, "fake engine {$version}");
        $tar->addFromString('README.md', 'readme');
        $tar->compress(Phar::GZ);
        $path = "{$base}.tar.gz";
    }

    return [basename($path), file_get_contents($path)];
}

/** Fakes a GitHub release for v0.1.0 with the given archive and checksums. */
function fakeRelease(string $name, string $bytes, ?string $checksums = null): void
{
    $checksums ??= hash('sha256', $bytes)."  {$name}\n".str_repeat('0', 64)."  riven-engine_0.1.0_other_amd64.tar.gz\n";

    Http::fake([
        RELEASES."/v0.1.0/{$name}" => Http::response($bytes),
        RELEASES.'/v0.1.0/checksums.txt' => Http::response($checksums),
        '*' => Http::response('not found', 404),
    ]);
}

beforeEach(function () {
    $this->dir = sys_get_temp_dir().'/riven-install-'.uniqid();
    File::ensureDirectoryExists($this->dir);
    $this->target = "{$this->dir}/bin/riven-engine";
    Process::fake(['*' => Process::result('', 'not found', 127)]);
});

afterEach(fn () => File::deleteDirectory($this->dir));

it('maps the platform to the GoReleaser archive name', function (string $os, string $arch, string $name) {
    expect((new EngineInstaller('x', RELEASES, $os, $arch))->archiveName('0.1.0'))->toBe($name);
})->with([
    ['linux', 'amd64', 'riven-engine_0.1.0_linux_amd64.tar.gz'],
    ['darwin', 'arm64', 'riven-engine_0.1.0_darwin_arm64.tar.gz'],
    ['windows', 'amd64', 'riven-engine_0.1.0_windows_amd64.zip'],
]);

it('detects the CPU architecture', function (string $machine, string $arch) {
    expect(EngineInstaller::detectArch($machine))->toBe($arch);
})->with([['x86_64', 'amd64'], ['AMD64', 'amd64'], ['aarch64', 'arm64'], ['arm64', 'arm64']]);

it('rejects unsupported CPUs', function () {
    EngineInstaller::detectArch('riscv64');
})->throws(EngineInstallException::class, 'riscv64');

it('downloads, verifies and installs the binary', function () {
    [$name, $bytes] = fakeArchive($this->dir, 'linux', 'amd64');
    fakeRelease($name, $bytes);

    $result = (new EngineInstaller($this->target, RELEASES, 'linux', 'amd64'))->install('v0.1.0');

    expect($result->installed)->toBeTrue()
        ->and($result->version)->toBe('0.1.0')
        ->and(file_get_contents($this->target))->toBe('fake engine 0.1.0')
        ->and(is_executable($this->target))->toBeTrue()
        ->and(File::exists($this->target.'.download'))->toBeFalse();
    Http::assertSent(fn ($req) => $req->url() === RELEASES."/v0.1.0/{$name}");
});

it('installs from a Windows zip', function () {
    [$name, $bytes] = fakeArchive($this->dir, 'windows', 'amd64');
    fakeRelease($name, $bytes);
    $target = "{$this->dir}/bin/riven-engine.exe";

    (new EngineInstaller($target, RELEASES, 'windows', 'amd64'))->install('0.1.0');

    expect(file_get_contents($target))->toBe('fake engine 0.1.0');
});

it('refuses a binary whose checksum does not match', function () {
    [$name, $bytes] = fakeArchive($this->dir, 'linux', 'amd64');
    fakeRelease($name, $bytes, str_repeat('a', 64)."  {$name}\n");

    expect(fn () => (new EngineInstaller($this->target, RELEASES, 'linux', 'amd64'))->install('0.1.0'))
        ->toThrow(EngineInstallException::class, 'Checksum mismatch');
    expect(File::exists($this->target))->toBeFalse();
});

it('refuses a binary missing from checksums.txt', function () {
    [$name, $bytes] = fakeArchive($this->dir, 'linux', 'amd64');
    fakeRelease($name, $bytes, str_repeat('a', 64)."  something-else.tar.gz\n");

    expect(fn () => (new EngineInstaller($this->target, RELEASES, 'linux', 'amd64'))->install('0.1.0'))
        ->toThrow(EngineInstallException::class, 'no entry for');
    expect(File::exists($this->target))->toBeFalse();
});

it('keeps the old binary when a download fails', function () {
    File::ensureDirectoryExists(dirname($this->target));
    File::put($this->target, 'old engine');
    Http::fake(['*' => Http::response('not found', 404)]);

    expect(fn () => (new EngineInstaller($this->target, RELEASES, 'linux', 'amd64'))->install('0.1.0'))
        ->toThrow(EngineInstallException::class, 'No release file at '.RELEASES.'/v0.1.0/riven-engine_0.1.0_linux_amd64.tar.gz');
    expect(file_get_contents($this->target))->toBe('old engine');
});

it('skips dev versions, which have no release', function () {
    Http::fake();

    $result = (new EngineInstaller($this->target, RELEASES, 'linux', 'amd64'))->install('dev-main');

    expect($result->installed)->toBeFalse()
        ->and($result->message)->toContain('dev-main')->toContain('go build');
    Http::assertNothingSent();
});

it('skips when the same version is already installed, unless forced', function () {
    File::ensureDirectoryExists(dirname($this->target));
    File::put($this->target, 'current');
    Process::fake(['*' => Process::result("riven-engine version 0.1.0\n")]);
    [$name, $bytes] = fakeArchive($this->dir, 'linux', 'amd64');
    fakeRelease($name, $bytes);
    $installer = new EngineInstaller($this->target, RELEASES, 'linux', 'amd64');

    expect($installer->install('0.1.0')->installed)->toBeFalse();
    Http::assertNothingSent();

    expect($installer->install('0.1.0', force: true)->installed)->toBeTrue()
        ->and(file_get_contents($this->target))->toBe('fake engine 0.1.0');
});

it('upgrades when a different version is installed', function () {
    File::ensureDirectoryExists(dirname($this->target));
    File::put($this->target, 'older');
    Process::fake(['*' => Process::result("riven-engine version 0.0.9\n")]);
    [$name, $bytes] = fakeArchive($this->dir, 'linux', 'amd64');
    fakeRelease($name, $bytes);

    expect((new EngineInstaller($this->target, RELEASES, 'linux', 'amd64'))->install('0.1.0')->installed)->toBeTrue();
});

it('runs from artisan with the configured path and mirror', function () {
    config(['riven.engine.path' => $this->target, 'riven.engine.releases_url' => RELEASES]);
    $os = EngineInstaller::detectOs();
    $arch = EngineInstaller::detectArch();
    [$name, $bytes] = fakeArchive($this->dir, $os, $arch);
    fakeRelease($name, $bytes);

    $this->artisan('riven:install-engine', ['--engine-version' => '0.1.0'])
        ->expectsOutputToContain('Installed riven-engine 0.1.0')
        ->assertSuccessful();
});

it('fails with the reason from artisan', function () {
    config(['riven.engine.path' => $this->target, 'riven.engine.releases_url' => RELEASES]);
    Http::fake(['*' => Http::response('', 500)]);

    $this->artisan('riven:install-engine', ['--engine-version' => '0.1.0'])
        ->expectsOutputToContain('failed with HTTP 500')
        ->assertFailed();
});
