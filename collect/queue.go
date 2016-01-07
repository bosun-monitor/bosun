package collect

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"sync/atomic"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

func queuer() {
	for dp := range tchan {
		qlock.Lock()
		select {
		case q <- dp:
		default:
			atomic.AddInt64(&dropped, 1)
		}
		qlock.Unlock()
	}
}

// Locks the queue and sends all datapoints. Intended to be used as scollector exits.
func Flush() {
	flushData()
	metadata.FlushMetadata()
	qlock.Lock()
	batch := make([]*opentsdb.DataPoint, 0, BatchSize)
	send := func() {
		if len(batch) == 0 {
			return
		}
		if Debug {
			slog.Infof("sending: %d, remaining: %d", len(batch), len(q))
		}
		sendBatch(batch)
	}
	for len(q) > 0 {
		if len(batch) == BatchSize {
			send()
			batch = make([]*opentsdb.DataPoint, 0, BatchSize)
		}
	}
	send()
	// sleep to let send loop complete as well.
	time.Sleep(time.Second * 2)
	qlock.Unlock()
}

func send() {
	for {
		batch := make([]*opentsdb.DataPoint, 0, BatchSize)
		timeout := time.After(time.Second)
		//aggregate points into batch. Send when full or after 1 sec
	Loop:
		for {
			select {
			case dp := <-q:
				batch = append(batch, dp)
				if len(batch) == BatchSize {
					break Loop
				}
			case <-timeout:
				break Loop
			}
		}
		if len(batch) == 0 {
			continue
		}
		if Debug {
			slog.Infof("sending: %d, remaining: %d", len(batch), len(q))
		}
		Sample("collect.post.batchsize", Tags, float64(len(batch)))
		sendBatch(batch)
	}
}

func sendBatch(batch []*opentsdb.DataPoint) {
	if Print {
		for _, d := range batch {
			j, err := d.MarshalJSON()
			if err != nil {
				slog.Error(err)
			}
			slog.Info(string(j))
		}
		recordSent(len(batch))
		return
	}
	now := time.Now()
	resp, err := SendDataPoints(batch, tsdbURL)
	if err == nil {
		defer resp.Body.Close()
	}
	d := time.Since(now).Nanoseconds() / 1e6
	Sample("collect.post.duration", Tags, float64(d))
	Add("collect.post.total_duration", Tags, d)
	Add("collect.post.count", Tags, 1)
	// Some problem with connecting to the server; retry later.
	if err != nil || resp.StatusCode != http.StatusNoContent {
		if err != nil {
			Add("collect.post.error", Tags, 1)
			slog.Error(err)
		} else if resp.StatusCode != http.StatusNoContent {
			Add("collect.post.bad_status", Tags, 1)
			slog.Errorln(resp.Status)
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				slog.Error(err)
			}
			if len(body) > 0 {
				slog.Error(string(body))
			}
		}
		restored := 0
		for _, msg := range batch {
			restored++
			tchan <- msg
		}
		d := time.Second * 5
		Add("collect.post.restore", Tags, int64(restored))
		slog.Infof("restored %d, sleeping %s", restored, d)
		time.Sleep(d)
		return
	}
	recordSent(len(batch))
}

func recordSent(num int) {
	if Debug {
		slog.Infoln("sent", num)
	}
	slock.Lock()
	sent += int64(num)
	slock.Unlock()
}

func SendDataPoints(dps []*opentsdb.DataPoint, tsdb string) (*http.Response, error) {
	var buf bytes.Buffer
	g := gzip.NewWriter(&buf)
	if err := json.NewEncoder(g).Encode(dps); err != nil {
		return nil, err
	}
	if err := g.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", tsdb, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	Add("collect.post.total_bytes", Tags, int64(buf.Len()))
	resp, err := client.Do(req)
	return resp, err
}
