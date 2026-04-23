package providers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessage_JSONRoundtrip(t *testing.T) {
	orig := Message{
		Role: "assistant",
		Content: []ContentPart{
			{Type: "text", Text: "hello"},
			{Type: "image", MIMEType: "image/png", Data: []byte{0x01, 0x02}},
		},
		ToolCalls: []ToolCall{
			{ID: "call_1", Name: "get_weather", Arguments: json.RawMessage(`{"city":"SF"}`)},
		},
	}
	b, err := json.Marshal(orig)
	require.NoError(t, err)

	var got Message
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, orig, got)
}

func TestChunk_Types(t *testing.T) {
	cases := []string{
		ChunkTextDelta, ChunkToolCallStart, ChunkToolCallDelta,
		ChunkThinkingDelta, ChunkMessageStop, ChunkUsage, ChunkError,
	}
	seen := map[string]bool{}
	for _, c := range cases {
		assert.False(t, seen[c], "duplicate chunk type: %s", c)
		seen[c] = true
	}
}
