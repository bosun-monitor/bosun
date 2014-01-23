package collectors

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/StackExchange/wmi"
)

func queryWmi(query string, dst interface{}) error {
	return queryWmiNamespace(query, dst, "")
}

func queryWmiNamespace(query string, dst interface{}, namespace string) error {
	args := []string{"-w"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, strings.Split(query, " ")...)
	cmd := exec.Command(os.Args[0], args...)
	b, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("%s: %s", err, b)
	}
	err = wmi.LoadJSON(b, dst)
	if err != nil {
		return err
	}
	return nil
}
