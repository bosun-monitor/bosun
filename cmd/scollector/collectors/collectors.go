package collectors

import (
	"bufio"
	"log"
	"os"
	"time"

	"github.com/StackExchange/tcollector/opentsdb"
)

var collectors []Collector

type Collector func() opentsdb.MultiDataPoint

var l = log.New(os.Stdout, "", log.LstdFlags)

var host = "unknown"
var timestamp int64

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
		return
	}
}
