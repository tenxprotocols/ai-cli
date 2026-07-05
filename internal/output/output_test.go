package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tenxprotocols/ai-cli/internal/providers"
)

func chunks(cs ...providers.Chunk) <-chan providers.Chunk {
	ch := make(chan providers.Chunk, len(cs))
	for _, c := range cs {
		ch <- c
	}
	close(ch)
	return ch
}

func TestParseFormat(t *testing.T) {
	for _, s := range []string{"text", "json", "jsonl"} {
		_, err := ParseFormat(s)
		assert.NoError(t, err, s)
	}
	_, err := ParseFormat("yaml")
	assert.Error(t, err)
}

func TestText_StreamsAndTerminatesLine(t *testing.T) {
	var buf bytes.Buffer
	err := Render(FormatText, &buf, chunks(
		providers.Chunk{Type: providers.ChunkTextDelta, Text: "hel"},
		providers.Chunk{Type: providers.ChunkTextDelta, Text: "lo"},
		providers.Chunk{Type: providers.ChunkMessageStop, StopReason: providers.StopEndTurn},
	))
	require.NoError(t, err)
	assert.Equal(t, "hello\n", buf.String())
}

func TestText_NoTrailingNewlineDuplication(t *testing.T) {
	var buf bytes.Buffer
	err := Render(FormatText, &buf, chunks(
		providers.Chunk{Type: providers.ChunkTextDelta, Text: "done\n"},
		providers.Chunk{Type: providers.ChunkMessageStop},
	))
	require.NoError(t, err)
	assert.Equal(t, "done\n", buf.String())
}

func TestText_ToolCallsSummarized(t *testing.T) {
	var buf bytes.Buffer
	err := Render(FormatText, &buf, chunks(
		providers.Chunk{Type: providers.ChunkToolCallStart, ToolCall: &providers.ToolCall{Name: "get_weather"}},
		providers.Chunk{Type: providers.ChunkMessageStop, StopReason: providers.StopToolUse},
	))
	require.NoError(t, err)
	assert.Equal(t, "<tool-calls: 1>\n", buf.String())
}

func TestText_SurfacesStreamError(t *testing.T) {
	boom := errors.New("boom")
	err := Render(FormatText, &bytes.Buffer{}, chunks(
		providers.Chunk{Type: providers.ChunkError, Err: boom},
	))
	assert.ErrorIs(t, err, boom)
}

func TestJSON_SingleObject(t *testing.T) {
	var buf bytes.Buffer
	err := Render(FormatJSON, &buf, chunks(
		providers.Chunk{Type: providers.ChunkTextDelta, Text: "json "},
		providers.Chunk{Type: providers.ChunkTextDelta, Text: "body"},
		providers.Chunk{Type: providers.ChunkUsage, Usage: &providers.Usage{InputTokens: 3, OutputTokens: 5}},
		providers.Chunk{Type: providers.ChunkMessageStop, StopReason: providers.StopEndTurn},
	))
	require.NoError(t, err)

	var got struct {
		Text       string          `json:"text"`
		StopReason string          `json:"stop_reason"`
		Usage      providers.Usage `json:"usage"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, "json body", got.Text)
	assert.Equal(t, "end_turn", got.StopReason)
	assert.Equal(t, 3, got.Usage.InputTokens)
}

func TestJSONL_SchemaFirstThenEvents(t *testing.T) {
	var buf bytes.Buffer
	err := Render(FormatJSONL, &buf, chunks(
		providers.Chunk{Type: providers.ChunkTextDelta, Text: "hi"},
		providers.Chunk{Type: providers.ChunkMessageStop, StopReason: providers.StopEndTurn},
	))
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 3)
	assert.JSONEq(t, `{"type":"schema","version":"1"}`, lines[0])
	assert.JSONEq(t, `{"type":"text_delta","text":"hi"}`, lines[1])
	assert.JSONEq(t, `{"type":"message_stop","stop_reason":"end_turn"}`, lines[2])
}
