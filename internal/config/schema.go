package config

// File is the on-disk TOML shape.
type File struct {
	DefaultProfile string              `toml:"default_profile"`
	Providers      map[string]Provider `toml:"providers"`
	Profiles       map[string]Profile  `toml:"profiles"`
}

type Provider struct {
	Type    string `toml:"type"`              // anthropic|openai|openrouter|gemini|openai-compat
	APIKey  string `toml:"api_key,omitempty"` // usually unset; filled from env
	BaseURL string `toml:"base_url,omitempty"`
}

type Profile struct {
	Provider    string   `toml:"provider"`
	Model       string   `toml:"model"`
	System      string   `toml:"system,omitempty"`
	Temperature *float64 `toml:"temperature,omitempty"`
	MaxTokens   *int     `toml:"max_tokens,omitempty"`
}
