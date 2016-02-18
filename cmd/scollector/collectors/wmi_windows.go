package collectors

import (
	"fmt"
	"strings"
	"time"

	"github.com/StackExchange/wmi"
)

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

// wmiParseDatetime converts a string from the CIM_DATETIME format into UTC time.
// Example: "20150616101948.494497-360" = 2015-06-16 04:19:48.494497 +0000 UTC.
func wmiParseCIMDatetime(cimdatetime string) (time.Time, error) {
	i := strings.IndexAny(cimdatetime, "+-")
	if i < 0 {
		return time.Time{}, fmt.Errorf("Invalid CIM_DATETIME format, cannot find UTC offset.")
	}
	t, err := time.Parse("20060102150405", cimdatetime[:i])
	if err != nil {
		return time.Time{}, err
	}
	offset, err := time.ParseDuration(fmt.Sprintf("%vm", cimdatetime[i:]))
	if err != nil {
		return time.Time{}, err
	}
	return t.Add(offset), nil
}
