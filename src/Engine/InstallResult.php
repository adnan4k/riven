<?php

namespace Riven\Engine;

final class InstallResult
{
    private function __construct(
        public readonly bool $installed,
        public readonly string $message,
        public readonly ?string $version = null,
    ) {}

    public static function installed(string $version, string $path): self
    {
        return new self(true, "Installed riven-engine {$version} at {$path}.", $version);
    }

    public static function skipped(string $reason): self
    {
        return new self(false, $reason);
    }
}
