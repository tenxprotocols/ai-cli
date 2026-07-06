package cli

import (
	"context"

	"github.com/tenxprotocols/ai-cli/internal/providers"
)

type ctxKey int

const injectedProvidersKey ctxKey = iota

// WithInjectedProvider lets tests substitute a fake provider for a given
// config block name, bypassing the registry.
func WithInjectedProvider(ctx context.Context, name string, p providers.Provider) context.Context {
	m, _ := ctx.Value(injectedProvidersKey).(map[string]providers.Provider)
	if m == nil {
		m = map[string]providers.Provider{}
	}
	m[name] = p
	return context.WithValue(ctx, injectedProvidersKey, m)
}

func injectedProvider(ctx context.Context, name string) (providers.Provider, bool) {
	m, ok := ctx.Value(injectedProvidersKey).(map[string]providers.Provider)
	if !ok {
		return nil, false
	}
	p, ok := m[name]
	return p, ok
}
