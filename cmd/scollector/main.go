package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"bosun.org/cmd/scollector/collectors"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
)

// These constants should remain in source control as their zero values.
const (
	// VersionDate should be set at build time as a date: 20140721184001.
	VersionDate uint64 = 0
	// VersionID should be set at build time as the most recent commit hash.
	VersionID string = ""
)

var (
	flagFilter          = flag.String("f", "", "Filters collectors matching this term, multiple terms separated by comma. Works with all other arguments.")
	flagList            = flag.Bool("l", false, "List available collectors.")
	flagPrint           = flag.Bool("p", false, "Print to screen instead of sending to a host")
	flagHost            = flag.String("h", "", `Bosun or OpenTSDB host. Ex: "http://bosun.example.com:8070".`)
	flagColDir          = flag.String("c", "", `External collectors directory.`)
	flagBatchSize       = flag.Int("b", 0, "OpenTSDB batch size. Used for debugging bad data.")
	flagSNMP            = flag.String("s", "", "SNMP host to poll of the format: \"community@host[,community@host...]\".")
	flagICMP            = flag.String("i", "", "ICMP host to ping of the format: \"host[,host...]\".")
	flagVsphere         = flag.String("v", "", `vSphere host to poll of the format: "user:password@host[,user:password@host...]".`)
	flagFake            = flag.Int("fake", 0, "Generates X fake data points on the test.fake metric per second.")
	flagDebug           = flag.Bool("d", false, "Enables debug output.")
	flagFullHost        = flag.Bool("u", false, `Enables full hostnames: doesn't truncate to first ".".`)
	flagTags            = flag.String("t", "", `Tags to add to every datapoint in the format dc=ny,rack=3. If a collector specifies the same tag key, this one will be overwritten. The host tag is not supported.`)
	flagDisableMetadata = flag.Bool("m", false, "Disable sending of metadata.")
	flagVersion         = flag.Bool("version", false, "Prints the version and exits.")
	flagDisableDefault  = flag.Bool("n", false, "Disable sending of scollector self metrics.")
	flagHostname        = flag.String("hostname", "", "If set, use as value of host tag instead of system hostname.")
	flagFreq            = flag.String("freq", "15", "Set the default frequency in seconds for most collectors.")
	flagConf            = flag.String("conf", "", "Location of configuration file. Defaults to scollector.conf in directory of the scollector executable.")
	flagAWS             = flag.String("aws", "", `AWS keys and region, format: "access_key:secret_key@region".`)

	mains []func()
)

func readConf() {
	loc := *flagConf
	if *flagConf == "" {
		p, err := exePath()
		if err != nil {
			slog.Error(err)
			return
		}
		dir := filepath.Dir(p)
		loc = filepath.Join(dir, "scollector.conf")
	}
	b, err := ioutil.ReadFile(loc)
	if err != nil {
		if *flagConf != "" {
			slog.Fatal(err)
		}
		if *flagDebug {
			slog.Error(err)
		}
		return
	}
	for i, line := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		sp := strings.SplitN(line, "=", 2)
		if len(sp) != 2 {
			if *flagDebug {
				slog.Errorf("expected = in %v:%v", loc, i+1)
			}
			continue
		}
		k := strings.TrimSpace(sp[0])
		v := strings.TrimSpace(sp[1])
		f := func(s *string) {
			if *s == "" {
				*s = v
			}
		}
		switch k {
		case "host":
			f(flagHost)
		case "hostname":
			f(flagHostname)
		case "filter":
			f(flagFilter)
		case "coldir":
			f(flagColDir)
		case "snmp":
			f(flagSNMP)
		case "icmp":
			f(flagICMP)
		case "tags":
			f(flagTags)
		case "aws":
			f(flagAWS)
		case "vsphere":
			f(flagVsphere)
		case "freq":
			f(flagFreq)
		case "process":
			if err := collectors.AddProcessConfig(v); err != nil {
				slog.Fatal(err)
			}
		case "process_dotnet":
			if err := collectors.AddProcessDotNetConfig(v); err != nil {
				slog.Fatal(err)
			}
		case "keepalived_community":
			collectors.KeepAliveCommunity = v
		default:
			slog.Fatalf("unknown key in %v:%v", loc, i+1)
		}
	}
}

func main() {
	flag.Parse()
	if *flagPrint || *flagDebug {
		slog.Set(&slog.StdLog{Log: log.New(os.Stdout, "", log.LstdFlags)})
	}
	if *flagVersion {
		fmt.Printf("scollector version %v (%v)\n", VersionDate, VersionID)
		os.Exit(0)
	}
	for _, m := range mains {
		m()
	}
	readConf()
	if *flagTags != "" {
		var err error
		collectors.AddTags, err = opentsdb.ParseTags(*flagTags)
		if err != nil {
			slog.Fatalf("failed to parse additional tags %v: %v", *flagTags, err)
		}
		if collectors.AddTags["host"] != "" {
			slog.Fatalf("host not supported in custom tags, use -hostname instead")
		}
	}
	util.FullHostname = *flagFullHost
	util.Set()
	if *flagHostname != "" {
		util.Hostname = *flagHostname
		if err := collect.SetHostname(*flagHostname); err != nil {
			slog.Fatal(err)
		}
	}
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
	if *flagAWS != "" {
		for _, s := range strings.Split(*flagAWS, ",") {
			sp := strings.SplitN(s, ":", 2)
			if len(sp) != 2 {
				slog.Fatal("invalid AWS string:", *flagAWS)
			}
			accessKey := sp[0]
			idx := strings.LastIndex(sp[1], "@")
			if idx == -1 {
				slog.Fatal("invalid AWS string:", *flagAWS)
			}
			secretKey := sp[1][:idx]
			region := sp[1][idx+1:]
			if len(accessKey) == 0 || len(secretKey) == 0 || len(region) == 0 {
				slog.Fatal("invalid AWS string:", *flagAWS)
			}
			collectors.AWS(accessKey, secretKey, region)
		}
	}
	if *flagVsphere != "" {
		for _, s := range strings.Split(*flagVsphere, ",") {
			sp := strings.SplitN(s, ":", 2)
			if len(sp) != 2 {
				slog.Fatal("invalid vsphere string:", *flagVsphere)
			}
			user := sp[0]
			idx := strings.LastIndex(sp[1], "@")
			if idx == -1 {
				slog.Fatal("invalid vsphere string:", *flagVsphere)
			}
			pwd := sp[1][:idx]
			host := sp[1][idx+1:]
			if len(user) == 0 || len(pwd) == 0 || len(host) == 0 {
				slog.Fatal("invalid vsphere string:", *flagVsphere)
			}
			collectors.Vsphere(user, pwd, host)
		}
	}
	// Add all process collectors. This is platform specific.
	collectors.WatchProcesses()
	collectors.WatchProcessesDotNet()

	if *flagFake > 0 {
		collectors.InitFake(*flagFake)
	}
	collect.Debug = *flagDebug
	util.Debug = *flagDebug
	if *flagDisableDefault {
		collect.DisableDefaultCollectors = true
	}
	c := collectors.Search(*flagFilter)
	if len(c) == 0 {
		slog.Fatalf("Filter %s matches no collectors.", *flagFilter)
	}
	for _, col := range c {
		col.Init()
	}
	u, err := parseHost()
	if *flagList {
		list(c)
		return
	} else if err != nil {
		slog.Fatal("invalid host:", *flagHost)
	}
	freq, err := strconv.ParseInt(*flagFreq, 10, 64)
	if err != nil {
		slog.Fatal(err)
	}
	collectors.DefaultFreq = time.Second * time.Duration(freq)
	collect.Freq = time.Second * time.Duration(freq)
	if *flagPrint {
		collect.Print = true
	}
	if !*flagDisableMetadata {
		if err := metadata.Init(u, *flagDebug); err != nil {
			slog.Fatal(err)
		}
	}
	cdp := collectors.Run(c)
	if u != nil {
		slog.Infoln("OpenTSDB host:", u)
	}
	if err := collect.InitChan(u, "scollector", cdp); err != nil {
		slog.Fatal(err)
	}
	if VersionDate > 0 {
		go func() {
			for {
				if err := collect.Put("version", nil, VersionDate); err != nil {
					slog.Error(err)
				}
				time.Sleep(time.Hour)
			}
		}()
	}
	if *flagBatchSize > 0 {
		collect.BatchSize = *flagBatchSize
	}
	go func() {
		const maxMem = 500 * 1024 * 1024 // 500MB
		var m runtime.MemStats
		for range time.Tick(time.Minute) {
			runtime.ReadMemStats(&m)
			if m.Alloc > maxMem {
				panic("memory max reached")
			}
		}
	}()
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

func list(cs []collectors.Collector) {
	for _, c := range cs {
		fmt.Println(c.Name())
	}
}

func parseHost() (*url.URL, error) {
	if *flagHost == "" {
		*flagHost = "bosun"
	}
	if !strings.Contains(*flagHost, "//") {
		*flagHost = "http://" + *flagHost
	}
	return url.Parse(*flagHost)
}

func printPut(c chan *opentsdb.DataPoint) {
	for dp := range c {
		b, _ := json.Marshal(dp)
		slog.Info(string(b))
	}
}
