package collectors

import (
	"sync"
	"time"

	"github.com/StackExchange/wmi"
)

func queryWmi(query string, dst interface{}) error {
	return queryWmiNamespace(query, dst, "")
}

func queryWmiNamespace(query string, dst interface{}, namespace string) error {
	return wmi.QueryNamespace(query, dst, namespace)
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
