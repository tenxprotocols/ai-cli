package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tenxprotocols/ai-cli/internal/logging"
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
// [commands.<name>] block > profile. Every step is logged at debug with the
// source that won, which is what makes "why that model?" answerable.
func Resolve(ctx context.Context, file File, overrides Overrides, env EnvLookup) (Resolved, error) {
	log := logging.FromContext(ctx)

	profileName, from := pick(
		source{"flag", overrides.Profile},
		source{"env AI_CLI_PROFILE", envOr(env, "AI_CLI_PROFILE")},
		source{"file default_profile", file.DefaultProfile},
	)
	if profileName == "" {
		return Resolved{}, fmt.Errorf("%w: no profile selected", ErrUnknownProfile)
	}
	log.Debug("resolve: profile", "profile", profileName, "from", from)

	profile, ok := file.Profiles[profileName]
	if !ok {
		return Resolved{}, fmt.Errorf("%w: %s", ErrUnknownProfile, profileName)
	}
	if command, ok := file.Commands[overrides.Command]; ok {
		profile = overlay(profile, command)
		log.Debug("resolve: command block applied", "command", overrides.Command)
	}

	providerName, from := pick(
		source{"flag", overrides.Provider},
		source{"env AI_CLI_PROVIDER", envOr(env, "AI_CLI_PROVIDER")},
		source{"profile", profile.Provider},
	)
	providerCfg, ok := file.Providers[providerName]
	if !ok {
		return Resolved{}, fmt.Errorf("%w: %s", ErrUnknownProvider, providerName)
	}
	log.Debug("resolve: provider", "provider", providerName, "from", from,
		"type", providerCfg.Type, "base_url", providerCfg.BaseURL)

	model, from := pick(
		source{"flag", overrides.Model},
		source{"env AI_CLI_MODEL", envOr(env, "AI_CLI_MODEL")},
		source{"profile", profile.Model},
	)
	if model == "" {
		return Resolved{}, fmt.Errorf("no model selected for profile %q", profileName)
	}
	log.Debug("resolve: model", "model", model, "from", from)

	system, from := pick(
		source{"flag", overrides.System},
		source{"env AI_CLI_SYSTEM", envOr(env, "AI_CLI_SYSTEM")},
		source{"profile", profile.System},
	)
	if system != "" {
		log.Debug("resolve: system prompt", "from", from, "chars", len(system))
		log.Log(ctx, logging.LevelTrace, "resolve: system prompt text", "system", system)
	}

	apiKey, keyFrom := resolveAPIKey(providerName, providerCfg.Type, providerCfg.APIKey, env)
	log.Debug("resolve: api key", "from", keyFrom, "key", logging.Redact(apiKey))

	resolved := Resolved{
		Profile:      profileName,
		ProviderName: providerName,
		ProviderType: providerCfg.Type,
		BaseURL:      providerCfg.BaseURL,
		APIKey:       apiKey,
		Model:        model,
		System:       system,
		Temperature:  profile.Temperature,
		MaxTokens:    profile.MaxTokens,
	}
	log.Log(ctx, logging.LevelTrace, "resolve: result",
		"profile", resolved.Profile, "provider", resolved.ProviderName,
		"type", resolved.ProviderType, "base_url", resolved.BaseURL, "model", resolved.Model,
		"key", logging.Redact(resolved.APIKey),
		"temperature", floatOrUnset(resolved.Temperature),
		"max_tokens", intOrUnset(resolved.MaxTokens))
	return resolved, nil
}

// source pairs a candidate value with where it came from.
type source struct {
	name  string
	value string
}

// pick returns the first non-empty candidate and the name of its source.
func pick(candidates ...source) (string, string) {
	for _, candidate := range candidates {
		if candidate.value != "" {
			return candidate.value, candidate.name
		}
	}
	return "", "unset"
}

func floatOrUnset(value *float64) string {
	if value == nil {
		return "unset"
	}
	return fmt.Sprintf("%g", *value)
}

func intOrUnset(value *int) string {
	if value == nil {
		return "unset"
	}
	return fmt.Sprintf("%d", *value)
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
	key, _ := resolveAPIKey(name, typ, fileVal, env)
	return key
}

// resolveAPIKey returns the key and the name of the env var or config field it
// came from. That name is safe to log; the key is not.
func resolveAPIKey(name, typ, fileVal string, env EnvLookup) (string, string) {
	specific := "AI_CLI_" + strings.ToUpper(name) + "_API_KEY"
	if v, ok := env(specific); ok && v != "" {
		return v, "env " + specific
	}
	for _, public := range publicKeyVars[typ] {
		if v, ok := env(public); ok && v != "" {
			return v, "env " + public
		}
	}
	if fileVal != "" {
		return fileVal, "config file"
	}
	return "", "unset"
}

// publicKeyVars are the conventional env vars each provider type honors.
var publicKeyVars = map[string][]string{
	"anthropic":  {"ANTHROPIC_API_KEY"},
	"openai":     {"OPENAI_API_KEY"},
	"openrouter": {"OPENROUTER_API_KEY"},
	"gemini":     {"GEMINI_API_KEY", "GOOGLE_API_KEY"},
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
