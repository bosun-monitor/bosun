package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/StackExchange/slog"
	"github.com/StackExchange/wmi"
)

var (
	wmiLock      sync.Mutex
	wmiCount     = 0
	wmiCmd       *exec.Cmd
	wmiIn        io.WriteCloser
	wmiOut       io.ReadCloser
	wmiOutReader *bufio.Reader
)

func queryWmi(query string, dst interface{}) error {
	return queryWmiNamespace(query, dst, "")
}

func queryWmiNamespace(query string, dst interface{}, namespace string) error {
	wmiLock.Lock()
	defer wmiLock.Unlock()
	if wmiCount == 0 || (wmiCmd != nil && wmiCmd.ProcessState != nil && wmiCmd.ProcessState.Exited()) {
		var err error
		wmiCmd = exec.Command(os.Args[0], "-w")
		wmiIn, err = wmiCmd.StdinPipe()
		if err != nil {
			return err
		}
		wmiOut, err = wmiCmd.StdoutPipe()
		if err != nil {
			return err
		}
		wmiOutReader = bufio.NewReader(wmiOut)
		if err := wmiCmd.Start(); err != nil {
			slog.Infoln(err)
		}
	}
	wmiCount++
	defer func() {
		if wmiCount > 50 {
			wmiCount = 0
			if err := wmiCmd.Process.Kill(); err != nil {
				slog.Infoln(err)
			}
		}
	}()

	q := wmi.WmiQuery{
		Query:     query,
		Namespace: namespace,
	}
	b, err := json.Marshal(&q)
	if err != nil {
		return err
	}
	//fmt.Println("QUERYING:", string(b))
	fmt.Fprintln(wmiIn, string(b))
	b2, err := wmiOutReader.ReadBytes('\n')
	if err != nil {
		return err
	}
	//fmt.Println("GOT BACK:", string(b2))
	err = wmi.LoadJSON(b2, dst)
	if err != nil {
		return err
	}
	return nil
}
