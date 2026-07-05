package providers

import (
	"context"
	"encoding/json"
	"fmt"
)

// openAI speaks the OpenAI chat-completions protocol. It backs the openai,
// openrouter, gemini (via Google's OpenAI-compatible endpoint), and
// openai-compat provider types.
type openAI struct {
	name    string
	baseURL string
	apiKey  string
}

func newOpenAI(cfg Config) (Provider, error) {
	base := cfg.BaseURL
	if base == "" {
		switch cfg.Type {
		case "openai":
			base = "https://api.openai.com/v1"
		case "openrouter":
			base = "https://openrouter.ai/api/v1"
		case "gemini":
			base = "https://generativelanguage.googleapis.com/v1beta/openai"
		default:
			return nil, fmt.Errorf("provider %q: base_url is required for type %q", cfg.Name, cfg.Type)
		}
	}
	return &openAI{name: cfg.Name, baseURL: base, apiKey: cfg.APIKey}, nil
}

func (o *openAI) Name() string { return o.name }

func (o *openAI) headers() map[string]string {
	return map[string]string{"Authorization": "Bearer " + o.apiKey}
}

type oaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (o *openAI) body(req Request) map[string]any {
	msgs := make([]oaMessage, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, oaMessage{RoleSystem, req.System})
	}
	for _, m := range req.Messages {
		msgs = append(msgs, oaMessage{m.Role, textOf(m)})
	}
	body := map[string]any{"model": req.Model, "messages": msgs}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		body["max_tokens"] = *req.MaxTokens
	}
	return body
}

func (o *openAI) Complete(ctx context.Context, req Request) (Response, error) {
	resp, err := send(ctx, "POST", o.baseURL+"/chat/completions", o.headers(), o.body(req))
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	var out struct {
		Choices []struct {
			Message      oaMessage `json:"message"`
			FinishReason string    `json:"finish_reason"`
		} `json:"choices"`
		Usage oaUsage `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, fmt.Errorf("decode %s response: %w", o.name, err)
	}
	if len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("%s returned no choices", o.name)
	}
	c := out.Choices[0]
	return Response{
		Messages: []Message{{
			Role:    RoleAssistant,
			Content: []ContentPart{{Type: PartText, Text: c.Message.Content}},
		}},
		Usage:      out.Usage.toUsage(),
		StopReason: oaStopReason(c.FinishReason),
	}, nil
}

func (o *openAI) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	body := o.body(req)
	body["stream"] = true
	body["stream_options"] = map[string]any{"include_usage": true}
	resp, err := send(ctx, "POST", o.baseURL+"/chat/completions", o.headers(), body)
	if err != nil {
		return nil, err
	}

	ch := make(chan Chunk, 16)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		stop := StopEndTurn
		sc := newSSEScanner(resp.Body)
		for {
			_, data, ok := sc.Next()
			if !ok || data == "[DONE]" {
				break
			}
			var ev struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
					FinishReason string `json:"finish_reason"`
				} `json:"choices"`
				Usage *oaUsage `json:"usage"`
			}
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue // ignore unparseable keep-alives
			}
			if len(ev.Choices) > 0 {
				c := ev.Choices[0]
				if c.Delta.Content != "" {
					ch <- Chunk{Type: ChunkTextDelta, Text: c.Delta.Content}
				}
				if c.FinishReason != "" {
					stop = oaStopReason(c.FinishReason)
				}
			}
			if ev.Usage != nil {
				u := ev.Usage.toUsage()
				ch <- Chunk{Type: ChunkUsage, Usage: &u}
			}
		}
		ch <- Chunk{Type: ChunkMessageStop, StopReason: stop}
	}()
	return ch, nil
}

func (o *openAI) ListModels(ctx context.Context) ([]ModelInfo, error) {
	resp, err := send(ctx, "GET", o.baseURL+"/models", o.headers(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode %s models: %w", o.name, err)
	}
	models := make([]ModelInfo, len(out.Data))
	for i, m := range out.Data {
		models[i] = ModelInfo{ID: m.ID}
	}
	return models, nil
}

type oaUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func (u oaUsage) toUsage() Usage {
	return Usage{InputTokens: u.PromptTokens, OutputTokens: u.CompletionTokens}
}

func oaStopReason(finish string) string {
	switch finish {
	case "stop", "":
		return StopEndTurn
	case "length":
		return StopMaxTokens
	case "tool_calls":
		return StopToolUse
	}
	return finish
}

// textOf concatenates a message's text parts.
func textOf(m Message) string {
	var s string
	for _, p := range m.Content {
		if p.Type == PartText {
			s += p.Text
		}
	}
	return s
}
