package hub

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	pbtests "github.com/pocketbase/pocketbase/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createLocalAgentTestHub(t testing.TB) (*Hub, *pbtests.TestApp, error) {
	testDataDir := t.TempDir()
	testApp, err := pbtests.NewTestApp(testDataDir)
	if err != nil {
		return nil, nil, err
	}
	return NewHub(testApp), testApp, nil
}

func createLocalAgentTestRecord(app core.App, collection string, data map[string]any) (*core.Record, error) {
	col, err := app.FindCachedCollectionByNameOrId(collection)
	if err != nil {
		return nil, err
	}
	record := core.NewRecord(col)
	for key, value := range data {
		record.Set(key, value)
	}
	return record, app.Save(record)
}

func createLocalAgentTestUser(app core.App) (*core.Record, error) {
	return createLocalAgentTestRecord(app, "users", map[string]any{
		"email":    "local-agent@test.com",
		"password": "testtesttest",
	})
}

func TestBuildLocalAgentCommandEnvFiltersAgentOverrides(t *testing.T) {
	t.Setenv("LOCAL_AGENT_TEST_SAFE", "1")
	t.Setenv("TOKEN", "blocked-token")
	t.Setenv("AETHER_AGENT_TOKEN", "blocked-prefixed-token")
	t.Setenv("AETHER_AGENT_DATA_DIR", "/tmp/blocked-data")
	t.Setenv("PATH", os.Getenv("PATH"))

	envMap := map[string]string{}
	for _, item := range buildLocalAgentCommandEnv() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			envMap[key] = value
		}
	}

	assert.Equal(t, "1", envMap["LOCAL_AGENT_TEST_SAFE"])
	assert.NotContains(t, envMap, "TOKEN")
	assert.NotContains(t, envMap, "AETHER_AGENT_TOKEN")
	assert.NotContains(t, envMap, "AETHER_AGENT_DATA_DIR")
}

func TestReadLogTailReturnsRecentWholeLines(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "agent.log")
	content := "line1\nline2\nline3\nline4\n"
	require.NoError(t, os.WriteFile(logPath, []byte(content), 0644))

	logs, truncated, err := readLogTail(logPath, 18)
	require.NoError(t, err)

	assert.True(t, truncated)
	assert.Equal(t, "line3\nline4\n", logs)
}

func TestResolveHubURLPrefersAppURLThenProxyHeaders(t *testing.T) {
	controller := &localAgentController{hub: &Hub{appURL: "https://configured.example.com"}}
	req := httptest.NewRequest("GET", "http://ignored.example.com/api/aether/local-agent/status", nil)
	req.Header.Set("X-Forwarded-Proto", "https")

	assert.Equal(t, "https://configured.example.com", controller.resolveHubURL(req))

	controller.hub.appURL = ""
	req.Host = "hub.example.com:8090"
	assert.Equal(t, "https://hub.example.com:8090", controller.resolveHubURL(req))
}

func TestLocalAgentStatusRequiresSystemRecordToBeConfigured(t *testing.T) {
	hub, testApp, err := createLocalAgentTestHub(t)
	require.NoError(t, err)
	defer testApp.Cleanup()

	tempDir := t.TempDir()
	binaryPath := filepath.Join(tempDir, "aether-agent")
	envPath := filepath.Join(tempDir, "env")
	require.NoError(t, os.WriteFile(binaryPath, []byte("#!/bin/sh\n"), 0755))
	require.NoError(t, os.WriteFile(envPath, []byte("TOKEN=test\n"), 0644))

	t.Setenv("LOCAL_AGENT_BIN", binaryPath)
	t.Setenv("LOCAL_AGENT_STATE_FILE", filepath.Join(tempDir, "local-agent-state.json"))
	t.Setenv("LOCAL_AGENT_LOG_FILE", filepath.Join(tempDir, "aether-agent.log"))
	t.Setenv("LOCAL_AGENT_PID_FILE", filepath.Join(tempDir, "aether-agent.pid"))
	t.Setenv("LOCAL_AGENT_DATA_DIR", filepath.Join(tempDir, "data"))

	controller := &localAgentController{hub: hub}
	resp, err := controller.status(httptest.NewRequest("GET", "http://hub.example.com", nil))
	require.NoError(t, err)

	assert.True(t, resp.Available)
	assert.False(t, resp.Configured)
	assert.False(t, resp.Running)
	assert.Empty(t, resp.SystemID)
}

func TestResolveSystemNameAlwaysUsesReservedLocalName(t *testing.T) {
	controller := &localAgentController{hub: &Hub{}}

	assert.Equal(t, localAgentDefaultName, controller.resolveSystemName(""))
	assert.Equal(t, localAgentDefaultName, controller.resolveSystemName("任意名称"))
}

func TestEnsureSystemRecordRejectsReservedNameConflict(t *testing.T) {
	hub, testApp, err := createLocalAgentTestHub(t)
	require.NoError(t, err)
	defer testApp.Cleanup()

	userRecord, err := createLocalAgentTestUser(testApp)
	require.NoError(t, err)

	_, err = createLocalAgentTestRecord(testApp, "systems", map[string]any{
		"name":   localAgentDefaultName,
		"host":   "10.0.0.8",
		"port":   "45876",
		"status": "pending",
		"users":  []string{userRecord.Id},
	})
	require.NoError(t, err)

	controller := &localAgentController{hub: hub}
	_, err = controller.ensureSystemRecord(userRecord.Id, localAgentDefaultName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已被其他客户端占用")
}
