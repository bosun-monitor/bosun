package collectors

import (
	"reflect"
	"runtime"

	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

/* StreamCollector is useful for collectors that do not produces metrics at a
   preset interval. Instead it consummes directly from a channel provided by
   the collector and forwards it internally. */

type StreamCollector struct {
	F    func() <-chan *opentsdb.MultiDataPoint
	name string
	init func()

	TagOverride
}

func (s *StreamCollector) Init() {
	if s.init != nil {
		s.init()
	}
}

func (s *StreamCollector) Run(dpchan chan<- *opentsdb.DataPoint, quit <-chan struct{}) {
	inputChan := s.F()
	count := 0
	for {
		select {
		case md := <-inputChan:
			if !collect.DisableDefaultCollectors {
				tags := opentsdb.TagSet{"collector": s.Name(), "os": runtime.GOOS}
				Add(md, "scollector.collector.count", count, tags, metadata.Counter, metadata.Count, "Counter of metrics passed through.")
			}

			for _, dp := range *md {
				if _, found := dp.Tags["host"]; !found {
					dp.Tags["host"] = util.Hostname
				}
				s.ApplyTagOverrides(dp.Tags)
				dpchan <- dp
				count++
			}
		case <-quit:
			return
		}
	}
}

func (s *StreamCollector) Enabled() bool {
	return true
}

func (s *StreamCollector) Name() string {
	if s.name != "" {
		return s.name
	}
	v := runtime.FuncForPC(reflect.ValueOf(s.F).Pointer())
	return v.Name()

}
