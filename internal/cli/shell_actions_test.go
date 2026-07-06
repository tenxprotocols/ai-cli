package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runShellInteractive drives `ai shell` with a fake TTY, capturing clipboard
// and run invocations. Returns stdout and stderr separately — stdout must
// stay pure.
func runShellInteractive(t *testing.T, answer string) (stdout, stderr string, copied, ran []string) {
	t.Helper()
	restoreTTY, restoreCopy, restoreRun := stdioIsTTY, copyToClipboard, runCommand
	stdioIsTTY = func() bool { return true }
	copyToClipboard = func(text string) error { copied = append(copied, text); return nil }
	runCommand = func(command string) error { ran = append(ran, command); return nil }
	t.Cleanup(func() { stdioIsTTY, copyToClipboard, runCommand = restoreTTY, restoreCopy, restoreRun })

	path := writeTempConfig(t, fakeConfig)
	cmd := NewRoot()
	cmd.SetContext(WithInjectedProvider(context.Background(), "fake",
		&scriptedProvider{name: "fake", reply: "ls -la"}))
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader(answer))
	cmd.SetArgs([]string{"--config", path, "shell", "list", "files"})
	require.NoError(t, cmd.Execute())
	return out.String(), errOut.String(), copied, ran
}

func TestShellAction_DefaultIsCopy(t *testing.T) {
	stdout, stderr, copied, ran := runShellInteractive(t, "\n")
	assert.Equal(t, "ls -la\n", stdout, "stdout carries only the command")
	assert.Contains(t, stderr, "[C/r/n]")
	assert.Contains(t, stderr, "copied")
	assert.Equal(t, []string{"ls -la"}, copied)
	assert.Empty(t, ran)
}

func TestShellAction_Run(t *testing.T) {
	stdout, _, copied, ran := runShellInteractive(t, "r\n")
	assert.Equal(t, "ls -la\n", stdout)
	assert.Equal(t, []string{"ls -la"}, ran)
	assert.Empty(t, copied)
}

func TestShellAction_Nothing(t *testing.T) {
	_, _, copied, ran := runShellInteractive(t, "n\n")
	assert.Empty(t, copied)
	assert.Empty(t, ran)
}

func TestShellAction_NoPromptWhenPiped(t *testing.T) {
	// Default test environment: stdioIsTTY() is false (pipes).
	fake := &scriptedProvider{name: "fake", reply: "ls -la"}
	out, err := runWithFake(t, fake, "shell", "list", "files")
	require.NoError(t, err)
	assert.Equal(t, "ls -la\n", out, "no prompt text anywhere in piped mode")
}
