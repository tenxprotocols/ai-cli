package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func envWith(vars map[string]string) EnvLookup {
	return func(k string) (string, bool) {
		v, ok := vars[k]
		return v, ok
	}
}

func noOllama() (string, bool) { return "", false }

func TestZeroConfig_FirstKnownKeyWins(t *testing.T) {
	env := envWith(map[string]string{
		"OPENAI_API_KEY":    "sk-openai",
		"ANTHROPIC_API_KEY": "sk-ant",
	})
	resolved, ok := ZeroConfig(Overrides{}, env, noOllama)
	require.True(t, ok)
	assert.Equal(t, "anthropic", resolved.ProviderName)
	assert.Equal(t, "sk-ant", resolved.APIKey)
	assert.Equal(t, "claude-sonnet-5", resolved.Model)
	assert.Equal(t, "zero-config", resolved.Profile)
}

func TestZeroConfig_FallsBackToOllama(t *testing.T) {
	resolved, ok := ZeroConfig(Overrides{}, envWith(nil), func() (string, bool) { return "llama3.1:8b", true })
	require.True(t, ok)
	assert.Equal(t, "ollama", resolved.ProviderName)
	assert.Equal(t, "openai-compat", resolved.ProviderType)
	assert.Equal(t, "http://localhost:11434/v1", resolved.BaseURL)
	assert.Equal(t, "llama3.1:8b", resolved.Model)
	assert.Empty(t, resolved.APIKey)
}

func TestZeroConfig_NothingAvailable(t *testing.T) {
	_, ok := ZeroConfig(Overrides{}, envWith(nil), noOllama)
	assert.False(t, ok)
}

func TestZeroConfig_OverridesStillApply(t *testing.T) {
	env := envWith(map[string]string{"GEMINI_API_KEY": "g-key"})
	resolved, ok := ZeroConfig(Overrides{Model: "gemini-2.5-pro", System: "terse"}, env, noOllama)
	require.True(t, ok)
	assert.Equal(t, "gemini", resolved.ProviderName)
	assert.Equal(t, "gemini-2.5-pro", resolved.Model)
	assert.Equal(t, "terse", resolved.System)
}
