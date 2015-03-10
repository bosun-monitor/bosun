package collectors

import (
	"net/http"
	"reflect"
	"runtime"
	"sync"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

type IntervalCollector struct {
	F        func() (opentsdb.MultiDataPoint, error)
	Interval time.Duration // defaults to DefaultFreq if unspecified
	Enable   func() bool
	name     string
	init     func()

	// internal use
	sync.Mutex
	enabled bool
}

func (c *IntervalCollector) Init() {
	if c.init != nil {
		c.init()
	}
}

func (c *IntervalCollector) Run(dpchan chan<- *opentsdb.DataPoint) {
	if c.Enable != nil {
		go func() {
			for {
				next := time.After(time.Minute * 5)
				c.Lock()
				c.enabled = c.Enable()
				c.Unlock()
				<-next
			}
		}()
	}
	tags := opentsdb.TagSet{"collector": c.Name()}
	for {
		interval := c.Interval
		if interval == 0 {
			interval = DefaultFreq
		}
		next := time.After(interval)
		if c.Enabled() {
			timeStart := time.Now()
			md, err := c.F()
			timeFinish := time.Since(timeStart)
			result := 0
			if err != nil {
				slog.Errorf("%v: %v", c.Name(), err)
				result = 1
			}
			Add(&md, "scollector.collector.duration", timeFinish.Seconds(), tags, metadata.Gauge, metadata.Second, "Duration in seconds for each collector run.")
			Add(&md, "scollector.collector.error", result, tags, metadata.Gauge, metadata.Ok, "Status of collector run. 1=Error, 0=Success.")
			for _, dp := range md {
				dpchan <- dp
			}
		}
		<-next
	}
}

func (c *IntervalCollector) Enabled() bool {
	if c.Enable == nil {
		return true
	}
	c.Lock()
	defer c.Unlock()
	return c.enabled
}

func (c *IntervalCollector) Name() string {
	if c.name != "" {
		return c.name
	}
	v := runtime.FuncForPC(reflect.ValueOf(c.F).Pointer())
	return v.Name()
}

func enableURL(url string) func() bool {
	return func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == 200
	}
}
