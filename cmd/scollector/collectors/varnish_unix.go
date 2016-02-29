package collectors

import (
	"encoding/json"
	"os/exec"
	"strings"
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

		ts := opentsdb.TagSet{}

		// special case for backend stats. extract backend name, host and port, put
		// them in tags and remove them in name.
		// the format is like "name(host,,port)" for the "ident" field of "VBE" type
		if v.Type == "VBE" {
			subtype := v.SubType

			name = strings.Replace(name, "."+subtype, "", -1)

			idx := strings.Index(subtype, "(")
			if idx < 0 || len(subtype)-idx < 4 {
				// output format changed, ignore
				continue
			}

			ss := strings.Split(subtype[idx+1:len(subtype)-1], ",")
			if len(ss) != 3 {
				// output format changed, ignore
				continue
			}

			ts.Merge(opentsdb.TagSet{"backend": subtype[:idx]})
			ts.Merge(opentsdb.TagSet{"endpoint": ss[0] + "_" + ss[2]})
		}

		rate := metadata.RateType(metadata.Gauge)
		if flag := v.Flag; flag == "a" || flag == "c" {
			rate = metadata.Counter
		}

		unit := metadata.Unit(metadata.Count)
		if v.Format == "B" {
			unit = metadata.Bytes
		}

		Add(&md, metric+strings.ToLower(name), v.Value, ts, rate, unit, v.Desc)
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
