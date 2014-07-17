package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/scollector/collectors"
	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

var (
	flagFilter    = flag.String("f", "", "Filters collectors matching this term. Works with all other arguments.")
	flagTest      = flag.Bool("t", false, "Test - run collectors once, print, and exit.")
	flagList      = flag.Bool("l", false, "List")
	flagPrint     = flag.Bool("p", false, "Print to screen instead of sending to a host")
	flagHost      = flag.String("h", "bosun", `OpenTSDB host. Ex: "tsdb.example.com". Can optionally specify port: "tsdb.example.com:4000", but will default to 4242 otherwise`)
	flagColDir    = flag.String("c", "", `Passthrough collector directory. It should contain numbered directories like the OpenTSDB scollector expects. Any executable file in those directories is run every N seconds, where N is the name of the directory. Use 0 for a program that should be run continuously and simply pass data through to OpenTSDB (the program will be restarted if it exits. Data output format is: "metric timestamp value tag1=val1 tag2=val2 ...". Timestamp is in Unix format (seconds since epoch). Tags are optional. A host tag is automatically added, but overridden if specified.`)
	flagBatchSize = flag.Int("b", 0, "OpenTSDB batch size. Used for debugging bad data.")
	flagSNMP      = flag.String("s", "", "SNMP host to poll of the format: \"community@host[,community@host...]\".")
	flagICMP      = flag.String("i", "", "ICMP host to ping of the format: \"host[,host...]\".")
	flagFake      = flag.Int("fake", 0, "Generates X fake data points on the test.fake metric per second.")
	flagDebug     = flag.Bool("d", false, "Enables debug output.")
	flagJSON      = flag.Bool("j", false, "With -p enabled, prints JSON.")

	mains []func()
)

func readConf() {
	p, err := exePath()
	if err != nil {
		slog.Error(err)
		return
	}
	dir := filepath.Dir(p)
	p = filepath.Join(dir, "scollector.conf")
	b, err := ioutil.ReadFile(p)
	if err != nil {
		if *flagDebug {
			slog.Error(err)
		}
		return
	}
	for i, line := range strings.Split(string(b), "\n") {
		sp := strings.SplitN(line, "=", 2)
		if len(sp) != 2 {
			if *flagDebug {
				slog.Errorf("expected = in %v:%v", p, i+1)
			}
			continue
		}
		k := strings.TrimSpace(sp[0])
		v := strings.TrimSpace(sp[1])
		f := func(s *string) {
			*s = v
		}
		switch k {
		case "host":
			f(flagHost)
		case "filter":
			f(flagFilter)
		case "coldir":
			f(flagColDir)
		case "snmp":
			f(flagSNMP)
		case "icmp":
			f(flagICMP)
		default:
			if *flagDebug {
				slog.Errorf("unknown key in %v:%v", p, i+1)
			}
		}
	}
}

func main() {
	flag.Parse()
	for _, m := range mains {
		m()
	}
	readConf()

	if *flagColDir != "" {
		collectors.InitPrograms(*flagColDir)
	}
	if *flagSNMP != "" {
		for _, s := range strings.Split(*flagSNMP, ",") {
			sp := strings.Split(s, "@")
			if len(sp) != 2 {
				slog.Fatal("invalid snmp string:", *flagSNMP)
			}
			collectors.SNMPIfaces(sp[0], sp[1])
			collectors.SNMPCisco(sp[0], sp[1])
		}
	}
	if *flagICMP != "" {
		for _, s := range strings.Split(*flagICMP, ",") {
			collectors.ICMP(s)
		}
	}
	if *flagFake > 0 {
		collectors.InitFake(*flagFake)
	}
	collect.Debug = *flagDebug
	c := collectors.Search(*flagFilter)
	for _, col := range c {
		col.Init()
	}
	u := parseHost()
	if *flagTest {
		test(c)
		return
	} else if *flagList {
		list(c)
		return
	} else if *flagHost != "" {
		if u == nil {
			slog.Fatal("invalid host:", *flagHost)
		}
	}
	metadata.Init(u.Host, *flagDebug)
	if *flagPrint {
		collectors.DefaultFreq = time.Second * 3
		slog.Infoln("Set default frequency to", collectors.DefaultFreq)
	}
	cdp := collectors.Run(c)
	if u != nil && !*flagPrint {
		slog.Infoln("OpenTSDB host:", u)
		if err := collect.InitChan(u.Host, "scollector", cdp); err != nil {
			slog.Fatal(err)
		}
		if *flagBatchSize > 0 {
			collect.BatchSize = *flagBatchSize
		}
		go func() {
			const maxMem = 500 * 1024 * 1024 // 500MB
			var m runtime.MemStats
			for _ = range time.Tick(time.Minute) {
				runtime.ReadMemStats(&m)
				if m.Alloc > maxMem {
					panic("memory max reached")
				}
			}
		}()
	} else {
		slog.Infoln("Outputting to screen")
		printPut(cdp)
	}
	select {}
}

func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}

func test(cs []collectors.Collector) {
	dpchan := make(chan *opentsdb.DataPoint)
	for _, c := range cs {
		go c.Run(dpchan)
		slog.Infoln("run", c.Name())
	}
	dur := time.Second * 10
	slog.Infoln("running for", dur)
	next := time.After(dur)
Loop:
	for {
		select {
		case dp := <-dpchan:
			slog.Info(dp.Telnet())
		case <-next:
			break Loop
		}
	}
}

func list(cs []collectors.Collector) {
	for _, c := range cs {
		slog.Infoln(c.Name())
	}
}

func parseHost() *url.URL {
	if *flagHost == "" {
		return nil
	}
	u := url.URL{
		Scheme: "http",
		Path:   "/api/put",
	}
	if !strings.Contains(*flagHost, ":") {
		*flagHost += ":4242"
	}
	u.Host = *flagHost
	return &u
}

func printPut(c chan *opentsdb.DataPoint) {
	for dp := range c {
		if *flagJSON {
			b, _ := json.Marshal(dp)
			slog.Info(string(b))
		} else {
			slog.Info(dp.Telnet())
		}
	}
}
