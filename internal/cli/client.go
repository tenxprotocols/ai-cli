package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tenxprotocols/ai-cli/internal/config"
	"github.com/tenxprotocols/ai-cli/internal/providers"
)

// resolveForCall loads the config file and resolves profile, provider, model,
// and system prompt for one invocation of the named subcommand.
func resolveForCall(command string, flags *GlobalFlags) (config.Resolved, error) {
	path, err := resolveConfigPath(flags.ConfigPath)
	if err != nil {
		return config.Resolved{}, err
	}
	file, err := config.LoadFile(path)
	if err != nil {
		return config.Resolved{}, err
	}
	return config.Resolve(file, config.Overrides{
		Command:  command,
		Profile:  flags.Profile,
		Provider: flags.Provider,
		Model:    flags.Model,
		System:   systemPrompt(flags),
	}, config.OSEnv)
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
