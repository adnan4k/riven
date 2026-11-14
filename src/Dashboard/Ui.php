<?php

namespace Riven\Dashboard;

use Illuminate\Support\Facades\File;

/**
 * The dashboard's built files. They are compiled into riven-engine, so the
 * first visit writes them to storage (one folder per binary version), and
 * Laravel serves them from there.
 */
class Ui
{
    public const TYPES = [
        'html' => 'text/html; charset=utf-8',
        'js' => 'text/javascript; charset=utf-8', // module scripts are refused with any other type
        'css' => 'text/css; charset=utf-8',
        'json' => 'application/json',
        'svg' => 'image/svg+xml',
        'png' => 'image/png',
        'ico' => 'image/x-icon',
        'woff2' => 'font/woff2',
        'woff' => 'font/woff',
    ];

    public function __construct(private Engine $engine, private string $storage) {}

    public function directory(): string
    {
        $dir = $this->storage.'/ui-'.$this->engine->fingerprint();
        if (is_file($dir.'/index.html')) {
            return $dir;
        }

        // Extract next to it, then rename: two first visits at once can't
        // serve a half-written folder.
        $tmp = $dir.'.tmp-'.bin2hex(random_bytes(4));
        File::ensureDirectoryExists($tmp);
        try {
            $this->engine->extractUi($tmp);
            if (! @rename($tmp, $dir) && ! is_file($dir.'/index.html')) {
                throw new DashboardException("Couldn't write the dashboard files to {$dir}.");
            }
        } finally {
            File::deleteDirectory($tmp);
        }

        return $dir;
    }

    /**
     * index.html, told where it's mounted (e.g. "/riven/").
     */
    public function index(string $mountPath): string
    {
        $html = file_get_contents($this->directory().'/index.html');
        $base = '/'.trim($mountPath, '/').'/';

        return str_replace('<base href="/" />', '<base href="'.htmlspecialchars($base === '//' ? '/' : $base).'" />', $html);
    }

    /**
     * The file for a URL path inside the dashboard, or null (a client-side route).
     */
    public function file(string $path): ?string
    {
        if ($path === '' || str_contains($path, '..') || str_contains($path, "\0")) {
            return null;
        }
        $dir = realpath($this->directory());
        $file = realpath($dir.'/'.$path);

        // realpath + prefix check: nothing outside the UI folder, symlinks included.
        return $file !== false && is_file($file) && str_starts_with($file, $dir.DIRECTORY_SEPARATOR) ? $file : null;
    }

    public static function type(string $file): string
    {
        return self::TYPES[strtolower(pathinfo($file, PATHINFO_EXTENSION))] ?? 'application/octet-stream';
    }
}
