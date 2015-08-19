package collectors

import (
	"bosun.org/cmd/scollector/conf"
	"fmt"
)

func AddProcessConfig(params conf.ProcessParams) error {
	return fmt.Errorf("process watching not implemented on Darwin")
}

func WatchProcesses() {
}
