package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeEnv(m map[string]string) EnvLookup {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestResolve_ProfileDefaults(t *testing.T) {
	f := File{
		DefaultProfile: "work",
		Providers: map[string]Provider{
			"anthropic": {Type: "anthropic"},
		},
		Profiles: map[string]Profile{
			"work": {Provider: "anthropic", Model: "claude-opus-4-7"},
		},
	}
	r, err := Resolve(f, Overrides{}, fakeEnv(nil))
	require.NoError(t, err)
	assert.Equal(t, "work", r.Profile)
	assert.Equal(t, "anthropic", r.ProviderName)
	assert.Equal(t, "anthropic", r.ProviderType)
	assert.Equal(t, "claude-opus-4-7", r.Model)
}

func TestResolve_FlagOverridesEnvOverridesFile(t *testing.T) {
	f := File{
		DefaultProfile: "default",
		Providers:      map[string]Provider{"anthropic": {Type: "anthropic"}},
		Profiles:       map[string]Profile{"default": {Provider: "anthropic", Model: "file-model"}},
	}
	env := fakeEnv(map[string]string{"AI_CLI_MODEL": "env-model"})

	r, err := Resolve(f, Overrides{Model: "flag-model"}, env)
	require.NoError(t, err)
	assert.Equal(t, "flag-model", r.Model)

	r, err = Resolve(f, Overrides{}, env)
	require.NoError(t, err)
	assert.Equal(t, "env-model", r.Model)

	r, err = Resolve(f, Overrides{}, fakeEnv(nil))
	require.NoError(t, err)
	assert.Equal(t, "file-model", r.Model)
}

func TestResolve_APIKeyPrecedence(t *testing.T) {
	f := File{
		Providers: map[string]Provider{
			"claude": {Type: "anthropic"},
		},
		Profiles: map[string]Profile{
			"default": {Provider: "claude", Model: "m"},
		},
		DefaultProfile: "default",
	}

	// 1. AI_CLI_<NAME>_API_KEY wins.
	r, err := Resolve(f, Overrides{}, fakeEnv(map[string]string{
		"AI_CLI_CLAUDE_API_KEY": "prefixed",
		"ANTHROPIC_API_KEY":     "public",
	}))
	require.NoError(t, err)
	assert.Equal(t, "prefixed", r.APIKey)

	// 2. Fall back to public convention keyed by Type.
	r, err = Resolve(f, Overrides{}, fakeEnv(map[string]string{"ANTHROPIC_API_KEY": "public"}))
	require.NoError(t, err)
	assert.Equal(t, "public", r.APIKey)

	// 3. Gemini chain: AI_CLI_... > GEMINI_API_KEY > GOOGLE_API_KEY.
	f.Providers["g"] = Provider{Type: "gemini"}
	f.Profiles["default"] = Profile{Provider: "g", Model: "m"}
	r, err = Resolve(f, Overrides{}, fakeEnv(map[string]string{"GOOGLE_API_KEY": "g"}))
	require.NoError(t, err)
	assert.Equal(t, "g", r.APIKey)
}

func TestResolve_VerbatimModelString(t *testing.T) {
	f := File{
		DefaultProfile: "r",
		Providers: map[string]Provider{
			"bifrost":   {Type: "openai-compat", BaseURL: "https://b.example.com/v1"},
			"anthropic": {Type: "anthropic"},
		},
		Profiles: map[string]Profile{"r": {Provider: "bifrost", Model: "anthropic/claude-opus-4-7"}},
	}
	r, err := Resolve(f, Overrides{Model: "anthropic/claude-opus-4-7"}, fakeEnv(nil))
	require.NoError(t, err)
	assert.Equal(t, "bifrost", r.ProviderName)
	assert.Equal(t, "anthropic/claude-opus-4-7", r.Model, "slashes preserved verbatim")
}

func TestResolve_UnknownProfile(t *testing.T) {
	f := File{
		DefaultProfile: "default",
		Providers:      map[string]Provider{"anthropic": {Type: "anthropic"}},
		Profiles:       map[string]Profile{"default": {Provider: "anthropic", Model: "m"}},
	}
	_, err := Resolve(f, Overrides{Profile: "ghost"}, fakeEnv(nil))
	assert.ErrorIs(t, err, ErrUnknownProfile)
}
