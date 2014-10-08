// Package collect provides functions for sending data to OpenTSDB.
//
// The "collect" namespace is used (i.e., <metric_root>.collect) to collect
// program and queue metrics.
package collect

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
)

var (
	// Freq is how often metrics are sent to OpenTSDB.
	Freq = time.Second * 15

	// MaxQueueLen is the maximum size of the queue, above which incoming data will
	// be discarded. Defaults to about 150MB.
	MaxQueueLen = 200000

	// BatchSize is the maximum length of data points sent at once to OpenTSDB.
	BatchSize = 50

	// Debug enables debug logging.
	Debug = false

	// Dropped is the number of dropped data points due to a full queue.
	dropped int64

	// Sent is the number of sent data points.
	sent int64

	tchan               chan *opentsdb.DataPoint
	tsdbURL             string
	osHostname          string
	metricRoot          string
	queue               opentsdb.MultiDataPoint
	qlock, mlock, slock sync.Mutex   // Locks for queues, maps, stats.
	counters                         = make(map[string]*addMetric)
	sets                             = make(map[string]*setMetric)
	puts                             = make(map[string]*putMetric)
	client              *http.Client = &http.Client{
		Transport: &timeoutTransport{Transport: new(http.Transport)},
		Timeout:   time.Minute,
	}
)

type timeoutTransport struct {
	*http.Transport
	Timeout time.Time
}

func (t *timeoutTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if time.Now().After(t.Timeout) {
		t.Transport.CloseIdleConnections()
		t.Timeout = time.Now().Add(time.Minute * 5)
	}
	return t.Transport.RoundTrip(r)
}

// InitChan is similar to Init, but uses the given channel instead of creating a
// new one.
func InitChan(tsdbhost *url.URL, metric_root string, ch chan *opentsdb.DataPoint) error {
	if tchan != nil {
		return fmt.Errorf("cannot init twice")
	}
	if err := checkClean(metric_root, "metric root"); err != nil {
		return err
	}
	u, err := tsdbhost.Parse("/api/put")
	if err != nil {
		return err
	}
	if strings.HasPrefix(u.Host, ":") {
		u.Host = "localhost" + u.Host
	}
	tsdbURL = u.String()
	metricRoot = metric_root + "."
	tchan = ch
	go func() {
		for dp := range tchan {
			qlock.Lock()
			for {
				if len(queue) > MaxQueueLen {
					slock.Lock()
					dropped++
					slock.Unlock()
					break
				}
				queue = append(queue, dp)
				select {
				case dp = <-tchan:
					continue
				default:
				}
				break
			}
			qlock.Unlock()
		}
	}()
	go send()

	go collect()
	Set("collect.dropped", nil, func() (i interface{}) {
		slock.Lock()
		i = dropped
		slock.Unlock()
		return
	})
	Set("collect.sent", nil, func() (i interface{}) {
		slock.Lock()
		i = sent
		slock.Unlock()
		return
	})
	Set("collect.alloc", nil, func() interface{} {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return ms.Alloc
	})
	Set("collect.goroutines", nil, func() interface{} {
		return runtime.NumGoroutine()
	})
	return nil
}

// Init sets up the channels and the queue for sending data to OpenTSDB. It also
// sets up the basename for all metrics.
func Init(tsdbhost *url.URL, metric_root string) error {
	return InitChan(tsdbhost, metric_root, make(chan *opentsdb.DataPoint))
}

func setHostName() error {
	h, err := os.Hostname()
	if err != nil {
		return err
	}
	osHostname = strings.ToLower(strings.SplitN(h, ".", 2)[0])
	if err := checkClean(osHostname, "host tag"); err != nil {
		return err
	}
	return nil
}

type setMetric struct {
	metric string
	ts     opentsdb.TagSet
	f      func() interface{}
}

func Set(metric string, ts opentsdb.TagSet, f func() interface{}) error {
	if err := check(metric, &ts); err != nil {
		return err
	}
	tss := metric + ts.String()
	mlock.Lock()
	sets[tss] = &setMetric{metric, ts.Copy(), f}
	mlock.Unlock()
	return nil
}

type addMetric struct {
	metric string
	ts     opentsdb.TagSet
	value  int64
}

// Add takes a metric and increments a counter for that metric. The metric name
// is appended to the basename specified in the Init function.
func Add(metric string, ts opentsdb.TagSet, inc int64) error {
	if err := check(metric, &ts); err != nil {
		return err
	}
	tss := metric + ts.String()
	mlock.Lock()
	if counters[tss] == nil {
		counters[tss] = &addMetric{
			metric: metric,
			ts:     ts.Copy(),
		}
	}
	counters[tss].value += inc
	mlock.Unlock()
	return nil
}

type putMetric struct {
	metric string
	ts     opentsdb.TagSet
	value  interface{}
}

// Put is useful for capturing "events" that have a gauge value. Subsequent
// calls between the sending interval will overwrite previous calls.
func Put(metric string, ts opentsdb.TagSet, v interface{}) error {
	if err := check(metric, &ts); err != nil {
		return err
	}
	tss := metric + ts.String()
	mlock.Lock()
	puts[tss] = &putMetric{metric, ts.Copy(), v}
	mlock.Unlock()
	return nil
}

func check(metric string, ts *opentsdb.TagSet) error {
	if err := checkClean(metric, "metric"); err != nil {
		return err
	}
	for k, v := range *ts {
		if err := checkClean(k, "tagk"); err != nil {
			return err
		}
		if err := checkClean(v, "tagv"); err != nil {
			return err
		}
	}
	if osHostname == "" {
		if err := setHostName(); err != nil {
			return err
		}
	}
	if *ts == nil {
		*ts = make(opentsdb.TagSet)
	}
	if (*ts)["host"] == "" {
		(*ts)["host"] = osHostname
	}
	return nil
}

func checkClean(s, t string) error {
	if sc, err := opentsdb.Clean(s); s != sc || err != nil {
		if err != nil {
			return err
		}
		return fmt.Errorf("%s %s may only contain a to z, A to Z, 0 to 9, -, _, ., / or Unicode letters and may not be empty", t, s)
	}
	return nil
}

func collect() {
	for {
		mlock.Lock()
		now := time.Now().Unix()
		for _, c := range counters {
			dp := &opentsdb.DataPoint{
				Metric:    metricRoot + c.metric,
				Timestamp: now,
				Value:     c.value,
				Tags:      c.ts,
			}
			tchan <- dp
		}
		for _, s := range sets {
			dp := &opentsdb.DataPoint{
				Metric:    metricRoot + s.metric,
				Timestamp: now,
				Value:     s.f(),
				Tags:      s.ts,
			}
			tchan <- dp
		}
		for _, s := range puts {
			dp := &opentsdb.DataPoint{
				Metric:    metricRoot + s.metric,
				Timestamp: now,
				Value:     s.value,
				Tags:      s.ts,
			}
			tchan <- dp
		}
		puts = make(map[string]*putMetric)
		mlock.Unlock()
		time.Sleep(Freq)
	}
}
