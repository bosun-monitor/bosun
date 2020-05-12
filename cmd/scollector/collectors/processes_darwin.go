package collectors

import (
	"fmt"

	"bosun.org/cmd/scollector/conf"
)

func AddProcessConfig(params conf.ProcessParams) error {
	return fmt.Errorf("process watching not implemented on Darwin")
}

func WatchProcesses() {
}
