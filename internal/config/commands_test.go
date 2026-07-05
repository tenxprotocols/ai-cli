package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve_CommandBlockOverlaysProfile(t *testing.T) {
	file := File{
		DefaultProfile: "default",
		Providers: map[string]Provider{
			"anthropic": {Type: "anthropic"},
			"ollama":    {Type: "openai-compat", BaseURL: "http://localhost:11434/v1"},
		},
		Profiles: map[string]Profile{
			"default": {Provider: "anthropic", Model: "big-model"},
		},
		Commands: map[string]Profile{
			"shell": {Model: "fast-model"},
			"ask":   {Provider: "ollama", Model: "local-model"},
		},
	}
	noEnv := func(string) (string, bool) { return "", false }

	shell, err := Resolve(file, Overrides{Command: "shell"}, noEnv)
	require.NoError(t, err)
	assert.Equal(t, "fast-model", shell.Model)
	assert.Equal(t, "anthropic", shell.ProviderName, "unset fields fall through to the profile")

	ask, err := Resolve(file, Overrides{Command: "ask"}, noEnv)
	require.NoError(t, err)
	assert.Equal(t, "ollama", ask.ProviderName, "command block may switch provider")
	assert.Equal(t, "local-model", ask.Model)

	models, err := Resolve(file, Overrides{Command: "models"}, noEnv)
	require.NoError(t, err)
	assert.Equal(t, "big-model", models.Model, "commands without a block use the profile")

	flag, err := Resolve(file, Overrides{Command: "shell", Model: "flag-model"}, noEnv)
	require.NoError(t, err)
	assert.Equal(t, "flag-model", flag.Model, "flags beat command blocks")
}
