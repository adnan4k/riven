<?php

use Riven\Tests\DashboardTestCase;
use Riven\Tests\TestCase;

pest()->extend(TestCase::class)->in('Feature', 'Unit');
pest()->extend(DashboardTestCase::class)->in('Dashboard');

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
