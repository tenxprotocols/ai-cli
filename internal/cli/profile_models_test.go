package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tenxprotocols/ai-cli/internal/config"
)

func TestModels_ListsInjectedProvider(t *testing.T) {
	out, err := runWithFake(t, &scriptedProvider{name: "fake"}, "models")
	require.NoError(t, err)
	assert.Equal(t, "fake/fake-model\tFake\n", out)
}

func TestModels_JSON(t *testing.T) {
	out, err := runWithFake(t, &scriptedProvider{name: "fake"}, "--format", "json", "models")
	require.NoError(t, err)
	assert.Contains(t, out, `"provider":"fake"`)
	assert.Contains(t, out, `"id":"fake-model"`)
}

func TestProfile_ListMarksDefault(t *testing.T) {
	out, err := runWithFake(t, &scriptedProvider{name: "fake"}, "profile", "list")
	require.NoError(t, err)
	assert.Equal(t, "* default\tfake/fake-model\n", out)
}

func TestProfile_CreateUseShowRm(t *testing.T) {
	p := writeTempConfig(t, fakeConfig)
	run := func(args ...string) (string, error) {
		cmd := NewRoot()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append([]string{"--config", p}, args...))
		err := cmd.Execute()
		return out.String(), err
	}

	_, err := run("profile", "create", "work", "--from", "default", "--model", "better-model")
	require.NoError(t, err)

	out, err := run("profile", "show", "work")
	require.NoError(t, err)
	assert.Contains(t, out, "model    better-model")

	_, err = run("profile", "use", "work")
	require.NoError(t, err)
	f, err := config.LoadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "work", f.DefaultProfile)

	_, err = run("profile", "rm", "work")
	assert.Error(t, err, "default profile requires --force")
	_, err = run("profile", "rm", "work", "--force")
	require.NoError(t, err)

	_, err = run("profile", "create", "empty")
	assert.Error(t, err, "create without provider/model must fail")
}
