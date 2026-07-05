package cli

import (
	"context"
	"errors"

	"github.com/tenxprotocols/ai-cli/internal/config"
	"github.com/tenxprotocols/ai-cli/internal/providers"
)

// Exit codes, per the design spec.
const (
	ExitOK          = 0
	ExitAPIError    = 1
	ExitUsageError  = 2
	ExitAuthError   = 3
	ExitInputError  = 4
	ExitInterrupted = 130
)

// ExitCode classifies an error from Execute into a process exit code.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	if errors.Is(err, context.Canceled) {
		return ExitInterrupted
	}
	var apiErr *providers.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Status == 401 || apiErr.Status == 403 {
			return ExitAuthError
		}
		return ExitAPIError
	}
	switch {
	case errors.Is(err, config.ErrMissingAPIKey):
		return ExitAuthError
	case errors.Is(err, config.ErrUnknownProfile),
		errors.Is(err, config.ErrUnknownProvider),
		errors.Is(err, providers.ErrUnknownProvider):
		return ExitUsageError
	}
	return ExitAPIError
}
