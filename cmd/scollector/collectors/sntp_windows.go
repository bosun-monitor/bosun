package collectors

import (
	"fmt"
	"strings"
	"time"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_sntp_windows})
}

func c_sntp_windows() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	const metric = "sntp."
	var (
		stratum string
		delay   string
		when    float64
		source  string
		poll    string
	)
	if err := util.ReadCommand(func(line string) error {
		f := strings.Split(line, ":")
		if len(f) < 1 {
			return nil
		}
		switch f[0] {
		case "Stratum":
			sf := strings.Fields(f[1])
			if len(sf) < 1 {
				return fmt.Errorf("Unexpected value for stratum")
			}
			stratum = sf[0]
		case "Root Delay":
			delay = strings.Trim(f[1], "s")
		case "Last Successful Sync Time":
			if t, err := time.Parse("1/2/2006 3:04:05 PM", strings.TrimSpace(strings.Join(f[1:len(f)], ":"))); err != nil {
				return err
			} else {
				when = time.Since(t).Seconds()
			}
		case "Source":
			source = strings.TrimSpace(f[1])
		case "Poll Interval":
			sf := strings.Fields(f[1])
			if len(sf) != 2 {
				return fmt.Errorf("Unexpected value for Poll Interval")
			}
			poll = strings.Trim(sf[1], "()s")
		}
		return nil
	}, "w32tm", "/query", "/status"); err != nil {
		return md, err
	}
	tags := opentsdb.TagSet{"remote": source}
	Add(&md, metric+"stratum", stratum, tags, metadata.Gauge, "Stratum", "")
	Add(&md, metric+"delay", delay, tags, metadata.Gauge, metadata.Second, "")
	Add(&md, metric+"when", when, tags, metadata.Gauge, metadata.Second, "")
	Add(&md, metric+"poll", poll, tags, metadata.Gauge, metadata.Second, "")
	if err := util.ReadCommand(func(line string) error {
		if !strings.Contains(line, ",") {
			return nil
		}
		f := strings.Split(line, ",")
		if len(f) < 2 {
			return fmt.Errorf("Unexpected output for ")
		}
		Add(&md, metric+"offset", strings.Trim(f[1], "s"), tags, metadata.Gauge, metadata.Second, "")
		return nil
	}, "w32tm", "/stripchart", fmt.Sprintf("/computer:%v", source), "/samples:1", "/dataonly"); err != nil {
		return md, err
	}
	return md, nil
}
