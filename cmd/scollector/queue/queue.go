package queue

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/StackExchange/scollector/collectors"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
	"github.com/mreiferson/go-httpclient"
)

var (
	// MaxQueueLen is the maximum size of the queue, above which incoming data will
	// be discarded. 200,000 guarantees the queue will not take more than around
	// 150MB memory.
	MaxQueueLen = 200000

	// MaxMem, if != 0, is the number of bytes of allocated memory at which a panic
	// is issued.
	MaxMem uint64 = 0
)

var l = log.New(os.Stdout, "", log.LstdFlags)

type Queue struct {
	sync.Mutex
	host  string
	queue opentsdb.MultiDataPoint
	c     chan *opentsdb.DataPoint
}

// Creates and starts a new Queue.
func New(host string, c chan *opentsdb.DataPoint) *Queue {
	q := Queue{
		host: host,
		c:    c,
	}
	go func() {
		var m runtime.MemStats
		for _ = range time.Tick(time.Minute) {
			if MaxMem == 0 {
				continue
			}
			runtime.ReadMemStats(&m)
			if m.Alloc > MaxMem {
				panic("memory max reached")
			}
		}
	}()
	go func() {
		for dp := range c {
			if len(q.queue) > MaxQueueLen {
				collectors.IncScollector("dropped", 1)
				continue
			}
			q.Lock()
			q.queue = append(q.queue, dp)
			q.Unlock()
		}
	}()
	go q.send()
	return &q
}

var BatchSize = 50

func (q *Queue) send() {
	for {
		if len(q.queue) > 0 {
			q.Lock()
			i := len(q.queue)
			if i > BatchSize {
				i = BatchSize
			}
			sending := q.queue[:i]
			q.queue = q.queue[i:]
			q.Unlock()
			slog.Infof("sending: %d, remaining: %d", len(sending), len(q.queue))
			q.sendBatch(sending)
		} else {
			time.Sleep(time.Second)
		}
	}
}

var qlock sync.Mutex
var client = &http.Client{
	Transport: &httpclient.Transport{
		RequestTimeout: time.Minute,
	},
}

func (q *Queue) sendBatch(batch opentsdb.MultiDataPoint) {
	b, err := batch.Json()
	if err != nil {
		slog.Error(err)
		// bad JSON encoding, just give up
		return
	}
	var buf bytes.Buffer
	g := gzip.NewWriter(&buf)
	if _, err = g.Write(b); err != nil {
		slog.Error(err)
		return
	}
	if err = g.Close(); err != nil {
		slog.Error(err)
		return
	}
	req, err := http.NewRequest("POST", q.host, &buf)
	if err != nil {
		slog.Error(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
	}
	// Some problem with connecting to the server; retry later.
	if err != nil || resp.StatusCode != http.StatusNoContent {
		if err != nil {
			slog.Error(err)
		} else if resp.StatusCode != http.StatusNoContent {
			slog.Errorln(resp.Status)
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				slog.Error(err)
			}
			if len(body) > 0 {
				slog.Error(string(body))
			}
		}
		t := time.Now().Add(-time.Minute * 30).Unix()
		old := 0
		restored := 0
		for _, dp := range batch {
			if dp.Timestamp < t {
				old++
				continue
			}
			restored++
			q.c <- dp
		}
		if old > 0 {
			slog.Infof("removed %d old records", old)
		}
		d := time.Second * 5
		slog.Infof("restored %d, sleeping %s", restored, d)
		time.Sleep(d)
		return
	} else {
		slog.Infoln("sent", len(batch))
		collectors.IncScollector("sent", len(batch))
	}
}
