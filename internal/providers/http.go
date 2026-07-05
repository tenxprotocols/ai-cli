package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// APIError is a non-2xx response from a provider API.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("provider error (HTTP %d): %s", e.Status, e.Message)
}

// send issues a JSON request and returns the response, translating non-2xx
// statuses into *APIError. Callers own resp.Body.
func send(ctx context.Context, method, url string, headers map[string]string, body any) (*http.Response, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		msg := strings.TrimSpace(string(b))
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(b, &e) == nil && e.Error.Message != "" {
			msg = e.Error.Message
		}
		return nil, &APIError{Status: resp.StatusCode, Message: msg}
	}
	return resp, nil
}

// sseScanner iterates server-sent events on a response body.
type sseScanner struct {
	s *bufio.Scanner
}

func newSSEScanner(r io.Reader) *sseScanner {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64<<10), 1<<20)
	return &sseScanner{s: s}
}

// Next returns the next event's name and data. ok is false at end of stream.
func (x *sseScanner) Next() (name, data string, ok bool) {
	var lines []string
	for x.s.Scan() {
		line := x.s.Text()
		switch {
		case line == "":
			if len(lines) > 0 {
				return name, strings.Join(lines, "\n"), true
			}
			name = ""
		case strings.HasPrefix(line, "event:"):
			name = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			lines = append(lines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if len(lines) > 0 {
		return name, strings.Join(lines, "\n"), true
	}
	return "", "", false
}
