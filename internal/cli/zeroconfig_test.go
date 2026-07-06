package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearProviderEnv blanks every env var zero-config consults.
func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY",
		"OPENROUTER_API_KEY", "AI_CLI_PROFILE", "AI_CLI_PROVIDER", "AI_CLI_MODEL", "AI_CLI_SYSTEM",
	} {
		t.Setenv(k, "")
	}
}

// runWithoutConfig executes the CLI pointing at a nonexistent config file.
func runWithoutConfig(t *testing.T, fake *scriptedProvider, args ...string) (string, error) {
	t.Helper()
	missing := filepath.Join(t.TempDir(), "nope.toml")
	cmd := NewRoot()
	ctx := context.Background()
	if fake != nil {
		ctx = WithInjectedProvider(ctx, fake.name, fake)
	}
	cmd.SetContext(ctx)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--config", missing}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestZeroConfig_AskWorksWithEnvKeyOnly(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant")
	fake := &scriptedProvider{name: "anthropic", reply: "hello"}

	out, err := runWithoutConfig(t, fake, "ask", "hi")
	require.NoError(t, err)
	assert.Equal(t, "hello\n", out)
	assert.Equal(t, "claude-sonnet-5", fake.lastReq.Model)
}

func TestZeroConfig_ModelFlagStillWins(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant")
	fake := &scriptedProvider{name: "anthropic", reply: "ok"}

	_, err := runWithoutConfig(t, fake, "--model", "claude-haiku-4-5", "ask", "hi")
	require.NoError(t, err)
	assert.Equal(t, "claude-haiku-4-5", fake.lastReq.Model)
}

func TestZeroConfig_OllamaFallback(t *testing.T) {
	clearProviderEnv(t)
	restore := ollamaProbe
	ollamaProbe = func() (string, bool) { return "llama3.1:8b", true }
	defer func() { ollamaProbe = restore }()
	fake := &scriptedProvider{name: "ollama", reply: "local"}

	out, err := runWithoutConfig(t, fake, "ask", "hi")
	require.NoError(t, err)
	assert.Equal(t, "local\n", out)
	assert.Equal(t, "llama3.1:8b", fake.lastReq.Model)
}

func TestZeroConfig_HelpfulErrorWhenNothingFound(t *testing.T) {
	clearProviderEnv(t)
	restore := ollamaProbe
	ollamaProbe = func() (string, bool) { return "", false }
	defer func() { ollamaProbe = restore }()

	_, err := runWithoutConfig(t, nil, "ask", "hi")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY")
	assert.Contains(t, err.Error(), "ollama serve")
	assert.NotContains(t, err.Error(), "unknown profile")
}
