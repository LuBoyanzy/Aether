//go:build testing
// +build testing

package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAptSourceLine(t *testing.T) {
	source, ok, err := parseAptSourceLine(
		"deb [arch=amd64 signed-by=/usr/share/keyrings/example.gpg] http://repo.example.com/ubuntu jammy main restricted",
		"sources.list",
		12,
	)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "apt", source.Manager)
	require.Equal(t, "sources.list:12", source.RepoID)
	require.Equal(t, "http://repo.example.com/ubuntu", source.URL)
	require.Equal(t, "deb jammy main restricted", source.Name)

	_, ok, err = parseAptSourceLine("# deb http://ignored.example.com jammy main", "sources.list", 3)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestParseAptSourceLineInvalid(t *testing.T) {
	_, ok, err := parseAptSourceLine("deb [arch=amd64 http://repo.example.com jammy main", "sources.list", 5)
	require.Error(t, err)
	require.False(t, ok)
}

func TestParseRpmRepoFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.repo")
	content := []byte(`[baseos]
name=BaseOS
baseurl=https://mirror.example.com/$releasever/BaseOS/$basearch/os/
enabled=1

[extras]
name=Extras
mirrorlist=https://mirror.example.com/mirrorlist
enabled=0
`)
	require.NoError(t, os.WriteFile(path, content, 0o644))

	sources, err := parseRpmRepoFile(path, "dnf")
	require.NoError(t, err)
	require.Len(t, sources, 2)

	require.Equal(t, "dnf", sources[0].Manager)
	require.Equal(t, "baseos#1", sources[0].RepoID)
	require.Equal(t, "BaseOS", sources[0].Name)
	require.Equal(t, "https://mirror.example.com/$releasever/BaseOS/$basearch/os/", sources[0].URL)
	require.True(t, sources[0].Enabled)

	require.Equal(t, "extras#mirrorlist", sources[1].RepoID)
	require.Equal(t, "Extras", sources[1].Name)
	require.Equal(t, "https://mirror.example.com/mirrorlist", sources[1].URL)
	require.False(t, sources[1].Enabled)
}

func TestResolveRepoVariables(t *testing.T) {
	ctx := repoVarContext{
		ReleaseVer: "9",
		BaseArch:   "x86_64",
		Arch:       "amd64",
		RepoID:     "baseos",
	}
	resolved, err := resolveRepoVariables(
		"https://repo.example.com/$releasever/${basearch}/$repoid/$arch",
		ctx,
	)
	require.NoError(t, err)
	require.Equal(t, "https://repo.example.com/9/x86_64/baseos/amd64", resolved)

	_, err = resolveRepoVariables("https://repo.example.com/$unknown", ctx)
	require.Error(t, err)
}
