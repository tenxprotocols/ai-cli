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

// Overrides holds CLI flag values. Empty strings mean "not set". Command is
// the subcommand being run; a matching [commands.<name>] block overlays the
// profile before env vars and flags are applied.
type Overrides struct {
	Command  string
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

// Resolve applies precedence: flag > AI_CLI_* env > public env >
// [commands.<name>] block > profile.
func Resolve(file File, overrides Overrides, env EnvLookup) (Resolved, error) {
	profileName := firstNonEmpty(overrides.Profile, envOr(env, "AI_CLI_PROFILE"), file.DefaultProfile)
	if profileName == "" {
		return Resolved{}, fmt.Errorf("%w: no profile selected", ErrUnknownProfile)
	}
	profile, ok := file.Profiles[profileName]
	if !ok {
		return Resolved{}, fmt.Errorf("%w: %s", ErrUnknownProfile, profileName)
	}
	if command, ok := file.Commands[overrides.Command]; ok {
		profile = overlay(profile, command)
	}

	providerName := firstNonEmpty(overrides.Provider, envOr(env, "AI_CLI_PROVIDER"), profile.Provider)
	providerCfg, ok := file.Providers[providerName]
	if !ok {
		return Resolved{}, fmt.Errorf("%w: %s", ErrUnknownProvider, providerName)
	}

	model := firstNonEmpty(overrides.Model, envOr(env, "AI_CLI_MODEL"), profile.Model)
	if model == "" {
		return Resolved{}, fmt.Errorf("no model selected for profile %q", profileName)
	}

	system := firstNonEmpty(overrides.System, envOr(env, "AI_CLI_SYSTEM"), profile.System)

	return Resolved{
		Profile:      profileName,
		ProviderName: providerName,
		ProviderType: providerCfg.Type,
		BaseURL:      providerCfg.BaseURL,
		APIKey:       resolveAPIKey(providerName, providerCfg.Type, providerCfg.APIKey, env),
		Model:        model,
		System:       system,
		Temperature:  profile.Temperature,
		MaxTokens:    profile.MaxTokens,
	}, nil
}

// overlay returns base with the command block's set fields applied on top.
func overlay(base, command Profile) Profile {
	if command.Provider != "" {
		base.Provider = command.Provider
	}
	if command.Model != "" {
		base.Model = command.Model
	}
	if command.System != "" {
		base.System = command.System
	}
	if command.Temperature != nil {
		base.Temperature = command.Temperature
	}
	if command.MaxTokens != nil {
		base.MaxTokens = command.MaxTokens
	}
	return base
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
