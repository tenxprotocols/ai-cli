package logging

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSink returns a sink writing to buf, plus the buffer.
func newTestSink(t *testing.T, opts Options) (*Sink, *bytes.Buffer) {
	t.Helper()
	buf := &bytes.Buffer{}
	sink, err := Open(opts, buf)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sink.Close() })
	return sink, buf
}

// stubRoundTripper returns a canned response or error without a network.
type stubRoundTripper struct {
	resp *http.Response
	err  error
	seen *http.Request
	body string
}

func (s *stubRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	s.seen = req
	if req.Body != nil {
		data, _ := io.ReadAll(req.Body)
		s.body = string(data)
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func okResponse(body string) *http.Response {
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestTransport_WarnLevelLogsNothingAndPassesThrough(t *testing.T) {
	sink, buf := newTestSink(t, Options{Level: LevelWarn})
	stub := &stubRoundTripper{resp: okResponse(`{"ok":true}`)}
	client := NewClient(nil, sink.Logger(), false)
	client.Transport = &Transport{Base: stub, Log: sink.Logger()}

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(`{"model":"x"}`))
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"ok":true}`, string(body), "response reaches the caller intact")
	assert.Equal(t, `{"model":"x"}`, stub.body, "request body reaches the server intact")
	assert.Empty(t, buf.String(), "default level logs no HTTP traffic")
}

func TestTransport_DebugLogsLinesWithoutBodies(t *testing.T) {
	sink, buf := newTestSink(t, Options{Level: LevelDebug})
	stub := &stubRoundTripper{resp: okResponse(`{"secret":"payload"}`)}
	transport := &Transport{Base: stub, Log: sink.Logger()}

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(`{"model":"claude"}`))
	require.NoError(t, err)
	req.Header.Set("x-api-key", "sk-ant-api03-longsecret1a2b")
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	_, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	out := buf.String()
	assert.Contains(t, out, "http: request")
	assert.Contains(t, out, "method=POST")
	assert.Contains(t, out, "url=https://api.anthropic.com/v1/messages")
	assert.Contains(t, out, "http: response")
	assert.Contains(t, out, "status=200")
	assert.Regexp(t, `dur=\d`, out)
	assert.NotContains(t, out, "claude", "debug does not dump the request body")
	assert.NotContains(t, out, "payload", "debug does not dump the response body")
	assert.NotContains(t, out, "sk-ant", "debug does not dump headers")
}

func TestTransport_TraceDumpsHeadersAndBodies(t *testing.T) {
	sink, buf := newTestSink(t, Options{Level: LevelTrace})
	stub := &stubRoundTripper{resp: okResponse(`{"content":"hello there"}`)}
	transport := &Transport{Base: stub, Log: sink.Logger()}

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", strings.NewReader(`{"model":"claude-opus-5"}`))
	require.NoError(t, err)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, `{"content":"hello there"}`, string(body), "teeing the body does not corrupt it")
	assert.Equal(t, `{"model":"claude-opus-5"}`, stub.body, "reading the request body for the log does not consume it")

	out := buf.String()
	assert.Contains(t, out, "http: request headers")
	assert.Contains(t, out, "anthropic-version=2023-06-01")
	assert.Contains(t, out, "http: request body")
	assert.Contains(t, out, "claude-opus-5")
	assert.Contains(t, out, "http: response headers")
	assert.Contains(t, out, "content-type=application/json")
	assert.Contains(t, out, "http: response body")
	assert.Contains(t, out, "hello there")
}

func TestTransport_RedactsCredentialHeadersByDefault(t *testing.T) {
	sink, buf := newTestSink(t, Options{Level: LevelTrace})
	stub := &stubRoundTripper{resp: okResponse(`{}`)}
	transport := &Transport{Base: stub, Log: sink.Logger()}

	req, err := http.NewRequest(http.MethodGet, "https://api.openai.com/v1/models", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer sk-proj-supersecretvalue9f7c")
	req.Header.Set("x-api-key", "sk-ant-api03-anothersecret1a2b")
	req.Header.Set("Cookie", "session=abcdefghijkl")
	req.Header.Set("User-Agent", "ai-cli/0.2.0")
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()

	out := buf.String()
	assert.NotContains(t, out, "supersecretvalue")
	assert.NotContains(t, out, "anothersecret")
	assert.NotContains(t, out, "abcdefghijkl")
	assert.Contains(t, out, "••••9f7c", "last four characters survive for identification")
	assert.Contains(t, out, "••••1a2b")
	assert.Contains(t, out, "user-agent=ai-cli/0.2.0", "ordinary headers are untouched")
	assert.Equal(t, "Bearer sk-proj-supersecretvalue9f7c", stub.seen.Header.Get("Authorization"),
		"redaction is only for the log, never the wire")
}

func TestTransport_LogSecretsPrintsCredentialsInFull(t *testing.T) {
	sink, buf := newTestSink(t, Options{Level: LevelTrace, Secrets: true})
	stub := &stubRoundTripper{resp: okResponse(`{}`)}
	transport := &Transport{Base: stub, Log: sink.Logger(), Secrets: true}

	req, err := http.NewRequest(http.MethodGet, "https://api.openai.com/v1/models", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer sk-proj-supersecretvalue9f7c")
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Contains(t, buf.String(), "Bearer sk-proj-supersecretvalue9f7c")
}

func TestTransport_RedactsKeyQueryParameter(t *testing.T) {
	sink, buf := newTestSink(t, Options{Level: LevelDebug})
	stub := &stubRoundTripper{resp: okResponse(`{}`)}
	transport := &Transport{Base: stub, Log: sink.Logger()}

	req, err := http.NewRequest(http.MethodGet,
		"https://generativelanguage.googleapis.com/v1beta/models?key=AIzaSyLongGoogleKey4f2e&alt=sse", nil)
	require.NoError(t, err)
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	resp.Body.Close()

	out := buf.String()
	assert.NotContains(t, out, "AIzaSyLongGoogleKey")
	assert.Contains(t, out, "••••4f2e")
	assert.Contains(t, out, "alt=sse", "other query parameters stay readable")
	assert.Equal(t, "AIzaSyLongGoogleKey4f2e", stub.seen.URL.Query().Get("key"),
		"the outbound URL keeps its key")
}

func TestTransport_LogsTransportErrorAtDefaultLevel(t *testing.T) {
	sink, buf := newTestSink(t, Options{Level: LevelWarn})
	stub := &stubRoundTripper{err: errors.New("dial tcp 160.79.104.10:443: i/o timeout")}
	transport := &Transport{Base: stub, Log: sink.Logger()}

	req, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", nil)
	require.NoError(t, err)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose // the request failed; there is no body
	require.Error(t, err)
	require.Nil(t, resp)

	out := buf.String()
	assert.Contains(t, out, "ERROR")
	assert.Contains(t, out, "http: request failed")
	assert.Contains(t, out, "i/o timeout")
	assert.Contains(t, out, "url=https://api.anthropic.com/v1/messages")
}

func TestTransport_ExpectedFailureIsSilentAtDefaultLevel(t *testing.T) {
	sink, buf := newTestSink(t, Options{Level: LevelWarn})
	stub := &stubRoundTripper{err: errors.New("dial tcp [::1]:11434: connect: connection refused")}
	transport := &Transport{Base: stub, Log: sink.Logger(), ExpectedFailure: true}

	req, err := http.NewRequest(http.MethodGet, "http://localhost:11434/v1/models", nil)
	require.NoError(t, err)
	resp, err := transport.RoundTrip(req) //nolint:bodyclose // the request failed; there is no body
	require.Error(t, err, "the caller still learns the request failed")
	require.Nil(t, resp)

	assert.Empty(t, buf.String(),
		"a probe that is allowed to fail must not shout at the default level")
}

func TestTransport_ExpectedFailureStillVisibleWhenDebugging(t *testing.T) {
	sink, buf := newTestSink(t, Options{Level: LevelDebug})
	stub := &stubRoundTripper{err: errors.New("dial tcp [::1]:11434: connect: connection refused")}
	transport := &Transport{Base: stub, Log: sink.Logger(), ExpectedFailure: true}

	req, err := http.NewRequest(http.MethodGet, "http://localhost:11434/v1/models", nil)
	require.NoError(t, err)
	_, err = transport.RoundTrip(req) //nolint:bodyclose // the request failed; there is no body
	require.Error(t, err)

	out := buf.String()
	assert.Contains(t, out, "DEBUG")
	assert.NotContains(t, out, "ERROR", "an expected failure is never an error")
	assert.Contains(t, out, "connection refused")
	assert.Contains(t, out, "localhost:11434")
}

func TestNewProbeClient_TreatsFailuresAsExpected(t *testing.T) {
	sink, _ := newTestSink(t, Options{Level: LevelDebug})

	client := NewProbeClient(&http.Client{Timeout: 300 * time.Millisecond}, sink.Logger(), false)

	assert.Equal(t, 300*time.Millisecond, client.Timeout)
	transport, ok := client.Transport.(*Transport)
	require.True(t, ok)
	assert.True(t, transport.ExpectedFailure)
}

func TestNewClient_TreatsFailuresAsRealByDefault(t *testing.T) {
	sink, _ := newTestSink(t, Options{Level: LevelDebug})

	client := NewClient(nil, sink.Logger(), false)

	transport, ok := client.Transport.(*Transport)
	require.True(t, ok)
	assert.False(t, transport.ExpectedFailure,
		"an ordinary provider call failing is a real error")
}

func TestTransport_StreamsSSEWithoutBuffering(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok)
		_, _ = io.WriteString(w, "data: {\"delta\":\"first\"}\n\n")
		flusher.Flush()
		<-release // hold the stream open until the client has read chunk one
		_, _ = io.WriteString(w, "data: {\"delta\":\"second\"}\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	sink, buf := newTestSink(t, Options{Level: LevelTrace})
	client := NewClient(nil, sink.Logger(), false)

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	chunk := make([]byte, 64)
	n, err := resp.Body.Read(chunk)
	require.NoError(t, err)
	assert.Contains(t, string(chunk[:n]), "first")

	out := buf.String()
	assert.Contains(t, out, "first", "the first chunk is logged as it arrives")
	assert.NotContains(t, out, "second", "the transport does not wait for the stream to finish")

	close(release)
	rest, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(rest), "second")
	assert.Contains(t, buf.String(), "second", "later chunks are logged too")
}

func TestTransport_NilLogAndBaseAreSafe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "pong")
	}))
	defer server.Close()

	client := &http.Client{Transport: &Transport{}}
	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "pong", string(body))
}

func TestNewClient_PreservesBaseClientSettings(t *testing.T) {
	sink, _ := newTestSink(t, Options{Level: LevelDebug})
	base := &http.Client{Timeout: 300 * time.Millisecond}

	client := NewClient(base, sink.Logger(), false)

	assert.Equal(t, 300*time.Millisecond, client.Timeout)
	assert.NotSame(t, base, client, "the base client is not mutated")
	transport, ok := client.Transport.(*Transport)
	require.True(t, ok)
	assert.Equal(t, sink.Logger(), transport.Log)
}
