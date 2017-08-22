package collectors

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	registerInit(func(c *conf.Conf) {
		if c.LocalListener != "" {
			collectors = append(collectors, &StreamCollector{F: func() <-chan *opentsdb.MultiDataPoint {
				return cLocalListener(c.LocalListener)
			},
				name: fmt.Sprintf("local_listener-%s", c.LocalListener),
			})
		}
	})
}

func cLocalListener(listenAddr string) <-chan *opentsdb.MultiDataPoint {
	pm := &putMetric{}
	pm.localMetrics = make(chan *opentsdb.MultiDataPoint, 1)

	mux := http.NewServeMux()
	mux.Handle("/api/put", pm)
	mux.HandleFunc("/api/metadata/put", putMetadata)
	go http.ListenAndServe(listenAddr, mux)

	return pm.localMetrics
}

type putMetric struct {
	localMetrics chan *opentsdb.MultiDataPoint
}

func (pm *putMetric) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		bodyReader io.ReadCloser
		err        error
	)
	defer r.Body.Close()

	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}

	if r.Header.Get("Content-Encoding") == "gzip" {
		if bodyReader, err = gzip.NewReader(r.Body); err != nil {
			w.WriteHeader(500)
			w.Write([]byte(fmt.Sprintf("Unable to decompress: %s\n", err)))
			return
		}
	} else {
		bodyReader = r.Body
	}

	body, err := ioutil.ReadAll(bodyReader)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	bodyReader.Close()

	var (
		dp  *opentsdb.DataPoint
		mdp opentsdb.MultiDataPoint
	)

	if err := json.Unmarshal(body, &mdp); err == nil {
	} else if err = json.Unmarshal(body, &dp); err == nil {
		mdp = opentsdb.MultiDataPoint{dp}
	} else {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("Unable to decode OpenTSDB json: %s\n", err)))
		return
	}

	for _, dp := range mdp {
		dp.Tags = AddTags.Copy().Merge(dp.Tags)
	}

	pm.localMetrics <- &mdp

	w.WriteHeader(204)
}

func putMetadata(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	var ms []metadata.Metasend
	if err := d.Decode(&ms); err != nil {
		w.WriteHeader(500)
		return
	}
	for _, m := range ms {
		metadata.AddMeta(m.Metric, m.Tags.Copy(), m.Name, m.Value, true)
	}
	w.WriteHeader(204)
}
