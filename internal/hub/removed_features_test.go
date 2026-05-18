package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApiTestsFeatureIsRemoved(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	forbiddenPaths := []string{
		"internal/hub/api_tests.go",
		"internal/migrations/202601151413_api_tests.go",
		"internal/migrations/202601261418_api_tests_remove_case_alert_enabled.go",
		"internal/site/src/components/routes/api-tests.tsx",
		"internal/site/src/lib/api-tests.ts",
		"tests/api-tests",
	}
	for _, relPath := range forbiddenPaths {
		if _, err := os.Stat(filepath.Join(repoRoot, relPath)); err == nil {
			t.Fatalf("api-tests feature artifact still exists: %s", relPath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("check api-tests feature artifact %s: %v", relPath, err)
		}
	}

	forbiddenTokens := []string{
		"api-tests",
		"api_tests",
		"ApiTests",
		"apiTest",
		"api_test_",
	}
	scanRoots := []string{
		"internal/hub",
		"internal/migrations",
		"internal/site/src",
		"internal/site/dist",
		"tests",
	}
	for _, relRoot := range scanRoots {
		root := filepath.Join(repoRoot, relRoot)
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := entry.Name()
			if entry.IsDir() {
				switch name {
				case "node_modules", "dist", "build", "test-results", "playwright-report":
					return filepath.SkipDir
				}
				return nil
			}
			if name == "removed_features_test.go" {
				return nil
			}
			switch filepath.Ext(name) {
			case ".go", ".ts", ".tsx", ".js", ".json", ".md", ".po":
			default:
				return nil
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			for _, token := range forbiddenTokens {
				if strings.Contains(string(content), token) {
					relPath, _ := filepath.Rel(repoRoot, path)
					t.Fatalf("api-tests feature reference %q still exists in %s", token, relPath)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
