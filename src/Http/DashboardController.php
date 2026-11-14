<?php

namespace Riven\Http;

use Illuminate\Http\Response;
use Riven\Dashboard\DashboardException;
use Riven\Dashboard\Engine;
use Riven\Dashboard\Ui;
use Symfony\Component\HttpFoundation\BinaryFileResponse;

/**
 * Serves the dashboard's files: assets as they are, index.html for every
 * other path (client-side routes like /riven/routes/...).
 */
class DashboardController
{
    public function __invoke(Ui $ui, Engine $engine, ?string $path = null): Response|BinaryFileResponse
    {
        if (! $engine->exists()) {
            return $this->page(503, "riven-engine isn't installed",
                'Expected it at <code>'.e($engine->path()).'</code>. Run <code>composer install</code>, or <code>php artisan riven:install-engine</code> to retry the download.');
        }

        try {
            $file = $ui->file((string) $path);
            if ($file !== null) {
                $response = new BinaryFileResponse($file, 200, ['Content-Type' => Ui::type($file)]);
                // Vite puts a content hash in asset names, so they can be cached for good.
                if (str_starts_with((string) $path, 'assets/')) {
                    $response->headers->set('Cache-Control', 'public, max-age=31536000, immutable');
                }

                return $response;
            }

            return response($ui->index(config('riven.dashboard.path')), 200, [
                'Content-Type' => Ui::TYPES['html'],
                'Cache-Control' => 'no-store',
            ]);
        } catch (DashboardException $e) {
            return $this->page(500, "The dashboard couldn't be loaded", e($e->getMessage()));
        }
    }

    private function page(int $status, string $title, string $html): Response
    {
        return response(<<<HTML
            <!doctype html><meta charset="utf-8"><title>Riven</title>
            <body style="font-family:system-ui;max-width:40rem;margin:4rem auto;line-height:1.5">
            <h1>{$title}</h1><p>{$html}</p>
            HTML, $status, ['Content-Type' => Ui::TYPES['html']]);
    }
}
