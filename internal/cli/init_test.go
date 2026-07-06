package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tenxprotocols/ai-cli/internal/config"
	"github.com/tenxprotocols/ai-cli/internal/providers"
)

// runInitWith drives `ai init` with scripted answers against the given
// config path and returns the transcript. Model listing is stubbed out so
// tests stay off the network.
func runInitWith(t *testing.T, path, answers string) (string, error) {
	t.Helper()
	restore := listModelsQuietly
	listModelsQuietly = func(context.Context, string, string, string, string) []providers.ModelInfo { return nil }
	t.Cleanup(func() { listModelsQuietly = restore })
	cmd := NewRoot()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetIn(strings.NewReader(answers))
	cmd.SetArgs([]string{"--config", path, "init"})
	err := cmd.Execute()
	return out.String(), err
}

func TestInit_FreshConfigWithDetectedKey(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant")
	restore := ollamaProbe
	ollamaProbe = func() (string, bool) { return "", false }
	defer func() { ollamaProbe = restore }()

	path := filepath.Join(t.TempDir(), "sub", "config.toml") // parent dir must be created
	// Accept every default: provider (anthropic preselected via detected key),
	// model (live list fails against the real API in tests -> free text default),
	// profile name, and default-profile prompt.
	transcript, err := runInitWith(t, path, "\n\n\n\n")
	require.NoError(t, err)
	assert.Contains(t, transcript, "anthropic  (key detected)")

	file, err := config.LoadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "default", file.DefaultProfile)
	assert.Equal(t, "anthropic", file.Providers["anthropic"].Type)
	assert.Equal(t, "anthropic", file.Profiles["default"].Provider)
	assert.Equal(t, config.DefaultModel("anthropic"), file.Profiles["default"].Model)
}

func TestInit_CustomCompatEndpoint(t *testing.T) {
	clearProviderEnv(t)
	restore := ollamaProbe
	ollamaProbe = func() (string, bool) { return "", false }
	defer func() { ollamaProbe = restore }()

	path := filepath.Join(t.TempDir(), "config.toml")
	answers := strings.Join([]string{
		"6",                          // custom OpenAI-compatible endpoint
		"bifrost",                    // provider name
		"https://bifrost.example/v1", // base url
		"anthropic/claude-sonnet-5",  // model (free text; endpoint unreachable)
		"work",                       // profile name
		"y",                          // make default
	}, "\n") + "\n"
	transcript, err := runInitWith(t, path, answers)
	require.NoError(t, err)
	assert.Contains(t, transcript, "AI_CLI_BIFROST_API_KEY")

	file, err := config.LoadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "work", file.DefaultProfile)
	assert.Equal(t, "openai-compat", file.Providers["bifrost"].Type)
	assert.Equal(t, "https://bifrost.example/v1", file.Providers["bifrost"].BaseURL)
	assert.Equal(t, "anthropic/claude-sonnet-5", file.Profiles["work"].Model)
}

func TestInit_MergesIntoExistingConfig(t *testing.T) {
	clearProviderEnv(t)
	restore := ollamaProbe
	ollamaProbe = func() (string, bool) { return "llama3.1:8b", true }
	defer func() { ollamaProbe = restore }()

	path := writeTempConfig(t, fakeConfig) // has provider "fake" + profile "default"
	answers := strings.Join([]string{
		"5",     // ollama (running)
		"",      // model -> detected llama3.1:8b (live list fails; free text default)
		"local", // profile name
		"n",     // keep existing default profile
	}, "\n") + "\n"
	_, err := runInitWith(t, path, answers)
	require.NoError(t, err)

	file, err := config.LoadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "default", file.DefaultProfile, "existing default kept")
	assert.Equal(t, "openai-compat", file.Providers["fake"].Type, "existing provider preserved")
	assert.Equal(t, "fake", file.Profiles["default"].Provider, "existing profile preserved")
	assert.Equal(t, "llama3.1:8b", file.Profiles["local"].Model)
}
