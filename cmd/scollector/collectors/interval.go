package collectors

import (
	"reflect"
	"runtime"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
)

type IntervalCollector struct {
	F        func() opentsdb.MultiDataPoint
	Interval time.Duration
	name     string
	init     func()
}

func (c *IntervalCollector) Init() {
	if c.init != nil {
		c.init()
	}
}

func (c *IntervalCollector) Run(dpchan chan<- *opentsdb.DataPoint) {
	for {
		interval := c.Interval
		if interval == 0 {
			interval = DefaultFreq
		}
		next := time.After(interval)
		md := c.F()
		for _, dp := range md {
			dpchan <- dp
		}
		<-next
	}
}

func (c *IntervalCollector) Name() string {
	if c.name != "" {
		return c.name
	}
	v := runtime.FuncForPC(reflect.ValueOf(c.F).Pointer())
	return v.Name()
}
