package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/util"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/slog"
)

type RateType string

const (
	Unknown RateType = ""
	Gauge            = "gauge"
	Counter          = "counter"
	Rate             = "rate"
)

type Unit string

const (
	None           Unit = ""
	Bytes               = "bytes"
	BytesPerSecond      = "bytes per second"
	Event               = ""
	Ok                  = "ok"      // "OK" or not status, 0 = ok, 1 = not ok
	Pct                 = "percent" // Range of 0-100.
	PerSecond           = "per second"
	RPM                 = "RPM" // Rotations per minute.
	Second              = "seconds"
	C                   = "C" // Celsius
)

type Metakey struct {
	Metric string
	Tags   string
	Name   string
}

func (m Metakey) TagSet() opentsdb.TagSet {
	tags, err := opentsdb.ParseTags(m.Tags)
	if err != nil {
		return nil
	}
	return tags
}

var (
	metadata  = make(map[Metakey]interface{})
	metalock  sync.Mutex
	metahost  string
	metafuncs []func()
	metadebug bool
)

func AddMeta(metric string, tags opentsdb.TagSet, name string, value interface{}, setHost bool) {
	if tags == nil {
		tags = make(opentsdb.TagSet)
	}
	if _, present := tags["host"]; setHost && !present {
		tags["host"] = util.Hostname
	}
	ts := tags.Tags()
	metalock.Lock()
	defer metalock.Unlock()
	prev, present := metadata[Metakey{metric, ts, name}]
	if !reflect.DeepEqual(prev, value) && present {
		slog.Infof("metadata changed for %s/%s/%s: %v to %v", metric, ts, name, prev, value)
	} else if metadebug {
		slog.Infof("AddMeta for %s/%s/%s: %v", metric, ts, name, value)
	}
	metadata[Metakey{metric, ts, name}] = value
}

func Init(host string, debug bool) {
	metahost = host
	metadebug = debug
	go collectMetadata()
}

func collectMetadata() {
	// Wait a bit so hopefully our collectors have run once and populated the
	// metadata.
	time.Sleep(time.Second * 5)
	for {
		for _, f := range metafuncs {
			f()
		}
		sendMetadata()
		time.Sleep(time.Hour)
	}
}

type Metasend struct {
	Metric string          `json:",omitempty"`
	Tags   opentsdb.TagSet `json:",omitempty"`
	Name   string          `json:",omitempty"`
	Value  interface{}
}

func sendMetadata() {
	metalock.Lock()
	if len(metadata) == 0 {
		metalock.Unlock()
		return
	}
	ms := make([]Metasend, len(metadata))
	i := 0
	for k, v := range metadata {
		ms[i] = Metasend{
			Metric: k.Metric,
			Tags:   k.TagSet(),
			Name:   k.Name,
			Value:  v,
		}
		i++
	}
	metalock.Unlock()
	b, err := json.MarshalIndent(&ms, "", "  ")
	if err != nil {
		slog.Error(err)
		return
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/api/metadata/put", metahost), "application/json", bytes.NewBuffer(b))
	if err != nil {
		slog.Error(err)
		return
	}
	if resp.StatusCode != 204 {
		slog.Error("bad metadata return:", resp.Status)
		return
	}
}
