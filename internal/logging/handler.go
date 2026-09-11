package logging

import (
	"context"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
)

// newHandler builds the handler backing a sink.
func newHandler(format Format, writer io.Writer, level *slog.LevelVar) slog.Handler {
	if format == FormatJSON {
		return slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level:       level,
			ReplaceAttr: nameTraceLevel,
		})
	}
	return &textHandler{mu: &sync.Mutex{}, writer: writer, level: level, start: time.Now()}
}

// nameTraceLevel spells the custom trace level, which slog would otherwise
// render as DEBUG-4.
func nameTraceLevel(groups []string, attr slog.Attr) slog.Attr {
	if len(groups) == 0 && attr.Key == slog.LevelKey {
		if level, ok := attr.Value.Any().(slog.Level); ok && level == LevelTrace {
			return slog.String(slog.LevelKey, "TRACE")
		}
	}
	return attr
}

// textHandler renders one compact line per record: an optional relative
// timestamp, a padded level, the message, then key=value pairs. The timestamp
// appears only at debug and trace, where request timing is half the point.
type textHandler struct {
	mu     *sync.Mutex
	writer io.Writer
	level  *slog.LevelVar
	start  time.Time
	attrs  []slog.Attr
	groups []string
}

func (h *textHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *textHandler) Handle(_ context.Context, record slog.Record) error {
	line := make([]byte, 0, 128)
	if h.level.Level() <= LevelDebug {
		line = append(line, '+')
		line = strconv.AppendInt(line, time.Since(h.start).Milliseconds(), 10)
		line = append(line, "ms "...)
	}
	line = append(line, levelLabel(record.Level)...)
	line = append(line, ' ')
	line = append(line, record.Message...)
	for _, attr := range h.attrs {
		line = appendAttr(line, h.groups, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		line = appendAttr(line, h.groups, attr)
		return true
	})
	line = append(line, '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.writer.Write(line)
	return err
}

func (h *textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &clone
}

func (h *textHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.groups = append(append([]string{}, h.groups...), name)
	return &clone
}

// levelLabel pads to five columns so messages line up.
func levelLabel(level slog.Level) string {
	switch {
	case level <= LevelTrace:
		return "TRACE"
	case level <= LevelDebug:
		return "DEBUG"
	case level <= LevelInfo:
		return "INFO "
	case level <= LevelWarn:
		return "WARN "
	default:
		return "ERROR"
	}
}

func appendAttr(line []byte, groups []string, attr slog.Attr) []byte {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return line
	}
	if attr.Value.Kind() == slog.KindGroup {
		nested := attr.Value.Group()
		if len(nested) == 0 {
			return line
		}
		if attr.Key != "" {
			groups = append(append([]string{}, groups...), attr.Key)
		}
		for _, inner := range nested {
			line = appendAttr(line, groups, inner)
		}
		return line
	}
	line = append(line, ' ')
	for _, group := range groups {
		line = append(line, group...)
		line = append(line, '.')
	}
	line = append(line, attr.Key...)
	line = append(line, '=')
	return append(line, quoteIfNeeded(attr.Value.String())...)
}

// quoteIfNeeded quotes values that would otherwise run into the next pair.
func quoteIfNeeded(value string) string {
	if value == "" || strings.ContainsAny(value, " \"'=\n\t\r") {
		return strconv.Quote(value)
	}
	return value
}
