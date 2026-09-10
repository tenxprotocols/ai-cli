package logging

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// sensitiveHeaders carry credentials and are logged as ••••<last4> unless
// Secrets is set.
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"x-api-key":           true,
	"api-key":             true,
	"x-goog-api-key":      true,
	"cookie":              true,
	"set-cookie":          true,
}

// sensitiveParams carry credentials in the query string, which is how
// Google's endpoints take a key.
var sensitiveParams = map[string]bool{
	"key":          true,
	"api_key":      true,
	"access_token": true,
}

// Transport logs HTTP traffic as it passes through: one line each way at
// debug, full headers and bodies at trace, and the underlying cause at error.
// Response bodies are teed as they are read rather than buffered, so a
// streaming response still reaches the caller chunk by chunk.
type Transport struct {
	Base    http.RoundTripper // nil uses http.DefaultTransport
	Log     *slog.Logger      // nil discards
	Secrets bool              // true logs credentials unredacted
}

// NewClient copies base — or starts from a zero client — and wraps whatever
// transport it had in a logging one.
func NewClient(base *http.Client, log *slog.Logger, secrets bool) *http.Client {
	client := &http.Client{}
	if base != nil {
		*client = *base
	}
	client.Transport = &Transport{Base: client.Transport, Log: log, Secrets: secrets}
	return client
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	log := t.Log
	if log == nil {
		log = Discard()
	}
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	ctx := req.Context()
	loggedURL := t.redactURL(req.URL)

	if log.Enabled(ctx, LevelDebug) {
		log.Debug("http: request", "method", req.Method, "url", loggedURL)
	}
	trace := log.Enabled(ctx, LevelTrace)
	if trace {
		log.Log(ctx, LevelTrace, "http: request headers", t.headerAttrs(req.Header)...)
		var err error
		if req, err = logRequestBody(ctx, log, req, loggedURL); err != nil {
			return nil, err
		}
	}

	start := time.Now()
	resp, err := base.RoundTrip(req)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		log.Error("http: request failed",
			"method", req.Method, "url", loggedURL, "dur", elapsed, "err", err)
		return nil, err
	}

	if log.Enabled(ctx, LevelDebug) {
		log.Debug("http: response", "status", resp.StatusCode, "dur", elapsed)
	}
	if trace {
		log.Log(ctx, LevelTrace, "http: response headers", t.headerAttrs(resp.Header)...)
		resp.Body = &teeBody{inner: resp.Body, log: log, ctx: ctx}
	}
	return resp, nil
}

// logRequestBody dumps the outbound body and returns a request that can still
// send it. Only called at trace, so ordinary runs never buffer a body.
func logRequestBody(ctx context.Context, log *slog.Logger, req *http.Request, loggedURL string) (*http.Request, error) {
	if req.Body == nil {
		return req, nil
	}
	data, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		log.Error("http: reading request body for the log failed", "url", loggedURL, "err", err)
		return nil, err
	}
	log.Log(ctx, LevelTrace, "http: request body", "body", string(data))

	clone := req.Clone(ctx)
	clone.Body = io.NopCloser(bytes.NewReader(data))
	clone.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(data)), nil
	}
	clone.ContentLength = int64(len(data))
	return clone, nil
}

// headerAttrs renders headers as sorted key=value log attributes, redacting
// credentials unless Secrets is set.
func (t *Transport) headerAttrs(header http.Header) []any {
	names := make([]string, 0, len(header))
	for name := range header {
		names = append(names, name)
	}
	sort.Strings(names)

	attrs := make([]any, 0, len(names)*2)
	for _, name := range names {
		key := strings.ToLower(name)
		value := strings.Join(header[name], ", ")
		if !t.Secrets && sensitiveHeaders[key] {
			value = Redact(value)
		}
		attrs = append(attrs, key, value)
	}
	return attrs
}

// redactURL rewrites credential-carrying query parameters for the log. It
// edits the raw query directly so the redaction marker is not re-escaped.
func (t *Transport) redactURL(target *url.URL) string {
	if target == nil {
		return ""
	}
	if t.Secrets || target.RawQuery == "" {
		return target.String()
	}
	pairs := strings.Split(target.RawQuery, "&")
	for i, pair := range pairs {
		name, value, found := strings.Cut(pair, "=")
		if !found || !sensitiveParams[strings.ToLower(name)] {
			continue
		}
		pairs[i] = name + "=" + Redact(value)
	}
	clone := *target
	clone.RawQuery = strings.Join(pairs, "&")
	return clone.String()
}

// Redact keeps the last four characters so a credential can be identified
// without being disclosed, matching `ai config show`.
func Redact(value string) string {
	if len(value) <= 4 {
		return "••••"
	}
	return "••••" + value[len(value)-4:]
}

// teeBody logs each chunk of a response body as the caller reads it.
type teeBody struct {
	inner io.ReadCloser
	log   *slog.Logger
	ctx   context.Context
}

func (b *teeBody) Read(p []byte) (int, error) {
	n, err := b.inner.Read(p)
	if n > 0 {
		b.log.Log(b.ctx, LevelTrace, "http: response body", "chunk", string(p[:n]))
	}
	if err != nil && !errors.Is(err, io.EOF) {
		b.log.Error("http: reading response body failed", "err", err)
	}
	return n, err
}

func (b *teeBody) Close() error { return b.inner.Close() }
