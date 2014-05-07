// Package collect provides functions for sending data to OpenTSDB.
package collect

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/queue"
)

var (
	// Freq is how often metrics are sent to OpenTSDB. Counters are timestamped at
	// the time they are added to the queue.
	Freq = time.Second * 15

	host     string
	mr       string
	tchan    chan *opentsdb.DataPoint
	counters = make(map[string]*opentsdb.DataPoint)
	lock     = sync.Mutex{}
)

// Init sets up the channels and the queue for sending data to OpenTSDB. It also
// sets up the basename for all metrics.
func Init(tsdbhost, metric_root string) error {
	if err := setHostName(); err != nil {
		return err
	}
	if err := checkClean(metric_root, "metric root"); err != nil {
		return err
	}
	if tsdbhost == "" {
		return errors.New("must specify non-empty tsdb host")
	}
	if tchan != nil {
		return errors.New("Init may only be called once, channel already initalized")
	}
	tchan = make(chan *opentsdb.DataPoint)
	u := url.URL{
		Scheme: "http",
		Path:   "/api/put",
	}
	if !strings.Contains(tsdbhost, ":") {
		tsdbhost += ":4242"
	}
	u.Host = tsdbhost
	queue.New(u.String(), tchan)
	mr = metric_root + "."
	go send()
	return nil
}

func setHostName() error {
	h, err := os.Hostname()
	if err != nil {
		return err
	}
	host = strings.SplitN(strings.ToLower(h), ".", 2)[0]
	if err := checkClean(host, "host tag"); err != nil {
		return err
	}
	return nil
}

// Add takes a metric and increments a counter for that metric. The metric name
// is appended to the basename specified in the Init function.
func Add(metric string, inc int64, ts opentsdb.TagSet) error {
	if tchan == nil || mr == "" {
		return errors.New("Init must be called before calling Add")
	}
	if err := checkClean(metric, "metric"); err != nil {
		return err
	}
	if ts == nil {
		ts = make(opentsdb.TagSet)
	}
	if _, present := ts["host"]; !present {
		ts["host"] = host
	}
	for k, v := range ts {
		if err := checkClean(k, "tagk"); err != nil {
			return err
		}
		if err := checkClean(v, "tagv"); err != nil {
			return err
		}
	}
	tss := metric + ts.String()
	lock.Lock()
	if counters[tss] == nil {
		counters[tss] = &opentsdb.DataPoint{
			Metric: mr + metric,
			Tags:   ts,
			Value:  int64(0),
		}
	}
	v := counters[tss].Value.(int64)
	counters[tss].Value = v + inc
	lock.Unlock()
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

func send() {
	for {
		lock.Lock()
		now := time.Now().Unix()
		for _, dp := range counters {
			dp.Timestamp = now
			tchan <- dp
		}
		lock.Unlock()
		time.Sleep(Freq)
	}
}
