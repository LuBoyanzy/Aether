package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEnvFileFromDir_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, loadEnvFileFromDir(tmpDir))
}

func TestLoadEnvFileFromDir_LoadsAndOverridesEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env")

	err := os.WriteFile(envPath, []byte(`
# comment
KEY="ssh-ed25519 AAAATEST"
HUB_URL=http://127.0.0.1:19090
TOKEN='token123'
export LISTEN=":45876"
AETHER_AGENT_DATA_DIR="/opt/aether/agent/data"
`), 0600)
	require.NoError(t, err)

	// Make sure env vars are effectively "unset" for the loader (empty will be overridden).
	t.Setenv("KEY", "")
	t.Setenv("HUB_URL", "")
	t.Setenv("TOKEN", "")
	t.Setenv("LISTEN", "")
	t.Setenv("AETHER_AGENT_DATA_DIR", "")

	require.NoError(t, loadEnvFileFromDir(tmpDir))

	assert.Equal(t, "ssh-ed25519 AAAATEST", os.Getenv("KEY"))
	assert.Equal(t, "http://127.0.0.1:19090", os.Getenv("HUB_URL"))
	assert.Equal(t, "token123", os.Getenv("TOKEN"))
	assert.Equal(t, ":45876", os.Getenv("LISTEN"))
	assert.Equal(t, "/opt/aether/agent/data", os.Getenv("AETHER_AGENT_DATA_DIR"))
}

func TestLoadEnvFileFromDir_DoesNotOverrideNonEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env")

	err := os.WriteFile(envPath, []byte(`KEY="ssh-ed25519 AAAANEW"`), 0600)
	require.NoError(t, err)

	t.Setenv("KEY", "ssh-ed25519 AAAAEXISTING")

	require.NoError(t, loadEnvFileFromDir(tmpDir))
	assert.Equal(t, "ssh-ed25519 AAAAEXISTING", os.Getenv("KEY"))
}

func TestLoadEnvFileFromDir_ParseError(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env")

	err := os.WriteFile(envPath, []byte("INVALID_LINE\n"), 0600)
	require.NoError(t, err)

	err = loadEnvFileFromDir(tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid line")
	assert.Contains(t, err.Error(), envPath)
}

