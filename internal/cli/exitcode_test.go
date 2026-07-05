package cli

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tenxprotocols/ai-cli/internal/config"
	"github.com/tenxprotocols/ai-cli/internal/providers"
)

func TestExitCode(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, ExitOK},
		{context.Canceled, ExitInterrupted},
		{&providers.APIError{Status: 401}, ExitAuthError},
		{&providers.APIError{Status: 429}, ExitAPIError},
		{&providers.APIError{Status: 500}, ExitAPIError},
		{fmt.Errorf("wrap: %w", config.ErrMissingAPIKey), ExitAuthError},
		{fmt.Errorf("wrap: %w", config.ErrUnknownProfile), ExitUsageError},
		{fmt.Errorf("wrap: %w", config.ErrUnknownProvider), ExitUsageError},
		{errors.New("anything else"), ExitAPIError},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, ExitCode(c.err), "error: %v", c.err)
	}
}
