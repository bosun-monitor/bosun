package collectors

import (
	"fmt"

	"bosun.org/cmd/scollector/conf"
)

// AddProcessConfig is not implemented on Mac OS
func AddProcessConfig(params conf.ProcessParams) error {
	return fmt.Errorf("process watching not implemented on Darwin")
}

// WatchProcesses does nothing on Mac OS
func WatchProcesses() {
}
