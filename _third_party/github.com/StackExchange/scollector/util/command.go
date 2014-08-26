package util

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/slog"
)

// Command executes the named program with the given arguments. If it does not
// exit within timeout, it is sent SIGINT (if supported by Go). After
// another timeout, it is killed.
func Command(timeout time.Duration, name string, arg ...string) ([]byte, error) {
	c := exec.Command(name, arg...)
	var b bytes.Buffer
	c.Stdout = &b
	done := make(chan error, 1)
	go func() {
		done <- c.Run()
	}()
	interrupt := time.After(timeout)
	kill := time.After(timeout * 2)
	for {
		select {
		case err := <-done:
			return b.Bytes(), err
		case <-interrupt:
			c.Process.Signal(os.Interrupt)
		case <-kill:
			// todo: figure out if this can leave the done chan hanging open
			c.Process.Kill()
			return nil, fmt.Errorf("%v killed after %v", name, timeout*2)
		}
	}
}

// ReadCommand runs command name with args and calls line for each line from its
// stdout. Command is interrupted (if supported by Go) after 10 seconds and
// killed after 20 seconds.
func ReadCommand(line func(string) error, name string, arg ...string) error {
	return ReadCommandTimeout(time.Second*10, line, name, arg...)
}

// ReadCommandTimeout is the same as ReadCommand with a specifiable timeout.
func ReadCommandTimeout(timeout time.Duration, line func(string) error, name string, arg ...string) error {
	b, err := Command(timeout, name, arg...)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(bytes.NewBuffer(b))
	for scanner.Scan() {
		if err := line(scanner.Text()); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		slog.Infof("%v: %v\n", name, err)
	}
	return nil
}
