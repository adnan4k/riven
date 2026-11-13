<?php

// The Go engine's tests run against this committed export. Validating it here
// keeps the two halves honest: a schema change fails until the sample is re-exported.
it('keeps the engine sample manifest valid against the schema', function () {
    $sample = json_decode(
        file_get_contents(__DIR__.'/../../engine/testdata/riven-app.json'),
        false,
        flags: JSON_THROW_ON_ERROR,
    );

    expect(schemaErrors($sample))->toBe([]);
});
