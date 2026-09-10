package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLevel(t *testing.T) {
	for name, want := range map[string]slog.Level{
		"error": LevelError,
		"warn":  LevelWarn,
		"info":  LevelInfo,
		"debug": LevelDebug,
		"trace": LevelTrace,
		"WARN":  LevelWarn,
		"":      LevelWarn,
	} {
		got, err := ParseLevel(name)
		require.NoError(t, err, "level %q", name)
		assert.Equal(t, want, got, "level %q", name)
	}
}

func TestParseLevel_UnknownListsValidNames(t *testing.T) {
	_, err := ParseLevel("chatty")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chatty")
	for _, name := range LevelNames() {
		assert.Contains(t, err.Error(), name)
	}
}

func TestParseFormat(t *testing.T) {
	for name, want := range map[string]Format{"text": FormatText, "json": FormatJSON, "": FormatText} {
		got, err := ParseFormat(name)
		require.NoError(t, err, "format %q", name)
		assert.Equal(t, want, got)
	}
	_, err := ParseFormat("yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yaml")
}

func TestSink_FiltersBelowLevel(t *testing.T) {
	var buf bytes.Buffer
	sink, err := Open(Options{Level: LevelInfo}, &buf)
	require.NoError(t, err)
	defer sink.Close()

	log := sink.Logger()
	log.Debug("hidden")
	log.Info("shown")
	log.Log(context.Background(), LevelTrace, "also hidden")

	out := buf.String()
	assert.Contains(t, out, "shown")
	assert.NotContains(t, out, "hidden")
}

func TestSink_SetLevelTakesEffectLive(t *testing.T) {
	var buf bytes.Buffer
	sink, err := Open(Options{Level: LevelWarn}, &buf)
	require.NoError(t, err)
	defer sink.Close()

	sink.Logger().Debug("before")
	sink.SetLevel(LevelTrace)
	sink.Logger().Debug("after")

	assert.NotContains(t, buf.String(), "before")
	assert.Contains(t, buf.String(), "after")
	assert.Equal(t, LevelTrace, sink.Level())
}

func TestSink_TraceRendersAsTrace(t *testing.T) {
	var buf bytes.Buffer
	sink, err := Open(Options{Level: LevelTrace}, &buf)
	require.NoError(t, err)
	defer sink.Close()

	sink.Logger().Log(context.Background(), LevelTrace, "wire", "method", "POST")

	out := buf.String()
	assert.Contains(t, out, "TRACE")
	assert.NotContains(t, out, "DEBUG-4")
	assert.Contains(t, out, "wire")
	assert.Contains(t, out, "method=POST")
}

func TestSink_TextOmitsTimestampAboveDebug(t *testing.T) {
	var quiet, loud bytes.Buffer

	sink, err := Open(Options{Level: LevelInfo}, &quiet)
	require.NoError(t, err)
	sink.Logger().Info("milestone")
	require.NoError(t, sink.Close())
	assert.NotContains(t, quiet.String(), "+", "info-level output carries no relative stamp")

	debugSink, err := Open(Options{Level: LevelDebug}, &loud)
	require.NoError(t, err)
	debugSink.Logger().Debug("step")
	require.NoError(t, debugSink.Close())
	assert.Regexp(t, `^\+\d+ms `, loud.String(), "debug output leads with a relative stamp")
}

func TestSink_QuotesValuesWithSpaces(t *testing.T) {
	var buf bytes.Buffer
	sink, err := Open(Options{Level: LevelInfo}, &buf)
	require.NoError(t, err)
	defer sink.Close()

	sink.Logger().Info("resolved", "model", "claude opus 5", "empty", "")
	assert.Contains(t, buf.String(), `model="claude opus 5"`)
	assert.Contains(t, buf.String(), `empty=""`)
}

func TestSink_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	sink, err := Open(Options{Level: LevelTrace, Format: FormatJSON}, &buf)
	require.NoError(t, err)
	defer sink.Close()

	sink.Logger().Log(context.Background(), LevelTrace, "wire", "method", "POST")

	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &record))
	assert.Equal(t, "TRACE", record["level"])
	assert.Equal(t, "wire", record["msg"])
	assert.Equal(t, "POST", record["method"])
}

func TestSink_FileDestinationIsPrivateAndNotStderr(t *testing.T) {
	var stderr bytes.Buffer
	path := filepath.Join(t.TempDir(), "ai.log")

	sink, err := Open(Options{Level: LevelDebug, File: path}, &stderr)
	require.NoError(t, err)
	sink.Logger().Debug("to the file")
	require.NoError(t, sink.Close())

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(contents), "to the file")
	assert.Empty(t, stderr.String(), "stderr stays clean when --log-file is set")

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "traces can contain prompts and keys")
}

func TestSink_FileOpenFailureIsReported(t *testing.T) {
	_, err := Open(Options{Level: LevelDebug, File: filepath.Join(t.TempDir(), "nope", "ai.log")}, &bytes.Buffer{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ai.log")
}

func TestSink_ReopenSwitchesFileAndKeepsOneHandle(t *testing.T) {
	dir := t.TempDir()
	first, second := filepath.Join(dir, "one.log"), filepath.Join(dir, "two.log")

	sink, err := Open(Options{Level: LevelDebug, File: first}, &bytes.Buffer{})
	require.NoError(t, err)
	sink.Logger().Debug("first")

	require.NoError(t, sink.Reopen(Options{Level: LevelTrace, File: second}, &bytes.Buffer{}))
	sink.Logger().Debug("second")
	require.NoError(t, sink.Close())

	one, err := os.ReadFile(first)
	require.NoError(t, err)
	two, err := os.ReadFile(second)
	require.NoError(t, err)
	assert.Contains(t, string(one), "first")
	assert.NotContains(t, string(one), "second")
	assert.Contains(t, string(two), "second")
	assert.Equal(t, LevelTrace, sink.Level())
}

func TestSink_ReopenSameFileKeepsContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai.log")
	sink, err := Open(Options{Level: LevelDebug, File: path}, &bytes.Buffer{})
	require.NoError(t, err)
	sink.Logger().Debug("first")

	require.NoError(t, sink.Reopen(Options{Level: LevelDebug, File: path}, &bytes.Buffer{}))
	sink.Logger().Debug("second")
	require.NoError(t, sink.Close())

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(contents), "first", "reopening the same path must not truncate")
	assert.Contains(t, string(contents), "second")
}

func TestContext_RoundTrip(t *testing.T) {
	var buf bytes.Buffer
	sink, err := Open(Options{Level: LevelDebug}, &buf)
	require.NoError(t, err)
	defer sink.Close()

	ctx := NewContext(context.Background(), sink.Logger())
	FromContext(ctx).Debug("from context")
	assert.Contains(t, buf.String(), "from context")
}

func TestFromContext_DiscardsWhenAbsent(t *testing.T) {
	log := FromContext(context.Background())
	require.NotNil(t, log)
	log.Error("must not panic")
	assert.False(t, log.Enabled(context.Background(), LevelError))
}
