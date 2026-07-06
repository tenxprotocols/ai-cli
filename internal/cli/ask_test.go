package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tenxprotocols/ai-cli/internal/providers"
)

// scriptedProvider returns a canned reply and records the last request.
type scriptedProvider struct {
	name    string
	reply   string
	lastReq providers.Request
}

func (s *scriptedProvider) Name() string { return s.name }

func (s *scriptedProvider) Complete(_ context.Context, req providers.Request) (providers.Response, error) {
	s.lastReq = req
	return providers.Response{
		Messages: []providers.Message{{
			Role:    providers.RoleAssistant,
			Content: []providers.ContentPart{{Type: providers.PartText, Text: s.reply}},
		}},
		StopReason: providers.StopEndTurn,
	}, nil
}

func (s *scriptedProvider) Stream(_ context.Context, req providers.Request) (<-chan providers.Chunk, error) {
	s.lastReq = req
	ch := make(chan providers.Chunk, len(s.reply)+1)
	for _, r := range s.reply {
		ch <- providers.Chunk{Type: providers.ChunkTextDelta, Text: string(r)}
	}
	ch <- providers.Chunk{Type: providers.ChunkMessageStop, StopReason: providers.StopEndTurn}
	close(ch)
	return ch, nil
}

func (s *scriptedProvider) ListModels(context.Context) ([]providers.ModelInfo, error) {
	return []providers.ModelInfo{{ID: "fake-model", DisplayName: "Fake"}}, nil
}

const fakeConfig = `
default_profile = "default"
[providers.fake]
type = "openai-compat"
base_url = "http://unused.invalid"
[profiles.default]
provider = "fake"
model    = "fake-model"
`

// runWithFake executes the CLI with a scripted provider injected and returns
// stdout.
func runWithFake(t *testing.T, fake *scriptedProvider, args ...string) (string, error) {
	t.Helper()
	p := writeTempConfig(t, fakeConfig)
	cmd := NewRoot()
	cmd.SetContext(WithInjectedProvider(context.Background(), "fake", fake))
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(append([]string{"--config", p}, args...))
	err := cmd.Execute()
	return buf.String(), err
}

func TestAsk_Streams(t *testing.T) {
	out, err := runWithFake(t, &scriptedProvider{name: "fake", reply: "hello"}, "ask", "say", "hi")
	require.NoError(t, err)
	assert.Equal(t, "hello\n", out)
}

func TestAsk_JoinsArgsIntoPrompt(t *testing.T) {
	fake := &scriptedProvider{name: "fake", reply: "ok"}
	_, err := runWithFake(t, fake, "ask", "say", "hi")
	require.NoError(t, err)
	assert.Equal(t, "say hi", fake.lastReq.Messages[0].Content[0].Text)
	assert.Equal(t, "fake-model", fake.lastReq.Model)
}

func TestAsk_NonStreamJSON(t *testing.T) {
	out, err := runWithFake(t, &scriptedProvider{name: "fake", reply: "json body"},
		"--format", "json", "--no-stream", "ask", "hi")
	require.NoError(t, err)
	assert.Contains(t, out, `"text":"json body"`)
}

func TestAsk_EmptyPromptErrors(t *testing.T) {
	_, err := runWithFake(t, &scriptedProvider{name: "fake", reply: "x"}, "ask")
	assert.Error(t, err)
}

func TestAsk_SystemFlagReachesProvider(t *testing.T) {
	fake := &scriptedProvider{name: "fake", reply: "ok"}
	_, err := runWithFake(t, fake, "--system", "be terse", "ask", "hi")
	require.NoError(t, err)
	assert.Equal(t, "be terse", fake.lastReq.System)
}
