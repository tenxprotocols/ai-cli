package cli

import (
	"bytes"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tenxprotocols/ai-cli/internal/logging"
)

func TestPeekLogOptions_FindsLevelWhereverItAppears(t *testing.T) {
	for name, argv := range map[string][]string{
		"before the subcommand":    {"--log-level=debug", "ask", "hi"},
		"after the subcommand":     {"ask", "--log-level=debug", "hi"},
		"space form":               {"--log-level", "debug", "ask", "hi"},
		"after a plugin name":      {"deploy", "--log-level=debug", "prod"},
		"among unknown flags":      {"config", "show", "--show-secrets", "--log-level=debug"},
		"after a valued flag":      {"--model", "claude-opus-5", "--log-level=debug", "ask", "hi"},
		"with the prompt trailing": {"--log-level=debug", "what", "is", "2+2"},
	} {
		options, err := PeekLogOptions(argv)
		require.NoError(t, err, name)
		assert.Equal(t, logging.LevelDebug, options.Level, name)
	}
}

func TestPeekLogOptions_StopsAtBareDoubleDash(t *testing.T) {
	options, err := PeekLogOptions([]string{"ask", "--", "--log-level=trace", "what", "does", "this", "do"})
	require.NoError(t, err)
	assert.Equal(t, logging.DefaultLevel, options.Level,
		"a log flag after -- is prompt text, not ours")
}

func TestPeekLogOptions_DoesNotModifyArgv(t *testing.T) {
	argv := []string{"--log-level=trace", "ask", "hi"}
	before := slices.Clone(argv)

	_, err := PeekLogOptions(argv)
	require.NoError(t, err)

	assert.Equal(t, before, argv, "peeking must not consume or reorder arguments")
}

func TestPeekLogOptions_ReadsFormatFileAndSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai.log")
	options, err := PeekLogOptions([]string{
		"--log-level", "trace", "--log-format", "json", "--log-file", path, "--log-secrets", "ask", "hi",
	})
	require.NoError(t, err)

	assert.Equal(t, logging.LevelTrace, options.Level)
	assert.Equal(t, logging.FormatJSON, options.Format)
	assert.Equal(t, path, options.File)
	assert.True(t, options.Secrets)
}

func TestPeekLogOptions_DefaultsWhenAbsent(t *testing.T) {
	options, err := PeekLogOptions([]string{"ask", "hi"})
	require.NoError(t, err)

	assert.Equal(t, logging.DefaultLevel, options.Level)
	assert.Equal(t, logging.FormatText, options.Format)
	assert.Empty(t, options.File)
	assert.False(t, options.Secrets)
}

func TestPeekLogOptions_EnvVarsApplyAndFlagsWin(t *testing.T) {
	t.Setenv("AI_CLI_LOG_LEVEL", "info")
	t.Setenv("AI_CLI_LOG_FORMAT", "json")

	options, err := PeekLogOptions([]string{"ask", "hi"})
	require.NoError(t, err)
	assert.Equal(t, logging.LevelInfo, options.Level)
	assert.Equal(t, logging.FormatJSON, options.Format)

	options, err = PeekLogOptions([]string{"--log-level=trace", "ask", "hi"})
	require.NoError(t, err)
	assert.Equal(t, logging.LevelTrace, options.Level, "the flag beats the env var")
}

func TestPeekLogOptions_InvalidLevelReportsAndFallsBack(t *testing.T) {
	options, err := PeekLogOptions([]string{"--log-level=chatty", "ask", "hi"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chatty")
	assert.Equal(t, logging.DefaultLevel, options.Level,
		"a bad level still yields a usable sink; cobra reports the error")
}

func TestRoot_RejectsInvalidLogLevel(t *testing.T) {
	root := NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--log-level=chatty", "version"})

	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "chatty")
	for _, name := range logging.LevelNames() {
		assert.Contains(t, err.Error(), name)
	}
}

func TestRoot_PutsLoggerOnTheCommandContext(t *testing.T) {
	root := NewRoot()
	var stderr bytes.Buffer
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.SetArgs([]string{"--log-level=debug", "version"})

	require.NoError(t, root.Execute())

	assert.Contains(t, stderr.String(), "cli: command", "the run is logged to stderr")
	assert.Contains(t, stderr.String(), "name=version")
}

func TestRoot_LogsGoToStderrNotStdout(t *testing.T) {
	root := NewRoot()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"--log-level=debug", "version"})

	require.NoError(t, root.Execute())

	assert.NotContains(t, stdout.String(), "DEBUG", "stdout stays pipeable")
	assert.Contains(t, stderr.String(), "DEBUG")
}

func TestRoot_LogFileKeepsStderrClean(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ai.log")
	root := NewRoot()
	var stderr bytes.Buffer
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&stderr)
	root.SetArgs([]string{"--log-level=debug", "--log-file", path, "version"})

	require.NoError(t, root.Execute())

	assert.Empty(t, stderr.String())
	assert.FileExists(t, path)
}

func TestResolveArgs_LogSecretsIsBooleanNotValued(t *testing.T) {
	known := map[string]bool{"ask": true, "version": true}

	got := ResolveArgs("ai", []string{"ai", "--log-secrets", "what", "is", "2+2"}, known, nil)

	assert.Equal(t, ResolveAskFallback, got.Kind)
	assert.Equal(t, []string{"--log-secrets", "ask", "what", "is", "2+2"}, got.Args,
		"--log-secrets takes no value, so the first prompt word must survive")
}

func TestResolveArgs_LogLevelTakesItsValue(t *testing.T) {
	known := map[string]bool{"ask": true, "version": true}

	got := ResolveArgs("ai", []string{"ai", "--log-level", "trace", "what", "is", "2+2"}, known, nil)

	assert.Equal(t, ResolveAskFallback, got.Kind)
	assert.Equal(t, []string{"--log-level", "trace", "ask", "what", "is", "2+2"}, got.Args)
}

func TestResolutionKind_String(t *testing.T) {
	assert.Equal(t, "builtin", ResolveBuiltin.String())
	assert.Equal(t, "plugin", ResolvePlugin.String())
	assert.Equal(t, "ask-fallback", ResolveAskFallback.String())
}
