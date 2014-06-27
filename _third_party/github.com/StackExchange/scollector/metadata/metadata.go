package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/util"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/slog"
)

type RateType int

const (
	Unknown RateType = iota
	Gauge
	Counter
	Rate
)

type Unit int

const (
	None  Unit = iota
	Event      // Unitless.
	Bytes
	Pct // Range of 0-100.
	Second
	PerSecond // Events per second.
	BytesPerSecond
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
	}
	metadata[Metakey{metric, ts, name}] = value
}

func Init(host string) {
	metahost = host
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

func init() {
	metafuncs = append(metafuncs, collectMetadataOmreport)
}

func collectMetadataOmreport() {
	util.ReadCommand(func(line string) {
		fields := strings.Split(line, ";")
		if len(fields) != 2 {
			return
		}
		switch fields[0] {
		case "Chassis Service Tag":
			AddMeta("", nil, "svctag", fields[1], true)
		case "Chassis Model":
			AddMeta("", nil, "model", fields[1], true)
		}
	}, "omreport", "chassis", "info", "-fmt", "ssv")
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
