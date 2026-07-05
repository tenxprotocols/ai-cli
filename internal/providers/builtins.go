package providers

// RegisterBuiltins installs the built-in provider constructors. The gemini
// type uses Google's OpenAI-compatible endpoint, so one HTTP client backs
// everything except Anthropic.
func RegisterBuiltins(r *Registry) {
	r.Register("anthropic", newAnthropic)
	for _, t := range []string{"openai", "openrouter", "gemini", "openai-compat"} {
		r.Register(t, newOpenAI)
	}
}
