package collectors

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
	"github.com/StackExchange/slog"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_ntp_peers_unix})
}

var FIELDS_NTPQ_PEERS = []string{
	"remote",
	"refid",
	"st",
	"t",
	"when",
	"poll",
	"reach",
	"delay",
	"offset",
	"jitter",
}

var whenRE = regexp.MustCompile(`(-?\d+)([dmh]?)`)

// Reverse human formating for poll and when, see prettyinterval in ntpq/ntpq-subs.c
func unPretty(s string) (int64, error) {
	re := whenRE.FindStringSubmatch(s)
	var i int64
	if len(re) <= 1 {
		return i, fmt.Errorf("ntp failed to parse 'when' field")
	}
	i, err := strconv.ParseInt(re[1], 10, 64)
	if err != nil {
		return i, err
	}
	if len(re) == 3 {
		switch re[2] {
		case "m":
			return i * 60, nil
		case "h":
			return i * 60 * 60, nil
		case "d":
			return i * 60 * 60 * 24, nil
		}
	}
	return i, nil
}

func c_ntp_peers_unix() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	metric := "ntp."
	util.ReadCommand(func(line string) {
		fields := strings.Fields(line)
		if line == "" || len(fields) != len(FIELDS_NTPQ_PEERS) || fields[0] == "remote" {
			return
		}
		tags := opentsdb.TagSet{"remote": fields[0], "refid": fields[1]}
		var current_source int
		if strings.HasPrefix(fields[0], "*") {
			current_source = 1
		}
		Add(&md, metric+"current_source", current_source, tags, metadata.Gauge, metadata.None, "")
		Add(&md, metric+"stratum", fields[2], tags, metadata.Gauge, metadata.None, "")
		if when, err := unPretty(fields[4]); err == nil {
			Add(&md, metric+"when", when, tags, metadata.Gauge, metadata.None, "")
		} else {
			slog.Error(err)
		}
		if poll, err := unPretty(fields[5]); err == nil {
			Add(&md, metric+"poll", poll, tags, metadata.Gauge, metadata.None, "")
		} else {
			slog.Error(err)
		}
		Add(&md, metric+"reach", fields[6], tags, metadata.Gauge, metadata.None, "")
		Add(&md, metric+"delay", fields[7], tags, metadata.Gauge, metadata.MilliSecond, "")
		Add(&md, metric+"offset", fields[8], tags, metadata.Gauge, metadata.MilliSecond, "")
		Add(&md, metric+"jitter", fields[9], tags, metadata.Gauge, metadata.MilliSecond, "")
	}, "ntpq", "-pn")
	return md, nil
}
