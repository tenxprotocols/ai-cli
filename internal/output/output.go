// Package output renders provider chunk streams as text, JSON, or JSONL.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/tenxprotocols/ai-cli/internal/providers"
)

type Format int

const (
	FormatText Format = iota
	FormatJSON
	FormatJSONL
)

func ParseFormat(s string) (Format, error) {
	switch s {
	case "text", "":
		return FormatText, nil
	case "json":
		return FormatJSON, nil
	case "jsonl":
		return FormatJSONL, nil
	}
	return 0, fmt.Errorf("unknown format %q (want text|json|jsonl)", s)
}

// Render consumes the chunk stream and writes it to w in the given format.
// It drains ch fully and returns the first stream error, if any.
func Render(f Format, w io.Writer, ch <-chan providers.Chunk) error {
	switch f {
	case FormatJSON:
		return renderJSON(w, ch)
	case FormatJSONL:
		return renderJSONL(w, ch)
	default:
		return renderText(w, ch)
	}
}

func renderText(w io.Writer, ch <-chan providers.Chunk) error {
	last, toolCalls := "", 0
	for c := range ch {
		switch c.Type {
		case providers.ChunkTextDelta:
			if _, err := io.WriteString(w, c.Text); err != nil {
				return err
			}
			last = c.Text
		case providers.ChunkToolCallStart:
			toolCalls++
		case providers.ChunkError:
			return c.Err
		}
	}
	if toolCalls > 0 {
		_, err := fmt.Fprintf(w, "<tool-calls: %d>\n", toolCalls)
		return err
	}
	if last != "" && !strings.HasSuffix(last, "\n") {
		_, err := io.WriteString(w, "\n")
		return err
	}
	return nil
}

func renderJSON(w io.Writer, ch <-chan providers.Chunk) error {
	var (
		text  strings.Builder
		calls []providers.ToolCall
		usage providers.Usage
		stop  string
	)
	for c := range ch {
		switch c.Type {
		case providers.ChunkTextDelta:
			text.WriteString(c.Text)
		case providers.ChunkToolCallStart:
			calls = append(calls, *c.ToolCall)
		case providers.ChunkUsage:
			usage = *c.Usage
		case providers.ChunkMessageStop:
			stop = c.StopReason
		case providers.ChunkError:
			return c.Err
		}
	}
	enc := json.NewEncoder(w)
	return enc.Encode(map[string]any{
		"text":        text.String(),
		"tool_calls":  calls,
		"stop_reason": stop,
		"usage":       usage,
	})
}

func renderJSONL(w io.Writer, ch <-chan providers.Chunk) error {
	enc := json.NewEncoder(w)
	if err := enc.Encode(map[string]string{"type": "schema", "version": "1"}); err != nil {
		return err
	}
	for c := range ch {
		if c.Type == providers.ChunkError {
			return c.Err
		}
		if err := enc.Encode(c); err != nil {
			return err
		}
	}
	return nil
}
