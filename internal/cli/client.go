package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tenxprotocols/ai-cli/internal/config"
	"github.com/tenxprotocols/ai-cli/internal/providers"
)

// resolveForCall loads the config file and resolves profile, provider, model,
// and system prompt for one invocation of the named subcommand. When the file
// defines no profiles, zero-config mode synthesizes one from well-known env
// keys or a running local Ollama.
func resolveForCall(command string, flags *GlobalFlags) (config.Resolved, error) {
	path, err := resolveConfigPath(flags.ConfigPath)
	if err != nil {
		return config.Resolved{}, err
	}
	file, err := config.LoadFile(path)
	if err != nil {
		return config.Resolved{}, err
	}
	overrides := config.Overrides{
		Command:  command,
		Profile:  flags.Profile,
		Provider: flags.Provider,
		Model:    flags.Model,
		System:   systemPrompt(flags),
	}
	resolved, err := config.Resolve(file, overrides, config.OSEnv)
	if err == nil || len(file.Profiles) > 0 || flags.Profile != "" {
		return resolved, err
	}
	if zero, ok := config.ZeroConfig(overrides, config.OSEnv, ollamaProbe); ok {
		return zero, nil
	}
	return config.Resolved{}, fmt.Errorf(`nothing configured yet. Fastest ways to a working ai:

  export ANTHROPIC_API_KEY=...    # or OPENAI_API_KEY / OPENROUTER_API_KEY
  export GEMINI_API_KEY=...       # free tier: https://aistudio.google.com
  ollama serve                    # local and free, no key needed

or write a config file at %s — see docs/configuration.md`, path)
}

// ollamaProbe checks for a local Ollama and returns its first model.
// Package-level so tests can substitute it.
var ollamaProbe = func() (string, bool) {
	client := &http.Client{Timeout: 300 * time.Millisecond}
	resp, err := client.Get("http://localhost:11434/v1/models")
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", false
	}
	defer resp.Body.Close()
	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil || len(out.Data) == 0 {
		return "", false
	}
	return out.Data[0].ID, true
}

func systemPrompt(flags *GlobalFlags) string {
	if flags.System != "" || flags.SystemFile == "" {
		return flags.System
	}
	contents, err := os.ReadFile(flags.SystemFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(contents))
}

// buildProvider returns the resolved provider, honoring test injection.
func buildProvider(ctx context.Context, resolved config.Resolved) (providers.Provider, error) {
	if provider, ok := injectedProvider(ctx, resolved.ProviderName); ok {
		return provider, nil
	}
	if resolved.APIKey == "" && resolved.ProviderType != "openai-compat" {
		return nil, fmt.Errorf("%w for provider %q (set AI_CLI_%s_API_KEY)",
			config.ErrMissingAPIKey, resolved.ProviderName, strings.ToUpper(resolved.ProviderName))
	}
	registry := providers.NewRegistry()
	providers.RegisterBuiltins(registry)
	return registry.Get(resolved.ProviderType, providers.Config{
		Name:    resolved.ProviderName,
		APIKey:  resolved.APIKey,
		BaseURL: resolved.BaseURL,
	})
}

// readStdinIfPiped returns piped stdin content, if any.
func readStdinIfPiped() (string, bool) {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice != 0 {
		return "", false
	}
	contents, err := io.ReadAll(os.Stdin)
	if err != nil || len(contents) == 0 {
		return "", false
	}
	return strings.TrimRight(string(contents), "\n"), true
}

func userMessage(prompt string) []providers.Message {
	return []providers.Message{{
		Role:    providers.RoleUser,
		Content: []providers.ContentPart{{Type: providers.PartText, Text: prompt}},
	}}
}
