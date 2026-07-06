package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveArgs(t *testing.T) {
	knownCmds := map[string]bool{"ask": true, "chat": true, "config": true, "version": true}

	// Case A: invoked as `ai-chat foo` — rewrite to `ai chat foo`.
	got := ResolveArgs("ai-chat", []string{"ai-chat", "foo"}, knownCmds, nil)
	assert.Equal(t, Resolution{Kind: ResolveBuiltin, Args: []string{"chat", "foo"}}, got)

	// Case B: `ai chat foo` — passes through.
	got = ResolveArgs("ai", []string{"ai", "chat", "foo"}, knownCmds, nil)
	assert.Equal(t, Resolution{Kind: ResolveBuiltin, Args: []string{"chat", "foo"}}, got)

	// Case C: `ai foo bar` where `ai-foo` exists on PATH -> plugin exec.
	plugin := func(name string) (string, bool) {
		if name == "foo" {
			return "/usr/local/bin/ai-foo", true
		}
		return "", false
	}
	got = ResolveArgs("ai", []string{"ai", "foo", "bar"}, knownCmds, plugin)
	assert.Equal(t, Resolution{
		Kind: ResolvePlugin, PluginPath: "/usr/local/bin/ai-foo",
		Args: []string{"ai-foo", "bar"},
	}, got)

	// Case D: `ai what is 2+2` -> ask fallback.
	got = ResolveArgs("ai", []string{"ai", "what", "is", "2+2"}, knownCmds, plugin)
	assert.Equal(t, Resolution{
		Kind: ResolveAskFallback,
		Args: []string{"ask", "what", "is", "2+2"},
	}, got)

	// Case E: flag before positional -> still dispatches correctly.
	got = ResolveArgs("ai", []string{"ai", "--model", "x", "what", "is", "X"}, knownCmds, plugin)
	assert.Equal(t, Resolution{
		Kind: ResolveAskFallback,
		Args: []string{"--model", "x", "ask", "what", "is", "X"},
	}, got)

	// Case F: `ai` alone -> no rewrite, let cobra show help.
	got = ResolveArgs("ai", []string{"ai"}, knownCmds, plugin)
	assert.Equal(t, Resolution{Kind: ResolveBuiltin, Args: []string{}}, got)

	// Case G: `ai --help` -> no rewrite.
	got = ResolveArgs("ai", []string{"ai", "--help"}, knownCmds, plugin)
	assert.Equal(t, Resolution{Kind: ResolveBuiltin, Args: []string{"--help"}}, got)
}
