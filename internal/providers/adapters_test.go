package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func drain(t *testing.T, ch <-chan Chunk) (text string, stop string, usage *Usage) {
	t.Helper()
	for c := range ch {
		switch c.Type {
		case ChunkTextDelta:
			text += c.Text
		case ChunkMessageStop:
			stop = c.StopReason
		case ChunkUsage:
			usage = c.Usage
		case ChunkError:
			t.Fatalf("stream error: %v", c.Err)
		}
	}
	return
}

func userSays(text string) Request {
	return Request{
		Model: "test-model",
		Messages: []Message{{
			Role:    RoleUser,
			Content: []ContentPart{{Type: PartText, Text: text}},
		}},
	}
}

// --- OpenAI-compatible ---

func TestOpenAI_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "Bearer k", r.Header.Get("Authorization"))
		var body struct {
			Model    string      `json:"model"`
			Messages []oaMessage `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "test-model", body.Model)
		require.Len(t, body.Messages, 1)
		assert.Equal(t, "hi", body.Messages[0].Content)

		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":2,"completion_tokens":4}
		}`))
	}))
	defer srv.Close()

	p, err := newOpenAI(Config{Name: "fake", Type: "openai-compat", APIKey: "k", BaseURL: srv.URL + "/v1"})
	require.NoError(t, err)
	resp, err := p.Complete(context.Background(), userSays("hi"))
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Messages[0].Content[0].Text)
	assert.Equal(t, StopEndTurn, resp.StopReason)
	assert.Equal(t, Usage{InputTokens: 2, OutputTokens: 4}, resp.Usage)
}

func TestOpenAI_Stream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"},\"finish_reason\":\"stop\"}]}\n\n" +
				"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":2}}\n\n" +
				"data: [DONE]\n\n"))
	}))
	defer srv.Close()

	p, _ := newOpenAI(Config{Name: "fake", Type: "openai-compat", APIKey: "k", BaseURL: srv.URL})
	ch, err := p.Stream(context.Background(), userSays("hi"))
	require.NoError(t, err)
	text, stop, usage := drain(t, ch)
	assert.Equal(t, "hello", text)
	assert.Equal(t, StopEndTurn, stop)
	require.NotNil(t, usage)
	assert.Equal(t, 2, usage.OutputTokens)
}

func TestOpenAI_ListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/models", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"id":"m1"},{"id":"m2"}]}`))
	}))
	defer srv.Close()

	p, _ := newOpenAI(Config{Name: "fake", Type: "openai-compat", APIKey: "k", BaseURL: srv.URL})
	models, err := p.ListModels(context.Background())
	require.NoError(t, err)
	require.Len(t, models, 2)
	assert.Equal(t, "m1", models[0].ID)
}

func TestOpenAI_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer srv.Close()

	p, _ := newOpenAI(Config{Name: "fake", Type: "openai-compat", APIKey: "k", BaseURL: srv.URL})
	_, err := p.Complete(context.Background(), userSays("hi"))
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 401, apiErr.Status)
	assert.Equal(t, "bad key", apiErr.Message)
}

func TestOpenAI_DefaultBaseURLRequired(t *testing.T) {
	_, err := newOpenAI(Config{Name: "custom", Type: "openai-compat"})
	assert.Error(t, err)

	p, err := newOpenAI(Config{Name: "openai", Type: "openai"})
	require.NoError(t, err)
	assert.Equal(t, "https://api.openai.com/v1", p.(*openAI).baseURL)
}

// --- Anthropic ---

func TestAnthropic_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "k", r.Header.Get("x-api-key"))
		assert.Equal(t, anthropicVersion, r.Header.Get("anthropic-version"))
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.EqualValues(t, anthropicMaxTokens, body["max_tokens"])

		_, _ = w.Write([]byte(`{
			"content":[{"type":"text","text":"hello"}],
			"stop_reason":"end_turn",
			"usage":{"input_tokens":2,"output_tokens":4}
		}`))
	}))
	defer srv.Close()

	p, err := newAnthropic(Config{Name: "anthropic", Type: "anthropic", APIKey: "k", BaseURL: srv.URL})
	require.NoError(t, err)
	resp, err := p.Complete(context.Background(), userSays("hi"))
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Messages[0].Content[0].Text)
	assert.Equal(t, StopEndTurn, resp.StopReason)
	assert.Equal(t, Usage{InputTokens: 2, OutputTokens: 4}, resp.Usage)
}

func TestAnthropic_Stream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"event: message_start\ndata: {\"message\":{\"usage\":{\"input_tokens\":3}}}\n\n" +
				"event: content_block_delta\ndata: {\"delta\":{\"type\":\"text_delta\",\"text\":\"hel\"}}\n\n" +
				"event: content_block_delta\ndata: {\"delta\":{\"type\":\"text_delta\",\"text\":\"lo\"}}\n\n" +
				"event: message_delta\ndata: {\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":5}}\n\n" +
				"event: message_stop\ndata: {}\n\n"))
	}))
	defer srv.Close()

	p, _ := newAnthropic(Config{Name: "anthropic", Type: "anthropic", APIKey: "k", BaseURL: srv.URL})
	ch, err := p.Stream(context.Background(), userSays("hi"))
	require.NoError(t, err)
	text, stop, usage := drain(t, ch)
	assert.Equal(t, "hello", text)
	assert.Equal(t, StopEndTurn, stop)
	require.NotNil(t, usage)
	assert.Equal(t, Usage{InputTokens: 3, OutputTokens: 5}, *usage)
}

func TestRegisterBuiltins(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltins(r)
	assert.Equal(t, []string{"anthropic", "gemini", "openai", "openai-compat", "openrouter"}, r.Types())
}
