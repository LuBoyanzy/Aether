package hub

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"aether/internal/ghupdate"
	"github.com/spf13/cobra"
)

// Update updates aether to the latest version
func Update(cmd *cobra.Command, _ []string) {
	dataDir := os.TempDir()

	// set dataDir to ./aether_data if it exists
	if _, err := os.Stat("./aether_data"); err == nil {
		dataDir = "./aether_data"
	}

	// Check if china-mirrors flag is set
	useMirror, _ := cmd.Flags().GetBool("china-mirrors")

	// Get the executable path before update
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	updated, err := ghupdate.Update(ghupdate.Config{
		ArchiveExecutable: "aether",
		DataDir:           dataDir,
		UseMirror:         useMirror,
	})
	if err != nil {
		log.Fatal(err)
	}
	if !updated {
		return
	}

	// make sure the file is executable
	if err := os.Chmod(exePath, 0755); err != nil {
		fmt.Printf("Warning: failed to set executable permissions: %v\n", err)
	}

	// Try to restart the service if it's running
	restartService()
}

// restartService attempts to restart the aether service
func restartService() {
	// Check if we're running as a service by looking for systemd
	if _, err := exec.LookPath("systemctl"); err == nil {
		// Check if aether service exists and is active
		cmd := exec.Command("systemctl", "is-active", "aether.service")
		if err := cmd.Run(); err == nil {
			ghupdate.ColorPrint(ghupdate.ColorYellow, "Restarting aether service...")
			restartCmd := exec.Command("systemctl", "restart", "aether.service")
			if err := restartCmd.Run(); err != nil {
				ghupdate.ColorPrintf(ghupdate.ColorYellow, "Warning: Failed to restart service: %v\n", err)
				ghupdate.ColorPrint(ghupdate.ColorYellow, "Please restart the service manually: sudo systemctl restart aether")
			} else {
				ghupdate.ColorPrint(ghupdate.ColorGreen, "Service restarted successfully")
			}
			return
		}
	}

	// Check for OpenRC (Alpine Linux)
	if _, err := exec.LookPath("rc-service"); err == nil {
		cmd := exec.Command("rc-service", "aether", "status")
		if err := cmd.Run(); err == nil {
			ghupdate.ColorPrint(ghupdate.ColorYellow, "Restarting aether service...")
			restartCmd := exec.Command("rc-service", "aether", "restart")
			if err := restartCmd.Run(); err != nil {
				ghupdate.ColorPrintf(ghupdate.ColorYellow, "Warning: Failed to restart service: %v\n", err)
				ghupdate.ColorPrint(ghupdate.ColorYellow, "Please restart the service manually: sudo rc-service aether restart")
			} else {
				ghupdate.ColorPrint(ghupdate.ColorGreen, "Service restarted successfully")
			}
			return
		}
	}

	ghupdate.ColorPrint(ghupdate.ColorYellow, "Service restart not attempted. If running as a service, restart manually.")
}
