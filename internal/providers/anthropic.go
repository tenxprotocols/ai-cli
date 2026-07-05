package providers

import (
	"context"
	"encoding/json"
	"fmt"
)

const (
	anthropicVersion   = "2023-06-01"
	anthropicMaxTokens = 4096 // API requires max_tokens; used when unset
)

type anthropic struct {
	name    string
	baseURL string
	apiKey  string
}

func newAnthropic(cfg Config) (Provider, error) {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	return &anthropic{name: cfg.Name, baseURL: base, apiKey: cfg.APIKey}, nil
}

func (a *anthropic) Name() string { return a.name }

func (a *anthropic) headers() map[string]string {
	return map[string]string{
		"x-api-key":         a.apiKey,
		"anthropic-version": anthropicVersion,
	}
}

func (a *anthropic) body(req Request) map[string]any {
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := make([]msg, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, msg{m.Role, textOf(m)})
	}
	maxTokens := anthropicMaxTokens
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	body := map[string]any{"model": req.Model, "messages": msgs, "max_tokens": maxTokens}
	if req.System != "" {
		body["system"] = req.System
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	return body
}

type anUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func (a *anthropic) Complete(ctx context.Context, req Request) (Response, error) {
	resp, err := send(ctx, "POST", a.baseURL+"/v1/messages", a.headers(), a.body(req))
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string  `json:"stop_reason"`
		Usage      anUsage `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, fmt.Errorf("decode %s response: %w", a.name, err)
	}
	var parts []ContentPart
	for _, c := range out.Content {
		if c.Type == "text" {
			parts = append(parts, ContentPart{Type: PartText, Text: c.Text})
		}
	}
	return Response{
		Messages:   []Message{{Role: RoleAssistant, Content: parts}},
		Usage:      Usage{InputTokens: out.Usage.InputTokens, OutputTokens: out.Usage.OutputTokens},
		StopReason: out.StopReason,
	}, nil
}

func (a *anthropic) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
	body := a.body(req)
	body["stream"] = true
	resp, err := send(ctx, "POST", a.baseURL+"/v1/messages", a.headers(), body)
	if err != nil {
		return nil, err
	}

	ch := make(chan Chunk, 16)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		var usage Usage
		stop := StopEndTurn
		sc := newSSEScanner(resp.Body)
		for {
			name, data, ok := sc.Next()
			if !ok {
				break
			}
			var ev struct {
				Message struct {
					Usage anUsage `json:"usage"`
				} `json:"message"`
				Delta struct {
					Type       string `json:"type"`
					Text       string `json:"text"`
					Thinking   string `json:"thinking"`
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage anUsage `json:"usage"`
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue
			}
			switch name {
			case "message_start":
				usage.InputTokens = ev.Message.Usage.InputTokens
			case "content_block_delta":
				switch ev.Delta.Type {
				case "text_delta":
					ch <- Chunk{Type: ChunkTextDelta, Text: ev.Delta.Text}
				case "thinking_delta":
					ch <- Chunk{Type: ChunkThinkingDelta, Text: ev.Delta.Thinking}
				}
			case "message_delta":
				if ev.Delta.StopReason != "" {
					stop = ev.Delta.StopReason
				}
				usage.OutputTokens = ev.Usage.OutputTokens
			case "error":
				ch <- Chunk{Type: ChunkError, Err: &APIError{Status: 0, Message: ev.Error.Message}}
				return
			}
		}
		ch <- Chunk{Type: ChunkUsage, Usage: &usage}
		ch <- Chunk{Type: ChunkMessageStop, StopReason: stop}
	}()
	return ch, nil
}

func (a *anthropic) ListModels(ctx context.Context) ([]ModelInfo, error) {
	resp, err := send(ctx, "GET", a.baseURL+"/v1/models", a.headers(), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode %s models: %w", a.name, err)
	}
	models := make([]ModelInfo, len(out.Data))
	for i, m := range out.Data {
		models[i] = ModelInfo{ID: m.ID, DisplayName: m.DisplayName}
	}
	return models, nil
}
