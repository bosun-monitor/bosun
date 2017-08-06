package collectors

import (
	"fmt"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"github.com/bosun-monitor/statusio"
)

func init() {
	registerInit(func(c *conf.Conf) {
		collectors = append(collectors, &IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return cIterableStat(c.Iterable.StatusBaseAddr)
			},
			name:     "cIterableStat",
			Interval: time.Minute * 2,
		})
	})
}

// components that we care about
// mapped to their tsdb key
// name => key
var componentKey = map[string]string{
	"Web Application": "webapp",
	"API":             "api",
	"Email Sending":   "email.sending",
	"Email Links":     "email.links",
	"Workflows":       "workflows",
	"Push Sending":    "pushSending",
	// "SMS Sending": nil,
	"System Webhooks":      "systemWebhooks",
	"Analytics Processing": "analyticsProcessing",
	"List Upload":          "listUpload",
}
var iterableComponentStatusDesc = fmt.Sprintf(fastlyComponentStatusDesc, "iterable asp")

const bosunPrefix = "iterable.status."

// Stat returns the MultiDataPoint with all the interesting
// components for iterable service.
// It uses status.io format (and library)
func cIterableStat(URL string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	c := statusio.NewClient(URL)
	c.SetScheme("http")
	summary, err := c.GetSummary() // TODO: add timeout
	if err != nil {
		return md, err
	}
	for _, comp := range summary.Components {
		if key, ok := componentKey[comp.Name]; ok {
			tagSet := opentsdb.TagSet{}
			Add(&md, bosunPrefix+key, int(comp.Status), tagSet,
				metadata.Gauge, metadata.StatusCode,
				iterableComponentStatusDesc)
		}
	}
	return md, nil
}
