package hub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const (
	localAgentDefaultName   = "本机"
	localAgentDefaultHost   = "127.0.0.1"
	localAgentDefaultPort   = "45876"
	localAgentLogTailBytes  = 64 * 1024
	localAgentStopTimeout   = 8 * time.Second
	localAgentStopPollDelay = 200 * time.Millisecond
)

type localAgentController struct {
	hub *Hub
}

type localAgentPaths struct {
	binaryPath string
	envPath    string
	logPath    string
	pidPath    string
	dataDir    string
	statePath  string
}

type localAgentState struct {
	SystemID string `json:"system_id"`
}

type localAgentRequest struct {
	Name      string `json:"name"`
	AutoStart *bool  `json:"autoStart"`
}

type localAgentStatusResponse struct {
	Enabled      bool   `json:"enabled"`
	Available    bool   `json:"available"`
	Configured   bool   `json:"configured"`
	Running      bool   `json:"running"`
	Pid          int    `json:"pid,omitempty"`
	SystemID     string `json:"systemId,omitempty"`
	SystemName   string `json:"systemName,omitempty"`
	SystemStatus string `json:"systemStatus,omitempty"`
	Host         string `json:"host,omitempty"`
	Port         string `json:"port,omitempty"`
	HubURL       string `json:"hubUrl,omitempty"`
	BinaryPath   string `json:"binaryPath"`
	EnvPath      string `json:"envPath"`
	LogPath      string `json:"logPath"`
	DataDir      string `json:"dataDir"`
	Error        string `json:"error,omitempty"`
}

type localAgentLogsResponse struct {
	Logs      string `json:"logs"`
	LogPath   string `json:"logPath"`
	Truncated bool   `json:"truncated"`
}

func newLocalAgentController(h *Hub) *localAgentController {
	return &localAgentController{hub: h}
}

func (h *Hub) getLocalAgentStatus(e *core.RequestEvent) error {
	resp, err := newLocalAgentController(h).status(e.Request)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, resp)
}

func (h *Hub) setupLocalAgent(e *core.RequestEvent) error {
	if err := requireLocalAgentWriteAccess(e); err != nil {
		return err
	}
	req, err := decodeLocalAgentRequest(e.Request.Body)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	autoStart := true
	if req.AutoStart != nil {
		autoStart = *req.AutoStart
	}
	resp, err := newLocalAgentController(h).setup(e.Request, e.Auth.Id, req.Name, autoStart)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, resp)
}

func (h *Hub) startLocalAgent(e *core.RequestEvent) error {
	if err := requireLocalAgentWriteAccess(e); err != nil {
		return err
	}
	resp, err := newLocalAgentController(h).start(e.Request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, resp)
}

func (h *Hub) stopLocalAgent(e *core.RequestEvent) error {
	if err := requireLocalAgentWriteAccess(e); err != nil {
		return err
	}
	resp, err := newLocalAgentController(h).stop(e.Request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, resp)
}

func (h *Hub) restartLocalAgent(e *core.RequestEvent) error {
	if err := requireLocalAgentWriteAccess(e); err != nil {
		return err
	}
	resp, err := newLocalAgentController(h).restart(e.Request)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, resp)
}

func (h *Hub) getLocalAgentLogs(e *core.RequestEvent) error {
	resp, err := newLocalAgentController(h).logs()
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, resp)
}

func requireLocalAgentWriteAccess(e *core.RequestEvent) error {
	if e.Auth == nil || e.Auth.GetString("role") == "readonly" {
		return e.ForbiddenError("requires write access", nil)
	}
	return nil
}

func decodeLocalAgentRequest(body io.Reader) (localAgentRequest, error) {
	req := localAgentRequest{}
	if body == nil {
		return req, nil
	}
	if err := json.NewDecoder(body).Decode(&req); err != nil && err != io.EOF {
		return req, fmt.Errorf("invalid request body")
	}
	return req, nil
}

func (c *localAgentController) setup(req *http.Request, userID, systemName string, autoStart bool) (*localAgentStatusResponse, error) {
	if userID == "" {
		return nil, fmt.Errorf("missing user")
	}
	paths, err := c.resolvePaths()
	if err != nil {
		return nil, err
	}
	if err := c.ensureAvailable(paths); err != nil {
		return nil, err
	}
	if _, err := c.hub.GetSSHKey(""); err != nil {
		return nil, err
	}

	name := c.resolveSystemName(systemName)
	systemRecord, err := c.ensureSystemRecord(userID, name)
	if err != nil {
		return nil, err
	}
	token, err := c.ensureFingerprintRecord(systemRecord.Id)
	if err != nil {
		return nil, err
	}
	if err := c.writeEnvFile(paths, token, c.resolveHubURL(req), name); err != nil {
		return nil, err
	}
	if autoStart {
		if err := c.stopProcess(paths); err != nil {
			return nil, err
		}
		if err := c.startProcess(paths); err != nil {
			return nil, err
		}
	}
	return c.status(req)
}

func (c *localAgentController) start(req *http.Request) (*localAgentStatusResponse, error) {
	paths, err := c.resolvePaths()
	if err != nil {
		return nil, err
	}
	if err := c.ensureAvailable(paths); err != nil {
		return nil, err
	}
	if err := c.ensureConfigured(paths); err != nil {
		return nil, err
	}
	if err := c.startProcess(paths); err != nil {
		return nil, err
	}
	return c.status(req)
}

func (c *localAgentController) stop(req *http.Request) (*localAgentStatusResponse, error) {
	paths, err := c.resolvePaths()
	if err != nil {
		return nil, err
	}
	if _, err := c.requireConfiguredSystemRecord(); err != nil {
		return nil, err
	}
	if err := c.stopProcess(paths); err != nil {
		return nil, err
	}
	return c.status(req)
}

func (c *localAgentController) restart(req *http.Request) (*localAgentStatusResponse, error) {
	paths, err := c.resolvePaths()
	if err != nil {
		return nil, err
	}
	if err := c.ensureAvailable(paths); err != nil {
		return nil, err
	}
	if err := c.ensureConfigured(paths); err != nil {
		return nil, err
	}
	if err := c.stopProcess(paths); err != nil {
		return nil, err
	}
	if err := c.startProcess(paths); err != nil {
		return nil, err
	}
	return c.status(req)
}

func (c *localAgentController) logs() (*localAgentLogsResponse, error) {
	if _, err := c.requireConfiguredSystemRecord(); err != nil {
		return nil, err
	}
	paths, err := c.resolvePaths()
	if err != nil {
		return nil, err
	}
	logs, truncated, err := readLogTail(paths.logPath, localAgentLogTailBytes)
	if err != nil {
		return nil, err
	}
	return &localAgentLogsResponse{
		Logs:      logs,
		LogPath:   paths.logPath,
		Truncated: truncated,
	}, nil
}

func (c *localAgentController) status(req *http.Request) (*localAgentStatusResponse, error) {
	paths, err := c.resolvePaths()
	if err != nil {
		return nil, err
	}
	resp := &localAgentStatusResponse{
		Enabled:    c.localAgentEnabled(),
		Host:       c.localAgentHost(),
		Port:       c.localAgentPort(),
		BinaryPath: paths.binaryPath,
		EnvPath:    paths.envPath,
		LogPath:    paths.logPath,
		DataDir:    paths.dataDir,
		HubURL:     c.resolveHubURL(req),
	}
	if !resp.Enabled {
		resp.Error = "本机 agent 功能已禁用"
		return resp, nil
	}
	if err := c.ensureAvailable(paths); err != nil {
		resp.Error = err.Error()
		return resp, nil
	}
	resp.Available = true
	hasEnvFile := false
	if _, err := os.Stat(paths.envPath); err == nil {
		hasEnvFile = true
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	systemRecord, err := c.getSystemRecord()
	if err != nil {
		return nil, err
	}
	if systemRecord != nil {
		resp.SystemID = systemRecord.Id
		resp.SystemName = systemRecord.GetString("name")
		resp.SystemStatus = systemRecord.GetString("status")
		resp.Host = systemRecord.GetString("host")
		resp.Port = systemRecord.GetString("port")
	}
	resp.Configured = systemRecord != nil && hasEnvFile
	if systemRecord != nil && !hasEnvFile {
		resp.Error = "本机 agent 配置文件缺失，请重新执行一键接入"
	}
	if !resp.Configured {
		return resp, nil
	}

	pid, running, err := c.readRunningPid(paths.pidPath)
	if err != nil {
		return nil, err
	}
	resp.Running = running
	resp.Pid = pid
	return resp, nil
}

func (c *localAgentController) ensureSystemRecord(userID, name string) (*core.Record, error) {
	record, err := c.getSystemRecord()
	if err != nil {
		return nil, err
	}
	if record == nil {
		record, err = c.findLocalSystemByIdentity(name)
		if err != nil {
			return nil, err
		}
	}
	if record == nil {
		conflict, err := c.findSystemByName(name)
		if err != nil {
			return nil, err
		}
		if conflict != nil && (conflict.GetString("host") != c.localAgentHost() || conflict.GetString("port") != c.localAgentPort()) {
			return nil, fmt.Errorf("名称“%s”已被其他客户端占用，请先修改该客户端名称", name)
		}
	}
	if record == nil {
		collection, err := c.hub.FindCollectionByNameOrId("systems")
		if err != nil {
			return nil, err
		}
		record = core.NewRecord(collection)
		record.Set("name", name)
		record.Set("host", c.localAgentHost())
		record.Set("port", c.localAgentPort())
		record.Set("users", []string{userID})
		record.Set("status", "pending")
		if err := c.hub.Save(record); err != nil {
			return nil, err
		}
		if err := c.saveState(localAgentState{SystemID: record.Id}); err != nil {
			return nil, err
		}
		return record, nil
	}

	changed := false
	if record.GetString("name") != name {
		record.Set("name", name)
		changed = true
	}
	if record.GetString("host") != c.localAgentHost() {
		record.Set("host", c.localAgentHost())
		changed = true
	}
	if record.GetString("port") != c.localAgentPort() {
		record.Set("port", c.localAgentPort())
		changed = true
	}
	users := record.GetStringSlice("users")
	if !slices.Contains(users, userID) {
		users = append(users, userID)
		record.Set("users", users)
		changed = true
	}
	if changed {
		if err := c.hub.Save(record); err != nil {
			return nil, err
		}
	}
	if err := c.saveState(localAgentState{SystemID: record.Id}); err != nil {
		return nil, err
	}
	return record, nil
}

func (c *localAgentController) requireConfiguredSystemRecord() (*core.Record, error) {
	record, err := c.getSystemRecord()
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("本机尚未接入，请先执行一键接入")
	}
	return record, nil
}

func (c *localAgentController) ensureConfigured(paths localAgentPaths) error {
	if _, err := c.requireConfiguredSystemRecord(); err != nil {
		return err
	}
	if _, err := os.Stat(paths.envPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("本机 agent 配置文件缺失，请重新执行一键接入")
		}
		return err
	}
	return nil
}

func (c *localAgentController) getSystemRecord() (*core.Record, error) {
	state, err := c.loadState()
	if err != nil {
		return nil, err
	}
	if state.SystemID == "" {
		return nil, nil
	}
	record, err := c.hub.FindRecordById("systems", state.SystemID)
	if err != nil {
		if removeErr := os.Remove(c.resolveStatePath()); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, removeErr
		}
		return nil, nil
	}
	return record, nil
}

func (c *localAgentController) findLocalSystemByIdentity(name string) (*core.Record, error) {
	record, err := c.hub.FindFirstRecordByFilter(
		"systems",
		"name = {:name} && host = {:host} && port = {:port}",
		dbx.Params{
			"name": name,
			"host": c.localAgentHost(),
			"port": c.localAgentPort(),
		},
	)
	if err != nil {
		return nil, nil
	}
	return record, nil
}

func (c *localAgentController) findSystemByName(name string) (*core.Record, error) {
	record, err := c.hub.FindFirstRecordByFilter(
		"systems",
		"name = {:name}",
		dbx.Params{"name": name},
	)
	if err != nil {
		return nil, nil
	}
	return record, nil
}

func (c *localAgentController) ensureFingerprintRecord(systemID string) (string, error) {
	record, err := c.hub.FindFirstRecordByFilter("fingerprints", "system = {:system}", dbx.Params{"system": systemID})
	if err == nil {
		token := strings.TrimSpace(record.GetString("token"))
		if token == "" {
			token = uuid.NewString()
			record.Set("token", token)
			if err := c.hub.Save(record); err != nil {
				return "", err
			}
		}
		return token, nil
	}

	collection, findErr := c.hub.FindCollectionByNameOrId("fingerprints")
	if findErr != nil {
		return "", findErr
	}
	record = core.NewRecord(collection)
	token := uuid.NewString()
	record.Set("system", systemID)
	record.Set("token", token)
	record.Set("fingerprint", "")
	if err := c.hub.Save(record); err != nil {
		return "", err
	}
	return token, nil
}

func (c *localAgentController) writeEnvFile(paths localAgentPaths, token, hubURL, systemName string) error {
	if err := os.MkdirAll(filepath.Dir(paths.envPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(paths.dataDir, 0755); err != nil {
		return err
	}
	var buf bytes.Buffer
	buf.WriteString("# Managed by Aether Hub local agent control.\n")
	buf.WriteString("KEY=" + strconv.Quote(c.hub.pubKey) + "\n")
	buf.WriteString("TOKEN=" + strconv.Quote(token) + "\n")
	buf.WriteString("HUB_URL=" + strconv.Quote(hubURL) + "\n")
	buf.WriteString("LISTEN=" + strconv.Quote(c.localAgentPort()) + "\n")
	buf.WriteString("SYSTEM_NAME=" + strconv.Quote(systemName) + "\n")
	buf.WriteString("DATA_DIR=" + strconv.Quote(paths.dataDir) + "\n")
	return os.WriteFile(paths.envPath, buf.Bytes(), 0644)
}

func (c *localAgentController) startProcess(paths localAgentPaths) error {
	pid, running, err := c.readRunningPid(paths.pidPath)
	if err != nil {
		return err
	}
	if running && pid > 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(paths.logPath), 0755); err != nil {
		return err
	}
	logFile, err := os.OpenFile(paths.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command(paths.binaryPath)
	cmd.Dir = filepath.Dir(paths.binaryPath)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = buildLocalAgentCommandEnv()
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	if err := os.WriteFile(paths.pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		_ = cmd.Process.Kill()
		_ = logFile.Close()
		return err
	}
	go func(pid int) {
		_ = cmd.Wait()
		_ = logFile.Close()
		_ = removePidFileIfMatches(paths.pidPath, pid)
	}(cmd.Process.Pid)
	return nil
}

func (c *localAgentController) stopProcess(paths localAgentPaths) error {
	pid, running, err := c.readRunningPid(paths.pidPath)
	if err != nil {
		return err
	}
	if !running || pid <= 0 {
		if err := os.Remove(paths.pidPath); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return removePidFileIfMatches(paths.pidPath, pid)
	}
	_ = process.Signal(os.Interrupt)
	deadline := time.Now().Add(localAgentStopTimeout)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return removePidFileIfMatches(paths.pidPath, pid)
		}
		time.Sleep(localAgentStopPollDelay)
	}
	if err := process.Kill(); err != nil && !strings.Contains(err.Error(), "process already finished") {
		return err
	}
	for i := 0; i < int(localAgentStopTimeout/localAgentStopPollDelay); i++ {
		if !processExists(pid) {
			return removePidFileIfMatches(paths.pidPath, pid)
		}
		time.Sleep(localAgentStopPollDelay)
	}
	return fmt.Errorf("本机 agent 停止超时")
}

func (c *localAgentController) readRunningPid(pidPath string) (int, bool, error) {
	pidBytes, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		if removeErr := os.Remove(pidPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return 0, false, removeErr
		}
		return 0, false, nil
	}
	if pid <= 0 || !processExists(pid) {
		if removeErr := os.Remove(pidPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return 0, false, removeErr
		}
		return 0, false, nil
	}
	return pid, true, nil
}

func (c *localAgentController) resolvePaths() (localAgentPaths, error) {
	hubExe, err := os.Executable()
	if err != nil {
		return localAgentPaths{}, err
	}
	hubDir := filepath.Dir(hubExe)
	binaryPath := c.envOrDefault("LOCAL_AGENT_BIN", filepath.Join(hubDir, "aether-agent"))
	binaryPath, err = filepath.Abs(binaryPath)
	if err != nil {
		return localAgentPaths{}, err
	}
	return localAgentPaths{
		binaryPath: binaryPath,
		envPath:    filepath.Join(filepath.Dir(binaryPath), "env"),
		logPath:    c.envOrDefault("LOCAL_AGENT_LOG_FILE", filepath.Join(filepath.Dir(binaryPath), "aether-agent.log")),
		pidPath:    c.envOrDefault("LOCAL_AGENT_PID_FILE", filepath.Join(filepath.Dir(binaryPath), "aether-agent.pid")),
		dataDir:    c.envOrDefault("LOCAL_AGENT_DATA_DIR", filepath.Join(c.hub.DataDir(), "local-agent-data")),
		statePath:  c.resolveStatePath(),
	}, nil
}

func (c *localAgentController) resolveStatePath() string {
	return c.envOrDefault("LOCAL_AGENT_STATE_FILE", filepath.Join(c.hub.DataDir(), "local-agent-state.json"))
}

func (c *localAgentController) loadState() (localAgentState, error) {
	statePath := c.resolveStatePath()
	state := localAgentState{}
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, err
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	return state, nil
}

func (c *localAgentController) saveState(state localAgentState) error {
	statePath := c.resolveStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(statePath, data, 0644)
}

func (c *localAgentController) localAgentEnabled() bool {
	value, exists := GetEnv("LOCAL_AGENT_ENABLED")
	if !exists {
		return true
	}
	value = strings.TrimSpace(strings.ToLower(value))
	return value != "0" && value != "false" && value != "off"
}

func (c *localAgentController) ensureAvailable(paths localAgentPaths) error {
	if !c.localAgentEnabled() {
		return fmt.Errorf("本机 agent 功能已禁用")
	}
	info, err := os.Stat(paths.binaryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("未找到本机 agent 二进制: %s", paths.binaryPath)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("本机 agent 路径不是可执行文件: %s", paths.binaryPath)
	}
	return nil
}

func (c *localAgentController) resolveHubURL(req *http.Request) string {
	if strings.TrimSpace(c.hub.appURL) != "" {
		return strings.TrimSpace(c.hub.appURL)
	}
	if req == nil {
		return ""
	}
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	if proto := strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = strings.Split(proto, ",")[0]
	}
	return fmt.Sprintf("%s://%s", scheme, req.Host)
}

func (c *localAgentController) resolveSystemName(input string) string {
	return localAgentDefaultName
}

func (c *localAgentController) localAgentHost() string {
	return c.envOrDefault("LOCAL_AGENT_HOST", localAgentDefaultHost)
}

func (c *localAgentController) localAgentPort() string {
	return c.envOrDefault("LOCAL_AGENT_PORT", localAgentDefaultPort)
}

func (c *localAgentController) envOrDefault(key, fallback string) string {
	if value, exists := GetEnv(key); exists && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func removePidFileIfMatches(pidPath string, pid int) error {
	data, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	currentPID, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || currentPID == pid {
		if removeErr := os.Remove(pidPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
	}
	return nil
}

func buildLocalAgentCommandEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	blocked := map[string]struct{}{
		"KEY":         {},
		"KEY_FILE":    {},
		"TOKEN":       {},
		"TOKEN_FILE":  {},
		"HUB_URL":     {},
		"LISTEN":      {},
		"PORT":        {},
		"SYSTEM_NAME": {},
		"DATA_DIR":    {},
	}
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if _, exists := blocked[key]; exists {
			continue
		}
		if strings.HasPrefix(key, "AETHER_AGENT_") {
			continue
		}
		env = append(env, item)
	}
	return env
}

func readLogTail(path string, maxBytes int64) (string, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", false, err
	}
	size := stat.Size()
	if size == 0 {
		return "", false, nil
	}

	start := int64(0)
	truncated := false
	if size > maxBytes {
		start = size - maxBytes
		truncated = true
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return "", false, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return "", false, err
	}
	if truncated {
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 && idx+1 < len(data) {
			data = data[idx+1:]
		}
	}
	return string(data), truncated, nil
}
