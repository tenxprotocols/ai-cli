package config

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUnknownProfile  = errors.New("unknown profile")
	ErrUnknownProvider = errors.New("unknown provider")
	ErrMissingAPIKey   = errors.New("missing API key")
)

// EnvLookup is an injectable env var reader, so tests don't depend on the real
// environment.
type EnvLookup func(string) (string, bool)

// OSEnv is the production EnvLookup.
func OSEnv(k string) (string, bool) {
	if v, ok := osLookupEnv(k); ok {
		return v, true
	}
	return "", false
}

// Overrides holds CLI flag values. Empty strings mean "not set".
type Overrides struct {
	Profile  string
	Provider string
	Model    string
	System   string
}

// Resolved is the fully resolved view used by subcommands.
type Resolved struct {
	Profile      string
	ProviderName string // config block key
	ProviderType string // built-in type
	BaseURL      string
	APIKey       string
	Model        string
	System       string
	Temperature  *float64
	MaxTokens    *int
}

// Resolve applies precedence: flag > AI_CLI_* env > public env > file.
func Resolve(f File, o Overrides, env EnvLookup) (Resolved, error) {
	profileName := firstNonEmpty(o.Profile, envOr(env, "AI_CLI_PROFILE"), f.DefaultProfile)
	if profileName == "" {
		return Resolved{}, fmt.Errorf("%w: no profile selected", ErrUnknownProfile)
	}
	prof, ok := f.Profiles[profileName]
	if !ok {
		return Resolved{}, fmt.Errorf("%w: %s", ErrUnknownProfile, profileName)
	}

	provName := firstNonEmpty(o.Provider, envOr(env, "AI_CLI_PROVIDER"), prof.Provider)
	provCfg, ok := f.Providers[provName]
	if !ok {
		return Resolved{}, fmt.Errorf("%w: %s", ErrUnknownProvider, provName)
	}

	model := firstNonEmpty(o.Model, envOr(env, "AI_CLI_MODEL"), prof.Model)
	if model == "" {
		return Resolved{}, fmt.Errorf("no model selected for profile %q", profileName)
	}

	system := firstNonEmpty(o.System, envOr(env, "AI_CLI_SYSTEM"), prof.System)

	apiKey := resolveAPIKey(provName, provCfg.Type, provCfg.APIKey, env)

	return Resolved{
		Profile:      profileName,
		ProviderName: provName,
		ProviderType: provCfg.Type,
		BaseURL:      provCfg.BaseURL,
		APIKey:       apiKey,
		Model:        model,
		System:       system,
		Temperature:  prof.Temperature,
		MaxTokens:    prof.MaxTokens,
	}, nil
}

// ResolveAPIKeyForProbe exposes resolveAPIKey for callers that know the
// provider type but don't have a resolved profile.
func ResolveAPIKeyForProbe(name, typ, fileVal string, env EnvLookup) string {
	return resolveAPIKey(name, typ, fileVal, env)
}

func resolveAPIKey(name, typ, fileVal string, env EnvLookup) string {
	if v, ok := env("AI_CLI_" + strings.ToUpper(name) + "_API_KEY"); ok && v != "" {
		return v
	}
	switch typ {
	case "anthropic":
		if v, ok := env("ANTHROPIC_API_KEY"); ok && v != "" {
			return v
		}
	case "openai":
		if v, ok := env("OPENAI_API_KEY"); ok && v != "" {
			return v
		}
	case "openrouter":
		if v, ok := env("OPENROUTER_API_KEY"); ok && v != "" {
			return v
		}
	case "gemini":
		if v, ok := env("GEMINI_API_KEY"); ok && v != "" {
			return v
		}
		if v, ok := env("GOOGLE_API_KEY"); ok && v != "" {
			return v
		}
	}
	return fileVal
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

func envOr(env EnvLookup, key string) string {
	if v, ok := env(key); ok {
		return v
	}
	return ""
}
