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
APP_URL="http://127.0.0.1:19090"
DATA_CLEANUP_KEY=0123456789abcdef0123456789abcdef
export AETHER_HUB_AUTO_LOGIN="me@example.com"
`), 0600)
	require.NoError(t, err)

	// Make sure env vars are effectively "unset" for the loader (empty will be overridden).
	t.Setenv("APP_URL", "")
	t.Setenv("DATA_CLEANUP_KEY", "")
	t.Setenv("AETHER_HUB_AUTO_LOGIN", "")

	require.NoError(t, loadEnvFileFromDir(tmpDir))

	assert.Equal(t, "http://127.0.0.1:19090", os.Getenv("APP_URL"))
	assert.Equal(t, "0123456789abcdef0123456789abcdef", os.Getenv("DATA_CLEANUP_KEY"))
	assert.Equal(t, "me@example.com", os.Getenv("AETHER_HUB_AUTO_LOGIN"))
}

func TestLoadEnvFileFromDir_DoesNotOverrideNonEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env")

	err := os.WriteFile(envPath, []byte(`APP_URL="http://new:19090"`), 0600)
	require.NoError(t, err)

	t.Setenv("APP_URL", "http://existing:19090")

	require.NoError(t, loadEnvFileFromDir(tmpDir))
	assert.Equal(t, "http://existing:19090", os.Getenv("APP_URL"))
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
