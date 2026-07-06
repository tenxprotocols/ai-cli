package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShell_UsesCommandModelOverride(t *testing.T) {
	path := writeTempConfig(t, fakeConfig+`
[commands.shell]
model = "fast-model"
`)
	fake := &scriptedProvider{name: "fake", reply: "ls"}
	cmd := NewRoot()
	cmd.SetContext(WithInjectedProvider(context.Background(), "fake", fake))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config", path, "shell", "list", "files"})
	require.NoError(t, cmd.Execute())

	assert.Equal(t, "fast-model", fake.lastReq.Model)
}
