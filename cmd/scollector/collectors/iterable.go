package collectors

import (
	"context"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/bosun-monitor/statusio"
)

// iterable.io collector
// to use add this in your scollector.toml:
// [Iterable]
//   StatusBaseAddr = "https://iterable.statuspage.io"
//   # TsdbPrefix = "iterable.status."
//   # MaxDuration = "10s"

type IterComp map[string]string

func init() {
	const (
		defaultStatusBaseAddr = "https://iterable.statuspage.io"
		defaultTsdbPrefix     = "iterable.status."
		defaultMaxDuration    = conf.Duration(10 * time.Second)
	)

	// components that we care about
	// mapped to their tsdb key
	// name => key
	var componentKey = IterComp{
		"Web Application": "webapp",
		"API":             "api",
		"Email Sending":   "email.sending",
		"Email Links":     "email.links",
		"Workflows":       "workflows",
		//"Push Sending":    "pushSending",
		// "SMS Sending": nil,
		//"System Webhooks":      "systemWebhooks",
		//"Analytics Processing": "analyticsProcessing",
		//"List Upload":          "listUpload",
		"User Updates": "userUpdate",
	}

	registerInit(func(c *conf.Conf) {
		// use default values when unspecified
		iter := c.Iterable
		if iter.StatusBaseAddr == "" {
			slog.Warningf("No StatusBaseAddr given, using: '%s'", defaultStatusBaseAddr)
			iter.StatusBaseAddr = defaultStatusBaseAddr
		}
		if iter.TsdbPrefix == "" {
			iter.TsdbPrefix = defaultTsdbPrefix
		}
		if iter.MaxDuration == conf.Duration(0) {
			iter.MaxDuration = defaultMaxDuration
		}
		if iter.MaxDuration <= conf.Duration(0) || iter.MaxDuration > conf.Duration(10000000*time.Second) {
			slog.Fatalf("Iterable: invalid MaxDuration: %d", iter.MaxDuration)
		}

		collectors = append(collectors, &IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(iter.MaxDuration)*time.Second)
				defer cancel()
				return iterable(ctx, iter, componentKey)
			},
			name:     "c_iterable_status",
			Interval: time.Minute * 5,
		})
	})
}

// iterable() returns the MultiDataPoint with all the interesting
// components for iterable service.
// It uses status.io format (and library)
func iterable(ctx context.Context, iter conf.Iterable, compKey IterComp) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	c := statusio.NewClient(iter.StatusBaseAddr)
	summary, err := c.GetSummary(ctx)
	if err != nil {
		return md, err
	}
	for _, comp := range summary.Components {
		if key, ok := compKey[comp.Name]; ok {
			Add(&md, iter.TsdbPrefix+key, int(comp.Status), opentsdb.TagSet{},
				metadata.Gauge, metadata.StatusCode,
				"Iterable status: 0: Operational, 1: Degraded Performance, 2: Partial Outage, 3: Major Outage.")
		}
	}
	return md, nil
}
