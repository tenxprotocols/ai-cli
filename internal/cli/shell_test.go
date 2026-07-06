package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShell_PrintsCommand(t *testing.T) {
	fake := &scriptedProvider{name: "fake", reply: "find . -type f -size +500M -mtime -7"}
	out, err := runWithFake(t, fake, "shell", "find", "big", "recent", "files")
	require.NoError(t, err)
	assert.Equal(t, "find . -type f -size +500M -mtime -7\n", out)
	assert.Equal(t, "find big recent files", fake.lastReq.Messages[0].Content[0].Text)
	assert.Contains(t, fake.lastReq.System, "shell commands")
}

func TestShell_StripsFencesAndPromptMarker(t *testing.T) {
	fake := &scriptedProvider{name: "fake", reply: "```bash\n$ kubectl config get-contexts\n```"}
	out, err := runWithFake(t, fake, "shell", "show", "kubernetes", "contexts")
	require.NoError(t, err)
	assert.Equal(t, "kubectl config get-contexts\n", out)
}

func TestShell_SystemFlagOverridesBuiltinPrompt(t *testing.T) {
	fake := &scriptedProvider{name: "fake", reply: "ls"}
	_, err := runWithFake(t, fake, "--system", "fish shell only", "shell", "list", "files")
	require.NoError(t, err)
	assert.Equal(t, "fish shell only", fake.lastReq.System)
}

func TestShell_EmptyDescriptionErrors(t *testing.T) {
	_, err := runWithFake(t, &scriptedProvider{name: "fake", reply: "ls"}, "shell")
	assert.Error(t, err)
}

func TestSanitizeCommand(t *testing.T) {
	cases := map[string]string{
		"ls -la":                        "ls -la",
		"  ls -la\n":                    "ls -la",
		"```\nls -la\n```":              "ls -la",
		"```zsh\nls -la\n```":           "ls -la",
		"$ ls -la":                      "ls -la",
		"```bash\ndu -sh * | sort\n```": "du -sh * | sort",
	}
	for in, want := range cases {
		assert.Equal(t, want, sanitizeCommand(in), "input: %q", in)
	}
}
