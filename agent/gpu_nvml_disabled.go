//go:build !(windows || (linux && (amd64 || arm64)))

package agent

import "fmt"

// NVML is only enabled on platforms where our dynamic linking path is supported.
// For all other OS/ARCH combinations, keep builds working by stubbing NVML out.
type nvmlCollector struct {
	gm *GPUManager
}

func (c *nvmlCollector) init() error {
	return fmt.Errorf("nvml is not supported on this platform")
}

func (c *nvmlCollector) start() {}

