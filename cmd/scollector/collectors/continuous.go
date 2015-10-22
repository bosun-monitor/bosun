package collectors

import (
	"reflect"
	"regexp"
	"runtime"
	"time"

	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

type ContinuousCollector struct {
	F    func(collectorStatsChan chan<- *ContinuousCollectorStats)
	name string
	tags opentsdb.TagSet
	init func()
}

type ContinuousCollectorStats struct {
	duration float64
	result   int
}

var ContinuousCollectorVars struct {
	dpChan                chan<- *opentsdb.DataPoint
	quit                  <-chan struct{}
	reTwoOrMoreUnderscore *regexp.Regexp
	nameWorkerChan        map[string]chan *ContinuousCollectorStats
}

func init() {
	ContinuousCollectorVars.reTwoOrMoreUnderscore = regexp.MustCompile("[_]{2,}")
	ContinuousCollectorVars.nameWorkerChan = make(map[string]chan *ContinuousCollectorStats)
}

func (c *ContinuousCollector) Init() {
	if c.init != nil {
		c.init()
	}
}

func (c *ContinuousCollector) Run(dpChan chan<- *opentsdb.DataPoint, quit <-chan struct{}) {
	ContinuousCollectorVars.dpChan = dpChan
	ContinuousCollectorVars.quit = quit
	collectorStatsChan, found := ContinuousCollectorVars.nameWorkerChan[c.Name()]
	if !found {
		collectorStatsChan = make(chan *ContinuousCollectorStats, 2)
		ContinuousCollectorVars.nameWorkerChan[c.Name()] = collectorStatsChan
	}
	if !collect.DisableDefaultCollectors {
		go ContinuousCollectorStatsWorker(c, collectorStatsChan, dpChan)
	}
	c.F(collectorStatsChan)
	close(collectorStatsChan)
}

func ContinuousCollectorStatsWorker(c *ContinuousCollector, collectorStatsChan <-chan *ContinuousCollectorStats, dpChan chan<- *opentsdb.DataPoint) {
	var md opentsdb.MultiDataPoint
	last := time.Now().Unix()
	var duration float64
	var result int

	tags := opentsdb.TagSet{"collector": c.Name(), "os": runtime.GOOS}
	if c.tags != nil {
		tags = c.tags.Copy().Merge(tags)
	}

	for collectorStats := range collectorStatsChan {
		duration += collectorStats.duration
		if result < 1 {
			result += collectorStats.result
		}
		// send collector stats at most every 10 seconds
		if time.Now().Unix() > last+10 {
			last = time.Now().Unix()
			Add(&md, "scollector.collector.duration", duration, tags, metadata.Gauge, metadata.Second, "Duration in seconds for each collector run.")
			Add(&md, "scollector.collector.error", result, tags, metadata.Gauge, metadata.Ok, "Status of collector run. 1=Error, 0=Success.")
			for _, dp := range md {
				dpChan <- dp
			}
			md = nil
			duration = 0
			result = 0
		}
	}
}

func (c *ContinuousCollector) Name() string {
	if c.name != "" {
		return c.name
	}
	return runtime.FuncForPC(reflect.ValueOf(c.F).Pointer()).Name()
}
