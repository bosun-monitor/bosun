package collectors

import (
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"reflect"
	"regexp"
	"runtime"
	"sync"
	"time"

	"bosun.org/collect"
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

	TagOverride
}

func (c *IntervalCollector) Init() {
	if c.init != nil {
		c.init()
	}
}

func (c *IntervalCollector) Run(dpchan chan<- *opentsdb.DataPoint, quit <-chan struct{}) {
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
			if !collect.DisableDefaultCollectors {
				tags := opentsdb.TagSet{"collector": c.Name(), "os": runtime.GOOS}
				Add(&md, "scollector.collector.duration", timeFinish.Seconds(), tags, metadata.Gauge, metadata.Second, "Duration in seconds for each collector run.")
				Add(&md, "scollector.collector.error", result, tags, metadata.Gauge, metadata.Ok, "Status of collector run. 1=Error, 0=Success.")
			}
			for _, dp := range md {
				c.ApplyTagOverrides(dp.Tags)
				dpchan <- dp
			}
		}
		select {
		case <-next:
		case <-quit:
			return
		}

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

func enableURL(url string, regexes ...string) func() bool {
	res := make([]*regexp.Regexp, len(regexes))
	for i, r := range regexes {
		res[i] = regexp.MustCompile(r)
	}
	return func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		defer func() {
			// Drain up to 512 bytes and close the body to let the Transport reuse the connection
			io.CopyN(ioutil.Discard, resp.Body, 512)
			resp.Body.Close()
		}()
		if resp.StatusCode != 200 {
			return false
		}
		if len(res) == 0 {
			return true
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return false
		}
		for _, r := range res {
			if !r.Match(b) {
				return false
			}
		}
		return true
	}
}

// enableExecutable returns true if name is an executable file in the
// environment's PATH.
func enableExecutable(name string) func() bool {
	return func() bool {
		_, err := exec.LookPath(name)
		return err == nil
	}
}
