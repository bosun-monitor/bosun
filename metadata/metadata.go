// Package metadata provides metadata information between bosun and OpenTSDB.
package metadata // import "bosun.org/metadata"

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
)

var (
	// AuthToken is an optional string that sets the X-Access-Token HTTP header
	// which is used to authenticate against Bosun
	AuthToken string
)

// RateType is the type of rate for a metric: gauge, counter, or rate.
type RateType string

const (
	// Unknown is a not-yet documented rate type.
	Unknown RateType = ""
	// Gauge rate type.
	Gauge = "gauge"
	// Counter rate type.
	Counter = "counter"
	// Rate rate type.
	Rate = "rate"
)

// Unit is the unit for a metric.
type Unit string

const (
	// None is a not-yet documented unit.
	None            Unit = ""
	A                    = "A"            // Amps
	ActiveUsers          = "active users" // Google Analytics
	Alert                = "alerts"
	Abort                = "aborts"
	Bool                 = "bool"
	BitsPerSecond        = "bits per second"
	Bytes                = "bytes"
	BytesPerSecond       = "bytes per second"
	C                    = "C" // Celsius
	CacheHit             = "cache hits"
	CacheMiss            = "cache misses"
	Change               = "changes"
	Channel              = "channels"
	Check                = "checks"
	CHz                  = "CentiHertz"
	Client               = "clients"
	Command              = "commands"
	Connection           = "connections"
	Consumer             = "consumers"
	Context              = "contexts"
	ContextSwitch        = "context switches"
	Count                = ""
	Document             = "documents"
	Enabled              = "enabled"
	Entropy              = "entropy"
	Error                = "errors"
	Event                = ""
	Eviction             = "evictions"
	Exchange             = "exchanges"
	Fault                = "faults"
	Flush                = "flushes"
	Files                = "files"
	Frame                = "frames"
	Fraction             = "fraction"
	Get                  = "gets"
	GetExists            = "get exists"
	Group                = "groups"
	Incident             = "incidents"
	Interupt             = "interupts"
	InProgress           = "in progress"
	Item                 = "items"
	KBytes               = "kbytes"
	Key                  = "keys"
	Load                 = "load"
	EMail                = "emails"
	MHz                  = "MHz" // MegaHertz
	Megabit              = "Mbit"
	Merge                = "merges"
	Message              = "messages"
	MilliSecond          = "milliseconds"
	Nanosecond           = "nanoseconds"
	Node                 = "nodes"
	Ok                   = "ok" // "OK" or not status, 0 = ok, 1 = not ok
	Operation            = "Operations"
	Packet               = "packets"
	Page                 = "pages"
	Pct                  = "percent" // Range of 0-100.
	PerSecond            = "per second"
	Pool                 = "pools"
	Process              = "processes"
	Priority             = "priority"
	Query                = "queries"
	Queue                = "queues"
	Ratio                = "ratio"
	Redispatch           = "redispatches"
	Refresh              = "refreshes"
	Replica              = "replicas"
	Retry                = "retries"
	Response             = "responses"
	Request              = "requests"
	RPM                  = "RPM" // Rotations per minute.
	Scheduled            = "scheduled"
	Score                = "score"
	Second               = "seconds"
	Sector               = "sectors"
	Segment              = "segments"
	Server               = "servers"
	Session              = "sessions"
	Shard                = "shards"
	Slave                = "slaves"
	Socket               = "sockets"
	Suggest              = "suggests"
	StatusCode           = "status code"
	Resync               = "resynchronizations"
	Syscall              = "system calls"
	Thread               = "threads"
	Timestamp            = "timestamp"
	Transition           = "transitions"
	USD                  = "US dollars"
	V                    = "V" // Volts
	V10                  = "tenth-Volts"
	Vulnerabilities      = "vulnerabilities"
	Watt                 = "Watts"
	Weight               = "weight"
	Yield                = "yields"
)

// Metakey uniquely identifies a metadata entry.
type Metakey struct {
	Metric string
	Tags   string
	Name   string
}

// TagSet returns m's tags.
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

// AddMeta adds a metadata entry to memory, which is queued for later sending.
func AddMeta(metric string, tags opentsdb.TagSet, name string, value interface{}, setHost bool) {
	if tags == nil {
		tags = make(opentsdb.TagSet)
	}
	if _, present := tags["host"]; setHost && !present {
		tags["host"] = util.Hostname
	}
	if err := tags.Clean(); err != nil {
		slog.Error(err)
		return
	}
	ts := tags.Tags()
	metalock.Lock()
	defer metalock.Unlock()
	prev, present := metadata[Metakey{metric, ts, name}]
	if present && !reflect.DeepEqual(prev, value) {
		slog.Infof("metadata changed for %s/%s/%s: %v to %v", metric, ts, name, prev, value)
		go sendMetadata([]Metasend{{
			Metric: metric,
			Tags:   tags,
			Name:   name,
			Value:  value,
		}})
	} else if metadebug {
		slog.Infof("AddMeta for %s/%s/%s: %v", metric, ts, name, value)
	}
	metadata[Metakey{metric, ts, name}] = value
}

// AddMetricMeta is a convenience function to set the main metadata fields for a
// metric. Those fields are rate, unit, and description. If you need to document
// tag keys then use AddMeta.
func AddMetricMeta(metric string, rate RateType, unit Unit, desc string) {
	AddMeta(metric, nil, "rate", rate, false)
	AddMeta(metric, nil, "unit", unit, false)
	AddMeta(metric, nil, "desc", desc, false)
}

// Init initializes the metadata send queue.
func Init(u *url.URL, debug bool) error {
	mh, err := u.Parse("/api/metadata/put")
	if err != nil {
		return err
	}
	if strings.HasPrefix(mh.Host, ":") {
		mh.Host = "localhost" + mh.Host
	}
	metahost = mh.String()
	metadebug = debug
	go collectMetadata()
	return nil
}

var putFunction func(k Metakey, v interface{}) error

func InitF(debug bool, f func(k Metakey, v interface{}) error) error {
	putFunction = f
	metadebug = debug
	go collectMetadata()
	return nil
}

func collectMetadata() {
	// Wait a bit so hopefully our collectors have run once and populated the
	// metadata.
	time.Sleep(time.Minute)
	for {
		FlushMetadata()
		time.Sleep(time.Hour)
	}
}

func FlushMetadata() {
	for _, f := range metafuncs {
		f()
	}
	if len(metadata) == 0 {
		return
	}
	metalock.Lock()
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
	sendMetadata(ms)
}

// Metasend is the struct for sending metadata to bosun.
type Metasend struct {
	Metric string          `json:",omitempty"`
	Tags   opentsdb.TagSet `json:",omitempty"`
	Name   string          `json:",omitempty"`
	Value  interface{}
	Time   *time.Time `json:",omitempty"`
}

func sendMetadata(ms []Metasend) {
	if putFunction != nil {
		for _, m := range ms {
			key := Metakey{
				Metric: m.Metric,
				Name:   m.Name,
				Tags:   m.Tags.Tags(),
			}
			err := putFunction(key, m.Value)
			if err != nil {
				slog.Error(err)
				continue
			}
		}
	} else {
		postMetadata(ms)
	}
}
func postMetadata(ms []Metasend) {
	b, err := json.Marshal(&ms)
	if err != nil {
		slog.Error(err)
		return
	}
	req, err := http.NewRequest(http.MethodPost, metahost, bytes.NewBuffer(b))
	if err != nil {
		slog.Error(err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if AuthToken != "" {
		req.Header.Set("X-Access-Token", AuthToken)
	}
	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		slog.Error(err)
		return
	}
	defer resp.Body.Close()
	// Drain up to 512 bytes and close the body to let the Transport reuse the connection
	io.CopyN(ioutil.Discard, resp.Body, 512)
	if resp.StatusCode != 204 {
		slog.Errorln("bad metadata return:", resp.Status)
		return
	}
}
