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
// and system prompt for this invocation.
func resolveForCall(gf *GlobalFlags) (config.Resolved, error) {
	path, err := resolveConfigPath(gf.ConfigPath)
	if err != nil {
		return config.Resolved{}, err
	}
	f, err := config.LoadFile(path)
	if err != nil {
		return config.Resolved{}, err
	}
	return config.Resolve(f, config.Overrides{
		Profile:  gf.Profile,
		Provider: gf.Provider,
		Model:    gf.Model,
		System:   systemPrompt(gf),
	}, config.OSEnv)
}

func systemPrompt(gf *GlobalFlags) string {
	if gf.System != "" || gf.SystemFile == "" {
		return gf.System
	}
	b, err := os.ReadFile(gf.SystemFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// buildProvider returns the resolved provider, honoring test injection.
func buildProvider(ctx context.Context, r config.Resolved) (providers.Provider, error) {
	if p, ok := injectedProvider(ctx, r.ProviderName); ok {
		return p, nil
	}
	if r.APIKey == "" && r.ProviderType != "openai-compat" {
		return nil, fmt.Errorf("%w for provider %q (set AI_CLI_%s_API_KEY)",
			config.ErrMissingAPIKey, r.ProviderName, strings.ToUpper(r.ProviderName))
	}
	reg := providers.NewRegistry()
	providers.RegisterBuiltins(reg)
	return reg.Get(r.ProviderType, providers.Config{
		Name:    r.ProviderName,
		APIKey:  r.APIKey,
		BaseURL: r.BaseURL,
	})
}

// readStdinIfPiped returns piped stdin content, if any.
func readStdinIfPiped() (string, bool) {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice != 0 {
		return "", false
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil || len(b) == 0 {
		return "", false
	}
	return strings.TrimRight(string(b), "\n"), true
}

func userMessage(prompt string) []providers.Message {
	return []providers.Message{{
		Role:    providers.RoleUser,
		Content: []providers.ContentPart{{Type: providers.PartText, Text: prompt}},
	}}
}
