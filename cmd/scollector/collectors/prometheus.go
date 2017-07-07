package collectors

import (
	"fmt"
	"net/http"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/opentsdb"
	"bosun.org/slog"

	"io"

	"bosun.org/metadata"
	models "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

func init() {
	registerInit(initPrometheus)
}

func initPrometheus(c *conf.Conf) {
	for _, endpoint := range c.Prometheus {
		collectors = append(collectors, &IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_prometheus(endpoint)
			},
			name: "prometheus",
		})
	}
}

func c_prometheus(endpoint string) (mdp opentsdb.MultiDataPoint, err error) {
	defer func() {
		if err != nil {
			fmt.Println("!!!!!", err)
		}
	}()
	resp, err := http.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	dec := expfmt.NewDecoder(resp.Body, expfmt.FmtUnknown)
	mf := &models.MetricFamily{}
families:
	for {
		err = dec.Decode(mf)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		desc := mf.GetHelp()
		name := mf.GetName()
		for _, m := range mf.Metric {
			ts := m.GetTimestampMs() / 1000
			if ts == 0 {
				ts = now()
			}
			tags := opentsdb.TagSet{}
			for _, l := range m.Label {
				tags[l.GetName()] = l.GetValue()
			}
			if _, ok := tags["host"]; !ok {
				tags["host"] = ""
			}
			switch mf.GetType() {
			case models.MetricType_COUNTER:
				AddTS(&mdp, name, ts, m.GetCounter().GetValue(), tags, metadata.Counter, "", desc)
			case models.MetricType_GAUGE:
				AddTS(&mdp, name, ts, m.GetGauge().GetValue(), tags, metadata.Gauge, "", desc)
			case models.MetricType_HISTOGRAM:
				fmt.Println("HISTO", name, desc)
				hist := m.GetHistogram()
				AddTS(&mdp, name+"_count", ts, hist.GetSampleCount(), tags, metadata.Counter, "", desc+" (total count)")
				AddTS(&mdp, name+"_sum", ts, hist.GetSampleSum(), tags, metadata.Gauge, "", desc+" (sum of values)")
				for _, b := range hist.GetBucket() {
					tags["le"] = fmt.Sprint(b.GetUpperBound())
					AddTS(&mdp, name, ts, b.GetCumulativeCount(), tags, metadata.Counter, "", desc)
				}
			default:
				slog.Errorf("Unimplemented prometheus metric type: %s", mf.GetType().String())
				continue families
			}

		}
	}
}
