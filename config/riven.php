<?php

return [

    /*
    |--------------------------------------------------------------------------
    | Route selection
    |--------------------------------------------------------------------------
    |
    | Patterns are matched against the route URI (no leading slash) with
    | Str::is(), so "*" is a wildcard: "api/*", "admin/*", "up".
    |
    | include: if non-empty, only matching routes are scanned (e.g. ['api/*']).
    | exclude: matching routes are always skipped, even if included.
    | exclude_vendor: skip routes whose closure or controller lives in vendor/
    |                 (routes registered by packages and the framework).
    |
    */

    'include' => [],

    'exclude' => [
        'up',
        '_ignition/*',
        'sanctum/*',
        'storage/*',
    ],

    'exclude_vendor' => true,

    /*
    |--------------------------------------------------------------------------
    | Manifest output
    |--------------------------------------------------------------------------
    |
    | Where riven:export writes the manifest read by the riven engine.
    | Relative paths are resolved from the application's base path.
    |
    */

    'output' => 'riven.json',

    /*
    |--------------------------------------------------------------------------
    | Engine binary
    |--------------------------------------------------------------------------
    |
    | riven:install-engine downloads riven-engine for this platform from the
    | GitHub release matching the installed faysal/riven version, verifies its
    | sha256 against the release's checksums.txt, and installs it here.
    |
    | path: null means vendor/bin/riven-engine (next to other Composer tools).
    | releases_url: change it only to use a mirror of the GitHub releases.
    |
    */

    'engine' => [
        'path' => null,
        'releases_url' => 'https://github.com/adnan4k/riven/releases/download',
    ],

];
