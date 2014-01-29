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

var MAX_PERSEC = 50

func (q *Queue) send() {
	for {
		if len(q.queue) > 0 {
			q.Lock()
			i := len(q.queue)
			if i > MAX_PERSEC {
				i = MAX_PERSEC
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

func (q *Queue) sendBatch(batch opentsdb.MultiDataPoint) {
	qlock.Lock()
	defer qlock.Unlock()
	slog.Infoln("sending", len(batch))
	b, err := batch.Json()
	if err != nil {
		slog.Infoln(err)
		// bad JSON encoding, just give up
		return
	}
	resp, err := http.Post(q.host, "application/json", bytes.NewReader(b))
	// Some problem with connecting to the server; retry later.
	if err != nil {
		slog.Infoln(err)
		slog.Infoln("restoring", len(batch))
		for _, dp := range batch {
			q.c <- dp
		}
		time.Sleep(time.Second * 5)
		return
	}
	// TSDB didn't like our data. Don't put it back in the queue since it's bad.
	if resp.StatusCode != http.StatusNoContent {
		slog.Infoln("RESP ERR", resp.Status)
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			slog.Infoln("ERR", err)
		}
		if len(body) > 0 {
			slog.Infoln("ERR BODY", string(body))
		}
		slog.Infoln("REQ BODY", string(b))
	}
}
