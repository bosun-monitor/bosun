package queue

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/StackExchange/scollector/collectors"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
	"github.com/mreiferson/go-httpclient"
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
		for dp := range c {
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
			go q.sendBatch(sending)
			q.Unlock()
		} else {
			time.Sleep(time.Second)
		}
	}
}

var qlock sync.Mutex
var client = &http.Client{
	Transport: &httpclient.Transport{
		RequestTimeout: 10 * time.Second,
	},
}

func (q *Queue) sendBatch(batch opentsdb.MultiDataPoint) {
	qlock.Lock()
	defer qlock.Unlock()
	slog.Infoln("sending", len(batch))
	b, err := batch.Json()
	if err != nil {
		slog.Error(err)
		// bad JSON encoding, just give up
		return
	}
	resp, err := client.Post(q.host, "application/json", bytes.NewReader(b))
	if resp != nil && resp.Body != nil {
		defer func() { resp.Body.Close() }()
	}
	// Some problem with connecting to the server; retry later.
	if err != nil {
		slog.Error(err)
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
	}
	// TSDB didn't like our data. Don't put it back in the queue since it's bad.
	if resp.StatusCode != http.StatusNoContent {
		slog.Errorln(resp.Status)
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			slog.Error(err)
		}
		if len(body) > 0 {
			slog.Error(string(body))
		}
		slog.Errorln("bad data:", string(body))
	} else {
		collectors.IncScollector("sent", len(batch))
	}
}
