package cli

import (
	"github.com/tenxprotocols/ai-cli/internal/logging"
)

// logOptions resolves the log flags. The flag defaults already fall back to
// the env vars, so precedence is flag > env > default. On error the returned
// options are still usable, so a bad value never leaves the run without a
// logger.
func (flags *GlobalFlags) logOptions() (logging.Options, error) {
	options := logging.Options{
		Level:   logging.DefaultLevel,
		Format:  logging.FormatText,
		File:    flags.LogFile,
		Secrets: flags.LogSecrets,
	}
	level, err := logging.ParseLevel(flags.LogLevel)
	if err != nil {
		return options, err
	}
	format, err := logging.ParseFormat(flags.LogFormat)
	if err != nil {
		return options, err
	}
	options.Level, options.Format = level, format
	return options, nil
}

// PeekLogOptions learns the log flags from a whole command line before
// dispatch, wherever on the line they appear and without consuming them: it
// parses a copy of argv against a throwaway command tree that ignores flags
// it does not own. The real root command re-parses the untouched argv, and
// plugins receive it unchanged.
//
// The peek is best effort. An error means the caller should log at the
// returned (default) settings and let cobra report the problem, which it does
// with the full flag definitions to hand.
func PeekLogOptions(argv []string) (logging.Options, error) {
	peek, flags := newRootWithFlags()
	peek.FParseErrWhitelist.UnknownFlags = true
	_ = peek.ParseFlags(argv)
	return flags.logOptions()
}
