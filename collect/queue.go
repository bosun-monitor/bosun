package collect

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

func queuer() {
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
}

// Locks the queue and sends all datapoints. Intended to be used as scollector exits.
func Flush() {
	flushData()
	if !DisableMetadata {
		metadata.FlushMetadata()
	}
	qlock.Lock()
	for len(queue) > 0 {
		i := len(queue)
		if i > BatchSize {
			i = BatchSize
		}
		sending := queue[:i]
		queue = queue[i:]
		if Debug {
			slog.Infof("sending: %d, remaining: %d", i, len(queue))
		}
		sendBatch(sending)
	}
	qlock.Unlock()
}

func send() {
	for {
		qlock.Lock()
		if i := len(queue); i > 0 {
			if i > BatchSize {
				i = BatchSize
			}
			sending := queue[:i]
			queue = queue[i:]
			if Debug {
				slog.Infof("sending: %d, remaining: %d", i, len(queue))
			}
			qlock.Unlock()
			Sample("collect.post.batchsize", Tags, float64(len(sending)))
			sendBatch(sending)
		} else {
			qlock.Unlock()
			time.Sleep(time.Second)
		}
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
