package collectors

import (
	"fmt"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_sntp_windows})
}

func c_sntp_windows() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	const metric = "sntp."
	var (
		stratum string
		delay   float64
		when    float64
		source  string
		poll    float64
	)
	if err := util.ReadCommand(func(line string) error {
		f := strings.SplitN(line, ":", 2)
		if len(f) != 2 {
			return nil
		}
		f[1] = strings.TrimSpace(f[1])
		switch f[0] {
		case "Stratum":
			sf := strings.Fields(f[1])
			if len(sf) < 1 {
				return fmt.Errorf("Unexpected value for stratum")
			}
			stratum = sf[0]
		case "Root Delay":
			d, err := time.ParseDuration(f[1])
			if err != nil {
				return err
			}
			delay = d.Seconds()
		case "Last Successful Sync Time":
			if f[1] == "unspecified" {
				break
			}
			t, err := time.Parse("1/2/2006 3:04:05 PM", f[1])
			if err != nil {
				return err
			}
			when = time.Since(t).Seconds()
		case "Source":
			source = strings.TrimSpace(f[1])
		case "Poll Interval":
			sf := strings.Fields(f[1])
			if len(sf) != 2 {
				return fmt.Errorf("Unexpected value for Poll Interval")
			}
			s := strings.Trim(sf[1], "()")
			d, err := time.ParseDuration(strings.TrimSpace(s))
			if err != nil {
				return err
			}
			poll = d.Seconds()
		}
		return nil
	}, "w32tm", "/query", "/status"); err != nil {
		return nil, nil
	}
	tags := opentsdb.TagSet{"remote": source}
	Add(&md, metric+"stratum", stratum, tags, metadata.Gauge, "Stratum", "")
	Add(&md, metric+"delay", delay, tags, metadata.Gauge, metadata.Second, "")
	Add(&md, metric+"when", when, tags, metadata.Gauge, metadata.Second, "")
	Add(&md, metric+"poll", poll, tags, metadata.Gauge, metadata.Second, "")
	_ = util.ReadCommand(func(line string) error {
		f := strings.SplitN(line, ",", 2)
		if len(f) != 2 {
			return nil
		}
		d, err := time.ParseDuration(strings.TrimSpace(f[1]))
		if err != nil {
			return nil
		}
		Add(&md, metric+"offset", d.Seconds(), tags, metadata.Gauge, metadata.Second, "")
		return nil
	}, "w32tm", "/stripchart", fmt.Sprintf("/computer:%v", source), "/samples:1", "/dataonly")
	return md, nil
}
