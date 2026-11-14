<?php

namespace Riven\Composer;

use Composer\Downloader\TransportException;
use Composer\Util\HttpDownloader;
use Riven\Engine\EngineInstallException;
use Riven\Engine\HttpClient;
use Riven\Engine\HttpResponse;

/**
 * Downloads through Composer's own HTTP stack, so the user's proxy, CA bundle
 * and GitHub token settings apply exactly as they do for packages.
 */
final class ComposerHttpClient implements HttpClient
{
    public function __construct(private HttpDownloader $downloader) {}

    public function get(string $url): HttpResponse
    {
        try {
            $response = $this->downloader->get($url);

            return new HttpResponse($response->getStatusCode(), (string) $response->getBody());
        } catch (TransportException $e) {
            // Composer throws on 4xx/5xx; the installer wants those as statuses.
            if ($e->getStatusCode() !== null) {
                return new HttpResponse($e->getStatusCode(), (string) $e->getResponse());
            }

            throw new EngineInstallException("Couldn't reach {$url}: {$e->getMessage()}", previous: $e);
        }
    }
}
