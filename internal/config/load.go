package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

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

// GetKey returns a value from the TOML tree by dotted path. Only known paths
// are supported.
func GetKey(f File, key string) (string, error) {
	switch key {
	case "default_profile":
		return f.DefaultProfile, nil
	}
	if n, field, ok := splitThree(key, "providers."); ok {
		p, ok := f.Providers[n]
		if !ok {
			return "", fmt.Errorf("unknown provider: %s", n)
		}
		switch field {
		case "type":
			return p.Type, nil
		case "api_key":
			return p.APIKey, nil
		case "base_url":
			return p.BaseURL, nil
		}
	}
	if n, field, ok := splitThree(key, "profiles."); ok {
		p, ok := f.Profiles[n]
		if !ok {
			return "", fmt.Errorf("unknown profile: %s", n)
		}
		switch field {
		case "provider":
			return p.Provider, nil
		case "model":
			return p.Model, nil
		case "system":
			return p.System, nil
		}
	}
	return "", fmt.Errorf("unsupported key: %s", key)
}

// SetKey writes a value by dotted path. Creates parents as needed.
func SetKey(f *File, key, val string) error {
	switch key {
	case "default_profile":
		f.DefaultProfile = val
		return nil
	}
	if n, field, ok := splitThree(key, "providers."); ok {
		p := f.Providers[n]
		switch field {
		case "type":
			p.Type = val
		case "api_key":
			p.APIKey = val
		case "base_url":
			p.BaseURL = val
		default:
			return fmt.Errorf("unsupported key: %s", key)
		}
		if f.Providers == nil {
			f.Providers = map[string]Provider{}
		}
		f.Providers[n] = p
		return nil
	}
	if n, field, ok := splitThree(key, "profiles."); ok {
		p := f.Profiles[n]
		switch field {
		case "provider":
			p.Provider = val
		case "model":
			p.Model = val
		case "system":
			p.System = val
		default:
			return fmt.Errorf("unsupported key: %s", key)
		}
		if f.Profiles == nil {
			f.Profiles = map[string]Profile{}
		}
		f.Profiles[n] = p
		return nil
	}
	return fmt.Errorf("unsupported key: %s", key)
}

// splitThree splits "prefix.<name>.<field>" into (name, field, ok).
func splitThree(s, prefix string) (string, string, bool) {
	if !strings.HasPrefix(s, prefix) {
		return "", "", false
	}
	rest := s[len(prefix):]
	dot := strings.IndexByte(rest, '.')
	if dot < 0 {
		return "", "", false
	}
	return rest[:dot], rest[dot+1:], true
}
