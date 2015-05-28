package collectors

import "fmt"

func AddProcessConfig(params ProcessParams) error {
	return fmt.Errorf("process watching not implemented on Darwin")
}

type ProcessParams struct{}

func WatchProcesses() {
}
