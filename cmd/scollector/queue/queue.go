package queue

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

var l = log.New(os.Stdout, "", log.LstdFlags)

type Queue struct {
	sync.Mutex
	host  string
	queue opentsdb.MultiDataPoint
	c     chan *opentsdb.DataPoint
	purge time.Time
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
		q.Purge()
	}
}

func (q *Queue) Purge() {
	if time.Now().Before(q.purge) {
		return
	}
	q.purge = time.Now().Add(time.Minute)
	q.Lock()
	defer q.Unlock()
	t := time.Now().Add(-time.Minute * 30).Unix()
	n := make(opentsdb.MultiDataPoint, 0, len(q.queue))
	for _, d := range q.queue {
		if d.Timestamp < t {
			continue
		}
		n = append(n, d)
	}
	q.queue = n
}

var qlock sync.Mutex

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
	resp, err := http.Post(q.host, "application/json", bytes.NewReader(b))
	// Some problem with connecting to the server; retry later.
	if err != nil {
		slog.Error(err)
		for _, dp := range batch {
			q.c <- dp
		}
		d := time.Second * 5
		slog.Infof("restored %d, sleeping %s", len(batch), d)
		time.Sleep(d)
		return
	}
	// TSDB didn't like our data. Don't put it back in the queue since it's bad.
	if resp.StatusCode != http.StatusNoContent {
		slog.Error(resp.Status)
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			slog.Error(err)
		}
		if len(body) > 0 {
			slog.Error(string(body))
		}
		slog.Errorln("bad data:", string(b))
	}
}
