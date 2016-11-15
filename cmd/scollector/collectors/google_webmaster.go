package collectors

import (
	"net/url"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"

	"google.golang.org/api/webmasters/v3"
)

func init() {
	registerInit(func(c *conf.Conf) {
		for _, g := range c.GoogleWebmaster {
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return c_google_webmaster(g.ClientID, g.Secret, g.Token)
				},
				name:     "c_google_webmaster",
				Interval: time.Hour * 1,
			})
		}
	})
}

func c_google_webmaster(clientID, secret, tokenStr string) (opentsdb.MultiDataPoint, error) {
	c, err := googleAPIClient(clientID, secret, tokenStr, []string{webmasters.WebmastersReadonlyScope})
	if err != nil {
		return nil, err
	}

	svc, err := webmasters.New(c)
	if err != nil {
		return nil, err
	}

	md, err := getWebmasterErrorsMetrics(svc)
	if err != nil {
		return nil, err
	}

	return *md, nil
}

// getWebmasterErrorsMetric utilizes the webmasters API to list all sites
// associated with an authenticated account, fetch the time-series data error
// metrics for each of those sites, and builds opentsdb datapoints from that
// data.
func getWebmasterErrorsMetrics(svc *webmasters.Service) (*opentsdb.MultiDataPoint, error) {
	md := &opentsdb.MultiDataPoint{}

	sites, err := svc.Sites.List().Do()
	if err != nil {
		return nil, err
	}

	metricName := "google.webmaster.errors"
	for _, site := range sites.SiteEntry {
		u, err := url.Parse(site.SiteUrl)

		if err != nil {
			return nil, err
		}
		if site.PermissionLevel == "siteUnverifiedUser" {
			slog.Errorf("Lack permission to fetch error metrics for site %s. Skipping.\n", u.Host)
			continue
		}
		tags := opentsdb.TagSet{}
		tags["site"] = u.Host
		crawlErrors, err := svc.Urlcrawlerrorscounts.Query(site.SiteUrl).LatestCountsOnly(true).Do()
		if err != nil {
			return nil, err
		}
		for _, e := range crawlErrors.CountPerTypes {
			tags["platform"] = e.Platform
			tags["category"] = e.Category
			for _, entry := range e.Entries {
				t, err := time.Parse(time.RFC3339, entry.Timestamp)
				if err != nil {
					return md, err
				}
				AddTS(md, metricName, t.Unix(), entry.Count, tags, metadata.Gauge, metadata.Error, "")
			}
		}
	}

	return md, nil
}
