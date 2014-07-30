// +build !windows

package collectors

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/util"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_ntp_peers_unix})
}

var ntpNtpqPeerFields = []string{
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

// ntUnPretty reverses human formating for poll and when, see prettyinterval in ntpq/ntpq-subs.c
func ntpUnPretty(s string) (int64, error) {
	if len(s) < 1 {
		return 0, fmt.Errorf("Zero length string passed to ntpUnPretty")
	}
	var multiplier int64 = 1
	shift := 1
	switch s[len(s)-1] {
	case 'm':
		multiplier = 60
	case 'h':
		multiplier = 60 * 60
	case 'd':
		multiplier = 60 * 60 * 24
	default:
		shift = 0
	}
	i, err := strconv.ParseInt(s[0:len(s)-shift], 10, 64)
	return i * multiplier, err
}

func c_ntp_peers_unix() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	const metric = "ntp."
	util.ReadCommand(func(line string) error {
		fields := strings.Fields(line)
		if len(fields) != len(ntpNtpqPeerFields) || fields[0] == "remote" {
			return nil
		}
		tags := opentsdb.TagSet{"remote": fields[0], "refid": fields[1]}
		var current_source int
		if strings.HasPrefix(fields[0], "*") {
			current_source = 1
		}
		Add(&md, metric+"current_source", current_source, tags, metadata.Gauge, metadata.Bool, "")
		Add(&md, metric+"stratum", fields[2], tags, metadata.Gauge, "Stratum", "")
		if when, err := ntpUnPretty(fields[4]); err != nil {
			return err
		} else {
			Add(&md, metric+"when", when, tags, metadata.Gauge, metadata.Second, "")
		}
		if poll, err := ntpUnPretty(fields[5]); err != nil {
			return err
		} else {
			Add(&md, metric+"poll", poll, tags, metadata.Gauge, metadata.Second, "")
		}
		Add(&md, metric+"reach", fields[6], tags, metadata.Gauge, "Code", "")
		Add(&md, metric+"delay", fields[7], tags, metadata.Gauge, metadata.MilliSecond, "")
		Add(&md, metric+"offset", fields[8], tags, metadata.Gauge, metadata.MilliSecond, "")
		Add(&md, metric+"jitter", fields[9], tags, metadata.Gauge, metadata.MilliSecond, "")
		return nil
	}, "ntpq", "-pn")
	return md, nil
}
