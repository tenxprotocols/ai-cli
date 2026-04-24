package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

// LoadFile reads a TOML config. A missing file is not an error — an empty
// but initialized File is returned.
func LoadFile(path string) (File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return File{
				Providers: map[string]Provider{},
				Profiles:  map[string]Profile{},
			}, nil
		}
		return File{}, fmt.Errorf("read %s: %w", path, err)
	}

	var f File
	if err := toml.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if f.Providers == nil {
		f.Providers = map[string]Provider{}
	}
	if f.Profiles == nil {
		f.Profiles = map[string]Profile{}
	}
	return f, nil
}

// SaveFile writes the TOML back. Used by `ai config set`.
func SaveFile(path string, f File) error {
	data, err := toml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

// DefaultPath returns the platform-appropriate default config file path.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/ai-cli/config.toml", dir), nil
}
