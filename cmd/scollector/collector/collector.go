package collector

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
	host     string
	mr       string
	tchan    chan *opentsdb.DataPoint
	counters = make(map[string]*opentsdb.DataPoint)
	lock     = sync.Mutex{}
)

const freq = time.Second * 15

func Init(tsdbhost, metric_root string) error {
	if err := setHostName(); err != nil {
		return err
	}
	if tsdbhost == "" {
		return errors.New("Must specify non-empty tsdb host")
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
	host = strings.ToLower(h)
	return nil
}

func IncCounter(metric string, inc int, ts opentsdb.TagSet) {
	if ts == nil {
		ts = make(opentsdb.TagSet)
	}
	ts["host"] = host
	tss := metric + ts.String()
	lock.Lock()
	if _, present := counters[tss]; !present {
		counters[tss] = &opentsdb.DataPoint{
			Metric: mr + metric,
			Tags:   ts,
			Value:  int64(0),
		}
	}
	now := time.Now().Unix()
	counters[tss].Timestamp = now
	v := counters[tss].Value.(int64)
	counters[tss].Value = v + int64(inc)
	lock.Unlock()
	return
}

func send() {
	for {
		var md opentsdb.MultiDataPoint
		lock.Lock()
		for _, dp := range counters {
			md = append(md, dp)
		}
		lock.Unlock()
		go func() {
			for _, dp := range md {
				fmt.Println("Putting onto chan", dp, tchan)
				tchan <- dp
			}
		}()
		time.Sleep(freq)
	}
}
