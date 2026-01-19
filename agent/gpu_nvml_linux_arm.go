//go:build linux && arm

package agent

import "fmt"

// NVML relies on purego on Linux, which currently doesn't compile for GOARCH=arm.
// Keep the agent buildable for linux/arm by disabling NVML on this target.
type nvmlCollector struct {
	gm *GPUManager
}

func (c *nvmlCollector) init() error {
	return fmt.Errorf("nvml is not supported on linux/arm")
}

func (c *nvmlCollector) start() {}

