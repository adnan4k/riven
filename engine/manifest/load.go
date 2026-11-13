package manifest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrNotManifest means the input is JSON but has no schema_version, so it
// isn't something riven:export wrote.
var ErrNotManifest = errors.New("not a Riven manifest (no schema_version)")

// VersionError means the manifest uses a schema version this engine can't read.
type VersionError struct {
	Got int
}

func (e *VersionError) Error() string {
	if e.Got > SupportedVersion {
		return fmt.Sprintf("manifest is schema v%d, but this riven-engine only supports v%d: upgrade riven-engine",
			e.Got, SupportedVersion)
	}

	return fmt.Sprintf("manifest is schema v%d, but this riven-engine only supports v%d: re-run `php artisan riven:export`",
		e.Got, SupportedVersion)
}

// Load reads and parses the manifest at path.
func Load(path string) (*Manifest, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s not found: run `php artisan riven:export` in the Laravel app first: %w", path, err)
	}
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer f.Close()

	m, err := Parse(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return m, nil
}

// Parse reads a manifest, checking schema_version before decoding the rest
// so an incompatible file fails with a version error, not a confusing type error.
func Parse(r io.Reader) (*Manifest, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var probe struct {
		SchemaVersion *int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if probe.SchemaVersion == nil {
		return nil, ErrNotManifest
	}
	if *probe.SchemaVersion != SupportedVersion {
		return nil, &VersionError{Got: *probe.SchemaVersion}
	}

	var m Manifest
	// Unknown fields are allowed on purpose: additive schema changes don't
	// bump the version, so a newer collector must still load here.
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}

	return &m, nil
}
