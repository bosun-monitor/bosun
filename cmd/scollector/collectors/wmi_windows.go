package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/slog"
	"github.com/StackExchange/wmi"
)

var (
	wmiLock      sync.Mutex
	wmiReadLock  sync.Mutex
	wmiCount     = 0
	wmiCmd       *exec.Cmd
	wmiIn        io.WriteCloser
	wmiOut       io.ReadCloser
	wmiOutReader *bufio.Reader
)

func queryWmi(query string, dst interface{}) error {
	return queryWmiNamespace(query, dst, "")
}

func queryWmiNamespace(query string, dst interface{}, namespace string) (err error) {
	wmiLock.Lock()
	defer wmiLock.Unlock()
	collect.Add("wmi.queries", opentsdb.TagSet{}, 1)
	if wmiCount == 0 || wmiCmd == nil {
		collect.Add("wmi.exec", opentsdb.TagSet{}, 1)
		wmiCmd = exec.Command(os.Args[0], "-w")
		if wmiIn != nil {
			wmiIn.Close()
		}
		wmiIn, err = wmiCmd.StdinPipe()
		if err != nil {
			return
		}
		if wmiOut != nil {
			wmiOut.Close()
		}
		wmiOut, err = wmiCmd.StdoutPipe()
		if err != nil {
			return
		}
		wmiOutReader = bufio.NewReader(wmiOut)
		if err = wmiCmd.Start(); err != nil {
			wmiCmd = nil
			return
		}
	}
	wmiCount++
	defer func() {
		if wmiCount > 50 {
			if err := wmiCmd.Process.Kill(); err != nil {
				slog.Infoln(err)
			}
			wmiCmd = nil
			wmiCount = 0
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
	fmt.Fprintln(wmiIn, string(b))
	done := make(chan error, 1)
	go func() {
		wmiReadLock.Lock()
		defer wmiReadLock.Unlock()
		b, err = wmiOutReader.ReadBytes('\n')
		if err != nil {
			done <- err
			return
		}
		done <- wmi.LoadJSON(b, dst)
	}()
	select {
	case err = <-done:
		// return
	case <-time.After(time.Second * 20):
		err = fmt.Errorf("wmi query timeout")
	}
	return
}
