package providers

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// ErrUnknownProvider is returned when a requested provider name has no
// constructor registered.
var ErrUnknownProvider = errors.New("unknown provider")

// Config is passed to each provider constructor. Adapters ignore fields they
// don't understand.
type Config struct {
	Name    string // user-facing name (config block key)
	Type    string // built-in type: anthropic|openai|openrouter|gemini|openai-compat
	APIKey  string
	BaseURL string            // for openai-compat
	Extra   map[string]string // future-proofing for provider-specific config
}

// Constructor builds a Provider from a Config.
type Constructor func(Config) (Provider, error)

type Registry struct {
	mu    sync.RWMutex
	ctors map[string]Constructor // keyed by Type
}

func NewRegistry() *Registry {
	return &Registry{ctors: map[string]Constructor{}}
}

func (r *Registry) Register(typ string, c Constructor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ctors[typ] = c
}

// Get builds a Provider for the given type. cfg.Name is used as the
// provider's Name() (so the openai adapter can identify itself as
// "openrouter" or "ollama" based on how it was configured).
func (r *Registry) Get(typ string, cfg Config) (Provider, error) {
	r.mu.RLock()
	c, ok := r.ctors[typ]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownProvider, typ)
	}
	cfg.Type = typ
	return c(cfg)
}

// Types returns the registered type names, sorted.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.ctors))
	for k := range r.ctors {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
