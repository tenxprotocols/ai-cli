// Package logging provides the leveled logger every ai subcommand writes to,
// the destination it writes to, and an HTTP transport that records requests
// and responses on the wire.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Levels. trace sits below slog's debug: debug carries the decision trail,
// trace adds full HTTP headers and bodies.
const (
	LevelTrace = slog.Level(-8)
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// DefaultLevel applies when neither the flag nor the env var says otherwise.
const DefaultLevel = LevelWarn

var levelsByName = map[string]slog.Level{
	"error": LevelError,
	"warn":  LevelWarn,
	"info":  LevelInfo,
	"debug": LevelDebug,
	"trace": LevelTrace,
}

// LevelNames lists accepted level names, quietest first, for flag help.
func LevelNames() []string {
	return []string{"error", "warn", "info", "debug", "trace"}
}

// ParseLevel maps a level name to a level. An empty name yields DefaultLevel.
func ParseLevel(name string) (slog.Level, error) {
	if name == "" {
		return DefaultLevel, nil
	}
	if level, ok := levelsByName[strings.ToLower(strings.TrimSpace(name))]; ok {
		return level, nil
	}
	return 0, fmt.Errorf("unknown log level %q: want one of %s",
		name, strings.Join(LevelNames(), ", "))
}

// Format selects the log record encoding.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// FormatNames lists accepted format names, for flag help.
func FormatNames() []string { return []string{string(FormatText), string(FormatJSON)} }

// ParseFormat maps a format name to a Format. An empty name yields FormatText.
func ParseFormat(name string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(name))) {
	case "":
		return FormatText, nil
	case FormatText:
		return FormatText, nil
	case FormatJSON:
		return FormatJSON, nil
	}
	return "", fmt.Errorf("unknown log format %q: want one of %s",
		name, strings.Join(FormatNames(), ", "))
}

// Options describes a log destination. The zero value logs warnings and above
// as text on stderr with secrets redacted.
type Options struct {
	Level   slog.Level
	Format  Format
	File    string // empty logs to stderr
	Secrets bool   // true prints credentials unredacted
}

// Sink owns the log destination. Its level is adjustable after construction,
// so the early logger built before flag parsing can be corrected in place
// rather than replaced.
type Sink struct {
	mu     sync.Mutex
	level  *slog.LevelVar
	opts   Options
	file   *os.File
	logger *slog.Logger
}

// Open builds a Sink. Records go to opts.File when set, else to stderr.
func Open(opts Options, stderr io.Writer) (*Sink, error) {
	sink := &Sink{level: new(slog.LevelVar)}
	if err := sink.reset(opts, stderr); err != nil {
		return nil, err
	}
	return sink, nil
}

// Reopen applies a new set of options, reusing the current file handle when
// the path is unchanged so nothing already written is lost.
func (s *Sink) Reopen(opts Options, stderr io.Writer) error {
	return s.reset(opts, stderr)
}

func (s *Sink) reset(opts Options, stderr io.Writer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	writer := stderr
	if opts.File != "" {
		if s.file == nil || s.file.Name() != opts.File {
			file, err := os.OpenFile(opts.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
			if err != nil {
				return fmt.Errorf("open log file %s: %w", opts.File, err)
			}
			s.closeFile()
			s.file = file
		}
		writer = s.file
	} else {
		s.closeFile()
	}

	s.level.Set(opts.Level)
	s.opts = opts
	s.logger = slog.New(newHandler(opts.Format, writer, s.level))
	return nil
}

// closeFile releases the current file handle. Callers hold s.mu.
func (s *Sink) closeFile() {
	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
}

// Logger returns the sink's logger. Safe to call from any goroutine.
func (s *Sink) Logger() *slog.Logger {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logger
}

// SetLevel changes the level of every logger this sink has handed out.
func (s *Sink) SetLevel(level slog.Level) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.level.Set(level)
	s.opts.Level = level
}

// Level reports the current level.
func (s *Sink) Level() slog.Level {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.level.Level()
}

// Secrets reports whether credentials print unredacted.
func (s *Sink) Secrets() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.opts.Secrets
}

// Close releases the log file, if one is open. Stderr is left alone.
func (s *Sink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

type ctxKey struct{}

// NewContext returns a context carrying log, for subcommands to pick up.
func NewContext(ctx context.Context, log *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, log)
}

// FromContext returns the context's logger, or a discarding one when absent,
// so callers never nil-check.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if log, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && log != nil {
			return log
		}
	}
	return Discard()
}

// Discard returns a logger that drops everything.
func Discard() *slog.Logger { return slog.New(slog.DiscardHandler) }
