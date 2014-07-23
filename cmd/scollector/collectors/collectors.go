package collectors

import (
	"bufio"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
	"github.com/StackExchange/slog"
)

var collectors []Collector

type Collector interface {
	Run(chan<- *opentsdb.DataPoint)
	Name() string
	Init()
}

const (
	osCPU          = "os.cpu"
	osDiskFree     = "os.disk.fs.space_free"
	osDiskPctFree  = "os.disk.fs.percent_free"
	osDiskTotal    = "os.disk.fs.space_total"
	osDiskUsed     = "os.disk.fs.space_used"
	osMemFree      = "os.mem.free"
	osMemPctFree   = "os.mem.percent_free"
	osMemTotal     = "os.mem.total"
	osMemUsed      = "os.mem.used"
	osNetBroadcast = "os.net.packets_broadcast"
	osNetBytes     = "os.net.bytes"
	osNetDropped   = "os.net.dropped"
	osNetErrors    = "os.net.errs"
	osNetPackets   = "os.net.packets"
	osNetUnicast   = "os.net.packets_unicast"
	osNetMulticast = "os.net.packets_multicast"
)

var (
	// DefaultFreq is the duration between collection intervals if none is
	// specified.
	DefaultFreq = time.Second * 15

	timestamp = time.Now().Unix()
	tlock     sync.Mutex
)

func init() {
	go func() {
		for t := range time.Tick(time.Second) {
			tlock.Lock()
			timestamp = t.Unix()
			tlock.Unlock()
		}
	}()
}

func now() (t int64) {
	tlock.Lock()
	t = timestamp
	tlock.Unlock()
	return
}

// Search returns all collectors matching the pattern s.
func Search(s string) []Collector {
	var r []Collector
	for _, c := range collectors {
		if strings.Contains(c.Name(), s) {
			r = append(r, c)
		}
	}
	return r
}

// Runs specified collectors. Use nil for all collectors.
func Run(cs []Collector) chan *opentsdb.DataPoint {
	if cs == nil {
		cs = collectors
	}
	ch := make(chan *opentsdb.DataPoint)
	for _, c := range cs {
		go c.Run(ch)
	}
	return ch
}

// AddTS is the same as Add but lets you specify the timestamp
func AddTS(md *opentsdb.MultiDataPoint, name string, ts int64, value interface{}, t opentsdb.TagSet, rate metadata.RateType, unit metadata.Unit, desc string) {
	tags := make(opentsdb.TagSet)
	for k, v := range t {
		tags[k] = v
	}
	if host, present := tags["host"]; !present {
		tags["host"] = util.Hostname
	} else if host == "" {
		delete(tags, "host")
	}
	d := opentsdb.DataPoint{
		Metric:    name,
		Timestamp: ts,
		Value:     value,
		Tags:      tags,
	}
	*md = append(*md, &d)
	if rate != metadata.Unknown {
		metadata.AddMeta(name, nil, "rate", rate, false)
	}
	if unit != metadata.None {
		metadata.AddMeta(name, nil, "unit", unit, false)
	}
	if desc != "" {
		metadata.AddMeta(name, tags, "desc", desc, false)
	}
}

// Add appends a new data point with given metric name, value, and tags. Tags
// may be nil. If tags is nil or does not contain a host key, it will be
// automatically added. If the value of the host key is the empty string, it
// will be removed (use this to prevent the normal auto-adding of the host tag).
func Add(md *opentsdb.MultiDataPoint, name string, value interface{}, t opentsdb.TagSet, rate metadata.RateType, unit metadata.Unit, desc string) {
	AddTS(md, name, now(), value, t, rate, unit, desc)
}

func readLine(fname string, line func(string)) error {
	f, err := os.Open(fname)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		slog.Infof("%v: %v\n", fname, err)
	}
	return nil
}

// IsDigit returns true if s consists of decimal digits.
func IsDigit(s string) bool {
	r := strings.NewReader(s)
	for {
		ch, _, err := r.ReadRune()
		if ch == 0 || err != nil {
			break
		} else if ch == utf8.RuneError {
			return false
		} else if !unicode.IsDigit(ch) {
			return false
		}
	}
	return true
}

// IsAlNum returns true if s is alphanumeric.
func IsAlNum(s string) bool {
	r := strings.NewReader(s)
	for {
		ch, _, err := r.ReadRune()
		if ch == 0 || err != nil {
			break
		} else if ch == utf8.RuneError {
			return false
		} else if !unicode.IsDigit(ch) && !unicode.IsLetter(ch) {
			return false
		}
	}
	return true
}
