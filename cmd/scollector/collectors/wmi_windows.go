package collectors

import (
	"fmt"
	"sync"
	"time"

	"github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/wmi"
)

func queryWmi(query string, dst interface{}) error {
	return queryWmiNamespace(query, dst, "")
}

func queryWmiNamespace(query string, dst interface{}, namespace string) (err error) {
	collect.Add("wmi.queries", nil, 1)
	done := make(chan error, 1)
	go func() {
		done <- wmi.Query(query, dst, namespace)
	}()
	select {
	case err = <-done:
	case <-time.After(time.Second * 20):
		err = fmt.Errorf("wmi query timeout")
	}
	return
}

func wmiInit(enable *bool, lock *sync.Mutex, dst interface{}, where string, query *string) func() {
	*query = wmi.CreateQuery(dst, where)
	return func() {
		update := func() {
			err := queryWmi(*query, &dst)
			lock.Lock()
			*enable = err == nil
			lock.Unlock()
		}
		update()
		go func() {
			for _ = range time.Tick(time.Minute * 5) {
				update()
			}
		}()
	}
}
