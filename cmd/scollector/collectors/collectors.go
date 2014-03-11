package collectors

import (
	"bufio"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

var collectors []Collector

type Collector interface {
	Run(chan<- *opentsdb.DataPoint)
	Name() string
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

var DefaultFreq = time.Second * 15

var host = "unknown"
var timestamp int64 = time.Now().Unix()

func init() {
	if h, err := os.Hostname(); err == nil {
		host = h
	}
	go func() {
		for t := range time.Tick(time.Second) {
			timestamp = t.Unix()
		}
	}()
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

// Runs specified collectors. Use nil for all.
func Run(cs []Collector) chan *opentsdb.DataPoint {
	dpchan := make(chan *opentsdb.DataPoint)
	if cs == nil {
		cs = collectors
	}
	for _, c := range cs {
		go c.Run(dpchan)
	}
	return dpchan
}

func Add(md *opentsdb.MultiDataPoint, name string, value interface{}, tags opentsdb.TagSet) {
	if tags == nil {
		tags = make(opentsdb.TagSet)
	}
	if _, present := tags["host"]; !present {
		tags["host"] = host
	}
	d := opentsdb.DataPoint{
		Metric:    name,
		Timestamp: timestamp,
		Value:     value,
		Tags:      tags,
	}
	*md = append(*md, &d)
}

func TSAdd(md *opentsdb.MultiDataPoint, name string, value interface{},
	tags opentsdb.TagSet, ts int64) {
	if tags == nil {
		tags = make(opentsdb.TagSet)
	}
	tags["host"] = host
	d := opentsdb.DataPoint{
		Metric:    name,
		Timestamp: timestamp,
		Value:     value,
		Tags:      tags,
	}
	*md = append(*md, &d)
}

func readLine(fname string, line func(string)) {
	f, err := os.Open(fname)
	if err != nil {
		slog.Infof("%v: %v\n", fname, err)
		return
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		slog.Infof("%v: %v\n", fname, err)
	}
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
