package collectors

import (
	"bufio"
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/StackExchange/tcollector/opentsdb"
)

var collectors []Collector

type Collector func() opentsdb.MultiDataPoint

var l = log.New(os.Stdout, "", log.LstdFlags)

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
		v := runtime.FuncForPC(reflect.ValueOf(c).Pointer())
		if strings.Contains(v.Name(), s) {
			r = append(r, c)
		}
	}
	return r
}

func Run() chan *opentsdb.DataPoint {
	dpchan := make(chan *opentsdb.DataPoint)
	for _, c := range collectors {
		go runCollector(dpchan, c)
	}
	return dpchan
}

func runCollector(dpchan chan *opentsdb.DataPoint, c Collector) {
	for _ = range time.Tick(time.Second * 3) {
		md := c()
		for _, dp := range md {
			dpchan <- dp
		}
	}
}

func Add(md *opentsdb.MultiDataPoint, name string, value interface{}, tags opentsdb.TagSet) {
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

func readProc(fname string, line func(string)) {
	f, err := os.Open(fname)
	if err != nil {
		l.Printf("%v: %v\n", fname, err)
		return
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		l.Printf("%v: %v\n", fname, err)
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
