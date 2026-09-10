package config

import (
	"context"

	"github.com/tenxprotocols/ai-cli/internal/logging"
)

// zeroConfigDefaults are tried in order when no config file defines profiles.
var zeroConfigDefaults = []struct {
	name, typ, model, envKey string
}{
	{"anthropic", "anthropic", "claude-sonnet-5", "ANTHROPIC_API_KEY"},
	{"openai", "openai", "gpt-5-mini", "OPENAI_API_KEY"},
	{"gemini", "gemini", "gemini-2.5-flash", "GEMINI_API_KEY"},
	{"openrouter", "openrouter", "openrouter/auto", "OPENROUTER_API_KEY"},
}

// ZeroConfig synthesizes a Resolved view for setups with no config file: the
// first well-known API key found in the environment wins; failing that, a
// running local Ollama (probed by the caller). Model and system overrides
// (flags, AI_CLI_* env) still apply.
func ZeroConfig(ctx context.Context, overrides Overrides, env EnvLookup, ollama func() (model string, ok bool)) (Resolved, bool) {
	log := logging.FromContext(ctx)

	resolved, from, ok := zeroProvider(env, ollama)
	if !ok {
		log.Debug("resolve: zero-config found nothing", "tried", len(zeroConfigDefaults)+1)
		return Resolved{}, false
	}
	resolved.Profile = "zero-config"

	model, modelFrom := pick(
		source{"flag", overrides.Model},
		source{"env AI_CLI_MODEL", envOr(env, "AI_CLI_MODEL")},
		source{"zero-config default", resolved.Model},
	)
	resolved.Model = model
	resolved.System = firstNonEmpty(overrides.System, envOr(env, "AI_CLI_SYSTEM"))

	log.Debug("resolve: zero-config",
		"provider", resolved.ProviderName, "from", from, "model", model, "model_from", modelFrom)
	return resolved, true
}

// DefaultModel suggests a model for a built-in provider type.
func DefaultModel(typ string) string {
	for _, d := range zeroConfigDefaults {
		if d.typ == typ {
			return d.model
		}
	}
	return ""
}

// zeroProvider returns the synthesized provider and the name of what selected
// it, for the log.
func zeroProvider(env EnvLookup, ollama func() (string, bool)) (Resolved, string, bool) {
	for _, d := range zeroConfigDefaults {
		if key, ok := env(d.envKey); ok && key != "" {
			return Resolved{
				ProviderName: d.name,
				ProviderType: d.typ,
				APIKey:       key,
				Model:        d.model,
			}, "env " + d.envKey, true
		}
	}
	if ollama != nil {
		if model, ok := ollama(); ok {
			return Resolved{
				ProviderName: "ollama",
				ProviderType: "openai-compat",
				BaseURL:      "http://localhost:11434/v1",
				Model:        model,
			}, "local ollama", true
		}
	}
	return Resolved{}, "unset", false
}
