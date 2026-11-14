<?php

namespace Riven\Engine;

use Closure;
use Composer\InstalledVersions;
use FilesystemIterator;
use PharData;
use RecursiveDirectoryIterator;
use RecursiveIteratorIterator;
use Throwable;

/**
 * Downloads the riven-engine binary that matches the installed faysal/riven
 * version from its GitHub release, verifies it against the release's
 * checksums.txt, and installs it (by default vendor/bin/riven-engine).
 *
 * Archive names follow .goreleaser.yaml:
 *   riven-engine_<version>_<os>_<arch>.tar.gz (.zip on Windows)
 *
 * Plain PHP on purpose (no Laravel facades): the Composer plugin runs it
 * inside Composer's process, where no Laravel app exists.
 */
class EngineInstaller
{
    public const PACKAGE = 'faysal/riven';

    public const RELEASES_URL = 'https://github.com/adnan4k/riven/releases/download';

    /**
     * @param  ?Closure(string): ?string  $versionProbe  runs `<binary> --version`, returns its output (null on failure)
     */
    public function __construct(
        private string $target,
        private HttpClient $http,
        private string $releasesUrl = self::RELEASES_URL,
        private ?string $os = null,
        private ?string $arch = null,
        private ?Closure $versionProbe = null,
    ) {
        $this->os ??= self::detectOs();
        $this->arch ??= self::detectArch();
        $this->versionProbe ??= self::runVersion(...);
    }

    /**
     * @param  ?string  $version  e.g. "0.1.0" or "v0.1.0"; default: the installed package version
     */
    public function install(?string $version = null, bool $force = false): InstallResult
    {
        $version ??= self::installedVersion();
        $version = $version === null ? null : ltrim($version, 'v');

        if ($version === null || ! preg_match('/^\d+\.\d+\.\d+([-+][0-9A-Za-z.-]+)?$/', $version)) {
            return InstallResult::skipped(
                'faysal/riven is installed as '.($version ?? 'an unknown version').', which has no release binary. '
                .'Build the engine from source instead, in the faysal/riven repository (github.com/adnan4k/riven): cd engine && go build -o '.$this->target.' ./cmd/riven-engine'
            );
        }

        if (! $force && $this->installedEngineVersion() === $version) {
            return InstallResult::skipped("riven-engine {$version} is already installed at {$this->target}.");
        }

        $archive = $this->archiveName($version);
        $base = rtrim($this->releasesUrl, '/')."/v{$version}";
        $tmp = sys_get_temp_dir().'/riven-engine-'.bin2hex(random_bytes(6));
        self::ensureDirectory($tmp);

        try {
            $archivePath = "{$tmp}/{$archive}";
            $this->download("{$base}/{$archive}", $archivePath);
            $this->verify($archivePath, $archive, $this->fetch("{$base}/checksums.txt"));
            $binary = $this->extract($archivePath, "{$tmp}/extracted");

            self::ensureDirectory(dirname($this->target));
            // Copy next to the target, then rename: replacing the binary is
            // atomic, so a failure never leaves a half-written engine behind.
            $staging = $this->target.'.download';
            if (! @copy($binary, $staging) || ! chmod($staging, 0755) || ! @rename($staging, $this->target)) {
                @unlink($staging);
                throw new EngineInstallException("Couldn't write {$this->target}. Is that directory writable by you?");
            }
        } finally {
            self::deleteDirectory($tmp);
        }

        return InstallResult::installed($version, $this->target);
    }

    public function archiveName(string $version): string
    {
        $ext = $this->os === 'windows' ? 'zip' : 'tar.gz';

        return "riven-engine_{$version}_{$this->os}_{$this->arch}.{$ext}";
    }

    public function binaryName(): string
    {
        return $this->os === 'windows' ? 'riven-engine.exe' : 'riven-engine';
    }

    /**
     * The version of the engine already at the target, from `riven-engine --version`.
     */
    public function installedEngineVersion(): ?string
    {
        if (! is_file($this->target)) {
            return null;
        }

        $output = ($this->versionProbe)($this->target);

        return $output !== null && preg_match('/version v?(\S+)/', $output, $m) ? $m[1] : null;
    }

    public static function installedVersion(): ?string
    {
        return InstalledVersions::isInstalled(self::PACKAGE) ? InstalledVersions::getPrettyVersion(self::PACKAGE) : null;
    }

    public static function detectOs(): string
    {
        return match (PHP_OS_FAMILY) {
            'Linux' => 'linux',
            'Darwin' => 'darwin',
            'Windows' => 'windows',
            default => throw new EngineInstallException('No riven-engine build for '.PHP_OS_FAMILY.': supported are Linux, macOS and Windows.'),
        };
    }

    public static function detectArch(?string $machine = null): string
    {
        $machine = strtolower($machine ?? php_uname('m'));

        return match ($machine) {
            'x86_64', 'amd64', 'x64' => 'amd64',
            'aarch64', 'arm64', 'armv8', 'armv8l' => 'arm64',
            default => throw new EngineInstallException("No riven-engine build for the {$machine} CPU: supported are amd64 and arm64."),
        };
    }

    private function download(string $url, string $path): void
    {
        $response = $this->http->get($url);

        if ($response->status === 404) {
            throw new EngineInstallException(
                "No release file at {$url}. Either that version has no {$this->os}/{$this->arch} build, "
                .'or its GitHub release is still a draft (see '.preg_replace('#/download$#', '', rtrim($this->releasesUrl, '/')).').'
            );
        }
        if (! $response->successful()) {
            throw new EngineInstallException("Downloading {$url} failed with HTTP {$response->status}.");
        }

        if (file_put_contents($path, $response->body) === false) {
            throw new EngineInstallException("Couldn't save the download to {$path}.");
        }
    }

    private function fetch(string $url): string
    {
        $response = $this->http->get($url);

        if (! $response->successful()) {
            throw new EngineInstallException("Downloading {$url} failed with HTTP {$response->status}.");
        }

        return $response->body;
    }

    /**
     * checksums.txt lines are "<sha256>  <file name>" (sha256sum format).
     */
    private function verify(string $path, string $name, string $checksums): void
    {
        $expected = null;
        foreach (preg_split('/\R/', trim($checksums)) as $line) {
            $parts = preg_split('/\s+/', trim($line));
            if (count($parts) === 2 && ltrim($parts[1], '*') === $name) {
                $expected = strtolower($parts[0]);
            }
        }

        if ($expected === null) {
            throw new EngineInstallException("checksums.txt has no entry for {$name}; refusing to install an unverified binary.");
        }

        $actual = hash_file('sha256', $path);
        if (! hash_equals($expected, $actual)) {
            throw new EngineInstallException("Checksum mismatch for {$name} (expected {$expected}, got {$actual}). The download was corrupted or tampered with; nothing was installed.");
        }
    }

    private function extract(string $archive, string $dir): string
    {
        try {
            (new PharData($archive))->extractTo($dir, $this->binaryName(), true);
        } catch (Throwable $e) {
            throw new EngineInstallException("Couldn't unpack {$this->binaryName()} from ".basename($archive).": {$e->getMessage()}", previous: $e);
        }

        $binary = "{$dir}/{$this->binaryName()}";
        if (! is_file($binary)) {
            throw new EngineInstallException(basename($archive)." doesn't contain {$this->binaryName()}.");
        }

        return $binary;
    }

    private static function runVersion(string $binary): ?string
    {
        $process = @proc_open([$binary, '--version'], [1 => ['pipe', 'w'], 2 => ['pipe', 'w']], $pipes);
        if (! is_resource($process)) {
            return null;
        }

        $output = stream_get_contents($pipes[1]);
        fclose($pipes[1]);
        fclose($pipes[2]);

        return proc_close($process) === 0 ? $output : null;
    }

    private static function ensureDirectory(string $dir): void
    {
        if (! is_dir($dir) && ! @mkdir($dir, 0755, true) && ! is_dir($dir)) {
            throw new EngineInstallException("Couldn't create {$dir}. Is it writable by you?");
        }
    }

    private static function deleteDirectory(string $dir): void
    {
        if (! is_dir($dir)) {
            return;
        }

        $items = new RecursiveIteratorIterator(
            new RecursiveDirectoryIterator($dir, FilesystemIterator::SKIP_DOTS),
            RecursiveIteratorIterator::CHILD_FIRST,
        );
        foreach ($items as $item) {
            $item->isDir() && ! $item->isLink() ? @rmdir($item->getPathname()) : @unlink($item->getPathname());
        }
        @rmdir($dir);
    }
}
