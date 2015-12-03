package collectors

import (
	"encoding/json"
	"os/exec"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
)

func init() {
	if _, err := exec.LookPath("varnishstat"); err == nil {
		collectors = append(collectors, &IntervalCollector{F: c_varnish_unix})
	}
}

func c_varnish_unix() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	const metric = "varnish."

	r, err := util.Command(5*time.Second, nil, "varnishstat", "-j")
	if err != nil {
		return nil, err
	}

	var stats varnishStats
	if err := json.NewDecoder(r).Decode(&stats); err != nil {
		return nil, err
	}

	for name, raw := range stats {
		if name == "timestamp" {
			continue
		}

		var v varnishStat
		if err := json.Unmarshal(raw, &v); err != nil {
			slog.Errorln("varnish parser error:", name, err)
			continue
		}

		ts := opentsdb.TagSet{"type": v.Type}
		if v.SubType != "" {
			ts.Merge(opentsdb.TagSet{"subtype": v.SubType})
		}

		rate := metadata.RateType(metadata.Gauge)
		if flag := v.Flag; flag == "a" || flag == "c" {
			rate = metadata.Counter
		}

		unit := metadata.Unit(metadata.Count)
		if v.Format == "B" {
			unit = metadata.Bytes
		}

		Add(&md, metric+name, v.Value, ts, rate, unit, v.Desc)
	}
	return md, nil
}

/*
{
  "timestamp": "YYYY-MM-DDTHH:mm:SS",
  "FIELD NAME": {
    "description": "FIELD DESCRIPTION",
    "type": "FIELD TYPE", "ident": "FIELD IDENT", "flag": "FIELD SEMANTICS", "format": "FIELD DISPLAY FORMAT",
    "value": FIELD VALUE
  },
  "FIELD2 NAME": {
    "description": "FIELD2 DESCRIPTION",
    "type": "FIELD2 TYPE", "ident": "FIELD2 IDENT", "flag": "FIELD2 SEMANTICS", "format": "FIELD2 DISPLAY FORMAT",
    "value": FIELD2 VALUE
  },
  [..]
}
*/
type varnishStats map[string]json.RawMessage

type varnishStat struct {
	Desc    string `json:"description"`
	Type    string `json:"type"`
	SubType string `json:"ident"`
	// for older version, flag can be either 'a', 'c', 'g' or 'i'
	// for newer version 'a' become 'c' and 'i' become 'g', which means
	// counter and gauge, see:
	// https://github.com/varnish/Varnish-Cache/commit/5cceef815
	Flag string `json:"flag"`
	// newer version add a format field
	// currently there are 'i' for integer and 'B' for bytes
	Format string `json:"format"`
	Value  int64  `json:"value"`
}
