package queue

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/StackExchange/tcollector/opentsdb"
)

var l = log.New(os.Stdout, "", log.LstdFlags)

type Queue struct {
	sync.Mutex
	host  string
	queue opentsdb.MultiDataPoint
}

// Creates and starts a new Queue.
func New(host string, c chan *opentsdb.DataPoint) *Queue {
	q := Queue{
		host: host,
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

const MAX_PERSEC = 50

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

func (q *Queue) sendBatch(batch opentsdb.MultiDataPoint) {
	fmt.Println("sending", len(batch))
	b, err := batch.Json()
	if err != nil {
		l.Println(err)
		// bad JSON encoding, just give up
		return
	}
	resp, err := http.Post(q.host, "application/json", bytes.NewReader(b))
	if err != nil {
		l.Println(1, err)
		goto Err
	}
	if resp.StatusCode != http.StatusNoContent {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			l.Println(err)
		} else {
			l.Println(string(body))
		}
		goto Err
	}
	return
Err:
	l.Println("error, restoring", len(batch))
	q.Lock()
	q.queue = append(q.queue, batch...)
	q.Unlock()
}
