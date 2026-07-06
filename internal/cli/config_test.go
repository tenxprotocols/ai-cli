package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	require.NoError(t, os.WriteFile(p, []byte(contents), 0o600))
	return p
}

func TestConfigShow_RedactsSecrets(t *testing.T) {
	p := writeTempConfig(t, `
[providers.anthropic]
type = "anthropic"
api_key = "sk-ant-SECRET1234"
`)
	cmd := NewRoot()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--config", p, "config", "show"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.NotContains(t, out, "SECRET1234")
	assert.Contains(t, out, "••••")
	assert.Contains(t, out, "1234") // last 4 visible
}

func TestConfigShow_UnredactedWithFlag(t *testing.T) {
	p := writeTempConfig(t, `
[providers.anthropic]
type = "anthropic"
api_key = "sk-ant-SECRET1234"
`)
	cmd := NewRoot()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--config", p, "config", "show", "--show-secrets"})
	require.NoError(t, cmd.Execute())
	assert.Contains(t, buf.String(), "sk-ant-SECRET1234")
}

func TestConfigPath(t *testing.T) {
	p := writeTempConfig(t, ``)
	cmd := NewRoot()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--config", p, "config", "path"})
	require.NoError(t, cmd.Execute())
	assert.Equal(t, strings.TrimSpace(buf.String()), p)
}
