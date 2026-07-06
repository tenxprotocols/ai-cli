package config

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
func ZeroConfig(overrides Overrides, env EnvLookup, ollama func() (model string, ok bool)) (Resolved, bool) {
	resolved, ok := zeroProvider(env, ollama)
	if !ok {
		return Resolved{}, false
	}
	resolved.Profile = "zero-config"
	resolved.Model = firstNonEmpty(overrides.Model, envOr(env, "AI_CLI_MODEL"), resolved.Model)
	resolved.System = firstNonEmpty(overrides.System, envOr(env, "AI_CLI_SYSTEM"))
	return resolved, true
}

func zeroProvider(env EnvLookup, ollama func() (string, bool)) (Resolved, bool) {
	for _, d := range zeroConfigDefaults {
		if key, ok := env(d.envKey); ok && key != "" {
			return Resolved{
				ProviderName: d.name,
				ProviderType: d.typ,
				APIKey:       key,
				Model:        d.model,
			}, true
		}
	}
	if ollama != nil {
		if model, ok := ollama(); ok {
			return Resolved{
				ProviderName: "ollama",
				ProviderType: "openai-compat",
				BaseURL:      "http://localhost:11434/v1",
				Model:        model,
			}, true
		}
	}
	return Resolved{}, false
}
