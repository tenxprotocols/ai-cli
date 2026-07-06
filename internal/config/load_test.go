package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleConfig = `
default_profile = "default"

[providers.anthropic]
type = "anthropic"

[providers.openai]
type = "openai"

[providers.ollama]
type = "openai-compat"
base_url = "http://localhost:11434/v1"

[profiles.default]
provider = "anthropic"
model    = "claude-sonnet-4-6"

[profiles.work]
provider = "anthropic"
model    = "claude-opus-4-7"
system   = "be terse"
temperature = 0.2
`

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(p, []byte(sampleConfig), 0o600))

	cfg, err := LoadFile(p)
	require.NoError(t, err)

	assert.Equal(t, "default", cfg.DefaultProfile)
	require.Contains(t, cfg.Providers, "ollama")
	assert.Equal(t, "openai-compat", cfg.Providers["ollama"].Type)
	assert.Equal(t, "http://localhost:11434/v1", cfg.Providers["ollama"].BaseURL)

	require.Contains(t, cfg.Profiles, "work")
	work := cfg.Profiles["work"]
	assert.Equal(t, "anthropic", work.Provider)
	assert.Equal(t, "claude-opus-4-7", work.Model)
	assert.Equal(t, "be terse", work.System)
	require.NotNil(t, work.Temperature)
	assert.InDelta(t, 0.2, *work.Temperature, 0.0001)
}

func TestLoadFile_Missing(t *testing.T) {
	cfg, err := LoadFile(filepath.Join(t.TempDir(), "no-such.toml"))
	require.NoError(t, err)
	assert.NotNil(t, cfg.Providers)
	assert.NotNil(t, cfg.Profiles)
}
