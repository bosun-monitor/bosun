package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"bosun.org/_third_party/github.com/BurntSushi/toml"
	"bosun.org/cmd/scollector/collectors"
	"bosun.org/cmd/scollector/conf"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
	"bosun.org/version"
)

var (
	flagHost            = flag.String("h", "", "OpenTSDB or Bosun host to send data. Overrides Host in conf file.")
	flagFilter          = flag.String("f", "", "Filters collectors matching these terms, separated by comma. Overrides Filter in conf file.")
	flagList            = flag.Bool("l", false, "List available collectors.")
	flagPrint           = flag.Bool("p", false, "Print to screen instead of sending to a host")
	flagBatchSize       = flag.Int("b", 0, "OpenTSDB batch size. Default is 500.")
	flagFake            = flag.Int("fake", 0, "Generates X fake data points on the test.fake metric per second.")
	flagDebug           = flag.Bool("d", false, "Enables debug output.")
	flagDisableMetadata = flag.Bool("m", false, "Disable sending of metadata.")
	flagVersion         = flag.Bool("version", false, "Prints the version and exits.")
	flagConf            = flag.String("conf", "", "Location of configuration file. Defaults to scollector.toml in directory of the scollector executable.")
	mains               []func()
)

func main() {
	flag.Parse()
	if *flagPrint || *flagDebug {
		slog.Set(&slog.StdLog{Log: log.New(os.Stdout, "", log.LstdFlags)})
	}
	if *flagVersion {
		fmt.Println(version.GetVersionInfo("scollector"))
		os.Exit(0)
	}
	for _, m := range mains {
		m()
	}
	conf := readConf()
	if *flagHost != "" {
		conf.Host = *flagHost
	}
	if *flagFilter != "" {
		conf.Filter = strings.Split(*flagFilter, ",")
	}
	if !conf.Tags.Valid() {
		slog.Fatalf("invalid tags: %v", conf.Tags)
	} else if conf.Tags["host"] != "" {
		slog.Fatalf("host not supported in custom tags, use Hostname instead")
	}
	if conf.PProf != "" {
		go func() {
			slog.Infof("Starting pprof at http://%s/debug/pprof/", conf.PProf)
			slog.Fatal(http.ListenAndServe(conf.PProf, nil))
		}()
	}
	collectors.AddTags = conf.Tags
	util.FullHostname = conf.FullHost
	util.Set()
	if conf.Hostname != "" {
		util.Hostname = conf.Hostname
	}
	if err := collect.SetHostname(util.Hostname); err != nil {
		slog.Fatal(err)
	}
	if conf.ColDir != "" {
		collectors.InitPrograms(conf.ColDir)
	}
	var err error
	check := func(e error) {
		if e != nil {
			err = e
		}
	}
	collectors.Init(conf)
	for _, rmq := range conf.RabbitMQ {
		check(collectors.RabbitMQ(rmq.URL))
	}
	for _, cfg := range conf.SNMP {
		check(collectors.SNMP(cfg, conf.MIBS))
	}
	for _, i := range conf.ICMP {
		check(collectors.ICMP(i.Host))
	}
	for _, a := range conf.AWS {
		check(collectors.AWS(a.AccessKey, a.SecretKey, a.Region))
	}
	for _, v := range conf.Vsphere {
		check(collectors.Vsphere(v.User, v.Password, v.Host))
	}
	for _, p := range conf.Process {
		check(collectors.AddProcessConfig(p))
	}
	for _, p := range conf.ProcessDotNet {
		check(collectors.AddProcessDotNetConfig(p))
	}
	for _, h := range conf.HTTPUnit {
		if h.TOML != "" {
			check(collectors.HTTPUnitTOML(h.TOML))
		}
		if h.Hiera != "" {
			check(collectors.HTTPUnitHiera(h.Hiera))
		}
	}
	for _, r := range conf.ElasticIndexFilters {
		check(collectors.AddElasticIndexFilter(r))
	}
	for _, r := range conf.Riak {
		check(collectors.Riak(r.URL))
	}
	if err != nil {
		slog.Fatal(err)
	}
	collectors.KeepalivedCommunity = conf.KeepalivedCommunity
	// Add all process collectors. This is platform specific.
	collectors.WatchProcesses()
	collectors.WatchProcessesDotNet()

	if *flagFake > 0 {
		collectors.InitFake(*flagFake)
	}
	collect.Debug = *flagDebug
	util.Debug = *flagDebug
	collect.DisableDefaultCollectors = conf.DisableSelf
	c := collectors.Search(conf.Filter)
	if len(c) == 0 {
		slog.Fatalf("Filter %v matches no collectors.", conf.Filter)
	}
	for _, col := range c {
		col.Init()
	}
	u, err := parseHost(conf.Host)
	if *flagList {
		list(c)
		return
	} else if err != nil {
		slog.Fatalf("invalid host %v: %v", conf.Host, err)
	}
	freq := time.Second * time.Duration(conf.Freq)
	if freq <= 0 {
		slog.Fatal("freq must be > 0")
	}
	collectors.DefaultFreq = freq
	collect.Freq = freq
	if conf.BatchSize < 0 {
		slog.Fatal("BatchSize must be > 0")
	}
	if conf.BatchSize != 0 {
		collect.BatchSize = conf.BatchSize
	}
	collect.Tags = conf.Tags.Copy().Merge(opentsdb.TagSet{"os": runtime.GOOS})
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

	if version.VersionDate != "" {
		v, err := strconv.ParseInt(version.VersionDate, 10, 64)
		if err == nil {
			go func() {
				metadata.AddMetricMeta("scollector.version", metadata.Gauge, metadata.None,
					"Scollector version number, which indicates when scollector was built.")
				for {
					if err := collect.Put("version", collect.Tags, v); err != nil {
						slog.Error(err)
					}
					time.Sleep(time.Hour)
				}
			}()
		}
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

func readConf() *conf.Conf {
	conf := &conf.Conf{
		Freq: 15,
	}
	loc := *flagConf
	if *flagConf == "" {
		p, err := exePath()
		if err != nil {
			slog.Error(err)
			return conf
		}
		dir := filepath.Dir(p)
		loc = filepath.Join(dir, "scollector.toml")
	}
	ext := filepath.Ext(loc)
	f, err := os.Open(loc)
	if err != nil {
		if *flagConf != "" {
			slog.Fatal(err)
		}
		if *flagDebug {
			slog.Error(err)
		}
	} else {
		defer f.Close()
		if ext == "toml" {
			md, err := toml.DecodeReader(f, conf)
			if err != nil {
				slog.Fatal(err)
			}
			if u := md.Undecoded(); len(u) > 0 {
				slog.Fatalf("extra keys in %s: %v", loc, u)
			}
		} else {
			decoder := json.NewDecoder(f)
			err := decoder.Decode(conf)
			if err != nil {
				slog.Fatal(err)
			}
		}
	}
	return conf
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

func parseHost(host string) (*url.URL, error) {
	if !strings.Contains(host, "//") {
		host = "http://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, fmt.Errorf("no host specified")
	}
	return u, nil
}

func printPut(c chan *opentsdb.DataPoint) {
	for dp := range c {
		b, _ := json.Marshal(dp)
		slog.Info(string(b))
	}
}
