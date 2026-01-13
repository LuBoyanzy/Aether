package agent

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"aether/internal/entities/repo"
	"aether/internal/entities/system"
)

const (
	aptSourcesPath = "/etc/apt/sources.list"
	aptSourcesDir  = "/etc/apt/sources.list.d"
	yumReposDir    = "/etc/yum.repos.d"
)

var (
	errRepoSourcesUnsupported = errors.New("repo sources collection is supported only on linux")
	errRepoSchemeUnsupported  = errors.New("repo source scheme not supported")
	repoVarPattern            = regexp.MustCompile(`\$\{?([A-Za-z0-9_]+)\}?`)
)

type repoSourcesOptions struct {
	Check bool
}

type repoVarContext struct {
	ReleaseVer string
	BaseArch   string
	Arch       string
	RepoID     string
}

func (ctx repoVarContext) withRepoID(repoID string) repoVarContext {
	ctx.RepoID = repoID
	return ctx
}

func (a *Agent) collectRepoSources(options repoSourcesOptions) ([]repo.Source, error) {
	if a.systemDetails.Os != system.Linux {
		return nil, errRepoSourcesUnsupported
	}

	sources := make([]repo.Source, 0)
	var errs []error

	aptSources, err := loadAptSources()
	if err != nil {
		errs = append(errs, err)
	}
	sources = append(sources, aptSources...)

	rpmSources, err := loadRpmSources()
	if err != nil {
		errs = append(errs, err)
	}
	sources = append(sources, rpmSources...)

	for i := range sources {
		if sources[i].Status == "" {
			sources[i].Status = repo.StatusUnknown
		}
	}

	if options.Check {
		checkRepoSources(sources, repoVarContextFromSystem(), time.Now())
	}

	if len(errs) > 0 {
		return sources, errors.Join(errs...)
	}
	return sources, nil
}

func repoVarContextFromSystem() repoVarContext {
	ctx := repoVarContext{Arch: runtime.GOARCH}
	if info, err := readOsRelease(); err == nil {
		ctx.ReleaseVer = info.VersionID
	} else {
		slog.Error("Failed to read os-release for repo source variables", "err", err)
	}
	if baseArch, err := mapBaseArch(runtime.GOARCH); err == nil {
		ctx.BaseArch = baseArch
	} else {
		slog.Error("Failed to map basearch for repo source variables", "goarch", runtime.GOARCH, "err", err)
	}
	return ctx
}

type osReleaseInfo struct {
	ID        string
	VersionID string
}

func readOsRelease() (osReleaseInfo, error) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return osReleaseInfo{}, err
	}
	defer file.Close()

	var info osReleaseInfo
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		switch key {
		case "ID":
			info.ID = value
		case "VERSION_ID":
			info.VersionID = value
		}
	}
	if err := scanner.Err(); err != nil {
		return osReleaseInfo{}, err
	}
	return info, nil
}

func mapBaseArch(goArch string) (string, error) {
	switch goArch {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "aarch64", nil
	case "386":
		return "i386", nil
	case "ppc64le":
		return "ppc64le", nil
	case "s390x":
		return "s390x", nil
	default:
		return "", fmt.Errorf("unsupported basearch mapping for %s", goArch)
	}
}

func detectRpmManager() string {
	if _, err := exec.LookPath("dnf"); err == nil {
		return "dnf"
	}
	if _, err := exec.LookPath("yum"); err == nil {
		return "yum"
	}
	return "yum"
}

func loadAptSources() ([]repo.Source, error) {
	var sources []repo.Source
	var errs []error

	fileSources, err := parseAptSourcesFile(aptSourcesPath)
	if err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	}
	sources = append(sources, fileSources...)

	dirEntries, err := os.ReadDir(aptSourcesDir)
	if err != nil && !os.IsNotExist(err) {
		errs = append(errs, err)
	} else if err == nil {
		for _, entry := range dirEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".list") {
				continue
			}
			path := filepath.Join(aptSourcesDir, entry.Name())
			fileSources, fileErr := parseAptSourcesFile(path)
			if fileErr != nil {
				errs = append(errs, fileErr)
			}
			sources = append(sources, fileSources...)
		}
	}

	if len(errs) > 0 {
		return sources, errors.Join(errs...)
	}
	return sources, nil
}

func parseAptSourcesFile(path string) ([]repo.Source, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var sources []repo.Source
	var errs []error
	scanner := bufio.NewScanner(file)
	lineNo := 0
	fileLabel := filepath.Base(path)
	for scanner.Scan() {
		lineNo++
		source, ok, parseErr := parseAptSourceLine(scanner.Text(), fileLabel, lineNo)
		if parseErr != nil {
			errs = append(errs, parseErr)
			continue
		}
		if ok {
			sources = append(sources, source)
		}
	}
	if err := scanner.Err(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return sources, errors.Join(errs...)
	}
	return sources, nil
}

func parseAptSourceLine(line, fileLabel string, lineNo int) (repo.Source, bool, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return repo.Source{}, false, nil
	}

	entryType := ""
	switch {
	case strings.HasPrefix(trimmed, "deb "):
		entryType = "deb"
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "deb"))
	case strings.HasPrefix(trimmed, "deb-src "):
		entryType = "deb-src"
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "deb-src"))
	default:
		return repo.Source{}, false, nil
	}

	if strings.HasPrefix(trimmed, "[") {
		endIdx := strings.Index(trimmed, "]")
		if endIdx == -1 {
			return repo.Source{}, false, fmt.Errorf("apt source line %s:%d missing closing bracket", fileLabel, lineNo)
		}
		trimmed = strings.TrimSpace(trimmed[endIdx+1:])
	}

	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return repo.Source{}, false, fmt.Errorf("apt source line %s:%d missing required fields", fileLabel, lineNo)
	}

	repoURL := fields[0]
	dist := fields[1]
	components := ""
	if len(fields) > 2 {
		components = strings.Join(fields[2:], " ")
	}

	name := strings.TrimSpace(strings.TrimSpace(entryType + " " + dist + " " + components))
	return repo.Source{
		Manager: "apt",
		RepoID:  fmt.Sprintf("%s:%d", fileLabel, lineNo),
		Name:    name,
		URL:     repoURL,
		Enabled: true,
		Status:  repo.StatusUnknown,
	}, true, nil
}

func loadRpmSources() ([]repo.Source, error) {
	dirEntries, err := os.ReadDir(yumReposDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sources []repo.Source
	var errs []error
	manager := detectRpmManager()
	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".repo") {
			continue
		}
		path := filepath.Join(yumReposDir, entry.Name())
		fileSources, fileErr := parseRpmRepoFile(path, manager)
		if fileErr != nil {
			errs = append(errs, fileErr)
		}
		sources = append(sources, fileSources...)
	}
	if len(errs) > 0 {
		return sources, errors.Join(errs...)
	}
	return sources, nil
}

type rpmRepoSection struct {
	ID         string
	Name       string
	Enabled    bool
	BaseURLs   []string
	MirrorList string
	Metalink   string
}

func parseRpmRepoFile(path, manager string) ([]repo.Source, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var sources []repo.Source
	var errs []error
	var current *rpmRepoSection

	flush := func() {
		if current == nil || current.ID == "" {
			return
		}
		repoID := current.ID
		name := current.Name
		enabled := current.Enabled
		if len(current.BaseURLs) > 0 {
			for idx, repoURL := range current.BaseURLs {
				trimmedURL := strings.TrimSpace(repoURL)
				if trimmedURL == "" {
					continue
				}
				sources = append(sources, repo.Source{
					Manager: manager,
					RepoID:  fmt.Sprintf("%s#%d", repoID, idx+1),
					Name:    name,
					URL:     trimmedURL,
					Enabled: enabled,
					Status:  repo.StatusUnknown,
				})
			}
		}
		if current.MirrorList != "" {
			sources = append(sources, repo.Source{
				Manager: manager,
				RepoID:  repoID + "#mirrorlist",
				Name:    name,
				URL:     strings.TrimSpace(current.MirrorList),
				Enabled: enabled,
				Status:  repo.StatusUnknown,
			})
		}
		if current.Metalink != "" {
			sources = append(sources, repo.Source{
				Manager: manager,
				RepoID:  repoID + "#metalink",
				Name:    name,
				URL:     strings.TrimSpace(current.Metalink),
				Enabled: enabled,
				Status:  repo.StatusUnknown,
			})
		}
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			sectionID := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			current = &rpmRepoSection{
				ID:      sectionID,
				Enabled: true,
			}
			continue
		}
		if current == nil {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			errs = append(errs, fmt.Errorf("invalid repo line in %s: %s", filepath.Base(path), line))
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "name":
			current.Name = value
		case "baseurl":
			current.BaseURLs = append(current.BaseURLs, splitRepoURLs(value)...)
		case "mirrorlist":
			current.MirrorList = value
		case "metalink":
			current.Metalink = value
		case "enabled":
			current.Enabled = value != "0"
		}
	}
	if err := scanner.Err(); err != nil {
		errs = append(errs, err)
	}
	flush()
	if len(errs) > 0 {
		return sources, errors.Join(errs...)
	}
	return sources, nil
}

func splitRepoURLs(value string) []string {
	if value == "" {
		return nil
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ','
	})
	var urls []string
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			urls = append(urls, trimmed)
		}
	}
	return urls
}

func checkRepoSources(sources []repo.Source, ctx repoVarContext, now time.Time) {
	client := &http.Client{Timeout: 6 * time.Second}
	slog.Info("Repo source check start", "count", len(sources))
	for i := range sources {
		source := &sources[i]
		source.CheckedAt = now.UnixMilli()
		resolvedURL, err := resolveRepoVariables(source.URL, ctx.withRepoID(source.RepoID))
		if err != nil {
			source.Status = repo.StatusUnreachable
			source.Error = err.Error()
			slog.Warn("Repo source variable resolution failed", "url", source.URL, "repo", source.RepoID, "err", err)
			continue
		}
		err = checkRepoURL(client, resolvedURL)
		if err != nil {
			if errors.Is(err, errRepoSchemeUnsupported) {
				source.Status = repo.StatusUnsupported
			} else {
				source.Status = repo.StatusUnreachable
			}
			source.Error = err.Error()
			slog.Warn("Repo source check failed", "url", resolvedURL, "repo", source.RepoID, "err", err)
			continue
		}
		source.Status = repo.StatusReachable
		source.Error = ""
	}
	slog.Info("Repo source check done", "count", len(sources))
}

func resolveRepoVariables(raw string, ctx repoVarContext) (string, error) {
	matches := repoVarPattern.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return raw, nil
	}
	var builder strings.Builder
	last := 0
	for _, match := range matches {
		start := match[0]
		end := match[1]
		nameStart := match[2]
		nameEnd := match[3]
		varName := strings.ToLower(raw[nameStart:nameEnd])
		value, ok := repoVariableValue(varName, ctx)
		if !ok || value == "" {
			return "", fmt.Errorf("repo url %q has unsupported variable %q", raw, varName)
		}
		builder.WriteString(raw[last:start])
		builder.WriteString(value)
		last = end
	}
	builder.WriteString(raw[last:])
	return builder.String(), nil
}

func repoVariableValue(name string, ctx repoVarContext) (string, bool) {
	switch name {
	case "releasever":
		return ctx.ReleaseVer, ctx.ReleaseVer != ""
	case "basearch":
		return ctx.BaseArch, ctx.BaseArch != ""
	case "arch":
		return ctx.Arch, ctx.Arch != ""
	case "repoid":
		return ctx.RepoID, ctx.RepoID != ""
	default:
		return "", false
	}
}

func checkRepoURL(client *http.Client, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid repo url %q: %w", raw, err)
	}
	switch parsed.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("%w: %s", errRepoSchemeUnsupported, parsed.Scheme)
	}

	req, err := http.NewRequest(http.MethodHead, raw, nil)
	if err != nil {
		return fmt.Errorf("repo request create failed for %q: %w", raw, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("repo request failed for %q: %w", raw, err)
	}
	if resp.StatusCode == http.StatusMethodNotAllowed {
		resp.Body.Close()
		reqGet, reqErr := http.NewRequest(http.MethodGet, raw, nil)
		if reqErr != nil {
			return fmt.Errorf("repo GET request create failed for %q: %w", raw, reqErr)
		}
		resp, err = client.Do(reqGet)
		if err != nil {
			return fmt.Errorf("repo GET request failed for %q: %w", raw, err)
		}
		defer resp.Body.Close()
	} else {
		defer resp.Body.Close()
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("repo request %q returned status %d", raw, resp.StatusCode)
	}
	return nil
}
