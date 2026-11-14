<?php

namespace Riven\Engine;

/**
 * The one HTTP call the installer needs. It's an interface because the
 * installer runs in two places: inside Composer (as a plugin, no Laravel app
 * yet) and from artisan. Each side passes its own client.
 */
interface HttpClient
{
    /**
     * Returns any HTTP status (including 404/500) as a response; throws
     * EngineInstallException only when the server couldn't be reached.
     */
    public function get(string $url): HttpResponse;
}
