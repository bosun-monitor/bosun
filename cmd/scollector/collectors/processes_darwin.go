package collectors

import "fmt"

func AddProcessConfig(line string) error {
	return fmt.Errorf("process watching not implemented on Darwin")
}

func WatchProcesses() {
}
