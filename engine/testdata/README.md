# Engine test fixtures

`riven-app.json` is a real `php artisan riven:export` from the riven-app host
app. It's the shared sample manifest for engine tests (manifest loader, golden
CLI output). A PHP test (`tests/Feature/SampleManifestTest.php`) validates it
against `schema/riven.schema.json`, so a schema change fails until it's re-exported:

    cd ../riven-app && php artisan riven:export --output=../riven/engine/testdata/riven-app.json

Then refresh the golden files: `go test ./internal/cli -update` (from `engine/`).
