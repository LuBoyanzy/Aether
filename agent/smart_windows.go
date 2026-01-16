//go:build windows

package agent

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

//go:embed all:smartmontools
var smartmontoolsFS embed.FS

var (
	smartctlOnce sync.Once
	smartctlPath string
	smartctlErr  error
)

func ensureEmbeddedSmartctl() (string, error) {
	smartctlOnce.Do(func() {
		// If the embedded binary is missing, gracefully fall back to system smartctl.
		// SmartManager.detectSmartctl() will try PATH / default install locations next.
		data, err := smartmontoolsFS.ReadFile("smartmontools/smartctl.exe")
		if err != nil {
			smartctlErr = fmt.Errorf("embedded smartctl.exe not available: %w", err)
			return
		}
		// Basic sanity check: PE executables start with "MZ".
		if len(data) < 2 || data[0] != 'M' || data[1] != 'Z' {
			smartctlErr = fmt.Errorf("embedded smartctl.exe is invalid (expected PE header)")
			return
		}

		destDir := filepath.Join(os.TempDir(), "aether", "smartmontools")
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			smartctlErr = fmt.Errorf("failed to create smartctl directory: %w", err)
			return
		}

		destPath := filepath.Join(destDir, "smartctl.exe")
		if err := os.WriteFile(destPath, data, 0o755); err != nil {
			smartctlErr = fmt.Errorf("failed to write embedded smartctl: %w", err)
			return
		}

		smartctlPath = destPath
	})

	return smartctlPath, smartctlErr
}
