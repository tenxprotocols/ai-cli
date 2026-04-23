package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoot_Help(t *testing.T) {
	cmd := NewRoot()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--help"})
	require.NoError(t, cmd.Execute())

	out := buf.String()
	assert.Contains(t, out, "ai")
	assert.Contains(t, out, "Usage:")
}

func TestRoot_Version(t *testing.T) {
	cmd := NewRoot()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"version"})
	require.NoError(t, cmd.Execute())
	assert.True(t, strings.HasPrefix(buf.String(), "ai "))
}
