package providers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeProvider struct{ name string }

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Complete(context.Context, Request) (Response, error) {
	return Response{}, nil
}

func (f *fakeProvider) Stream(context.Context, Request) (<-chan Chunk, error) {
	ch := make(chan Chunk)
	close(ch)
	return ch, nil
}

func (f *fakeProvider) ListModels(context.Context) ([]ModelInfo, error) {
	return nil, ErrNotSupported
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	r.Register("anthropic", func(Config) (Provider, error) {
		return &fakeProvider{name: "anthropic"}, nil
	})

	p, err := r.Get("anthropic", Config{})
	require.NoError(t, err)
	assert.Equal(t, "anthropic", p.Name())

	_, err = r.Get("missing", Config{})
	assert.ErrorIs(t, err, ErrUnknownProvider)
}

func TestRegistry_TypesSorted(t *testing.T) {
	r := NewRegistry()
	r.Register("zulu", func(Config) (Provider, error) { return nil, nil })
	r.Register("alpha", func(Config) (Provider, error) { return nil, nil })
	r.Register("mike", func(Config) (Provider, error) { return nil, nil })

	assert.Equal(t, []string{"alpha", "mike", "zulu"}, r.Types())
}
