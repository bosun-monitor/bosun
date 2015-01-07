package collectors

import "bosun.org/_third_party/github.com/StackExchange/wmi"

func queryWmi(query string, dst interface{}) error {
	return queryWmiNamespace(query, dst, "")
}

func queryWmiNamespace(query string, dst interface{}, namespace string) error {
	return wmi.QueryNamespace(query, dst, namespace)
}

func wmiInit(c *IntervalCollector, dst func() interface{}, where string, query *string) func() {
	return func() {
		*query = wmi.CreateQuery(dst(), where)
		c.Enable = func() bool {
			return queryWmi(*query, dst()) == nil
		}
	}
}

func wmiInitNamespace(c *IntervalCollector, dst func() interface{}, where string, query *string, namespace string) func() {
	return func() {
		*query = wmi.CreateQuery(dst(), where)
		c.Enable = func() bool {
			return queryWmiNamespace(*query, dst(), namespace) == nil
		}
	}
}
