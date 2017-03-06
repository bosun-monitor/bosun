package collectors

import (
	"net/url"
	"strings"
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

	throttle := time.Tick(time.Second / 2)

	<-throttle
	sites, err := svc.Sites.List().Do()
	if err != nil {
		return nil, err
	}

	for _, site := range sites.SiteEntry {
		u, err := url.Parse(site.SiteUrl)
		if err != nil {
			return nil, err
		}
		// Webmasters has these new sets with fake URLs like "sc-set:jJfZIHyI4-DY8wg0Ww4l-A".
		// Most API calls we use fail for these sites, so we skip em.
		// TODO: Allow these sites once the API supports em.
		if strings.HasPrefix(site.SiteUrl, "sc-set") {
			continue
		}
		if site.PermissionLevel == "siteUnverifiedUser" {
			slog.Errorf("Lack permission to fetch error metrics for site %s. Skipping.\n", u.Host)
			continue
		}
		tags := opentsdb.TagSet{}
		tags["site"] = u.Host
		tags["scheme"] = u.Scheme
		tags["path"] = u.Path
		<-throttle
		crawlErrors, err := svc.Urlcrawlerrorscounts.Query(site.SiteUrl).LatestCountsOnly(true).Do()
		if err != nil {
			slog.Errorf("Error fetching error counts for site %s: %s", u.Host, err)
			continue
		}
		for _, e := range crawlErrors.CountPerTypes {
			tags["platform"] = e.Platform
			tags["category"] = e.Category
			for _, entry := range e.Entries {
				t, err := time.Parse(time.RFC3339, entry.Timestamp)
				if err != nil {
					return md, err
				}
				AddTS(md, "google.webmaster.errors", t.Unix(), entry.Count, tags, metadata.Gauge, metadata.Error, descGoogleWebmasterErrors)
			}
		}
	}

	return md, nil
}

const descGoogleWebmasterErrors = "The number of crawl errors that Google experienced on a given site. Note that if Google webmaster is tracking multiple paths for a given site, then error counts in parent paths (/) will include errors from any child paths (/foo). As such, aggregation across parent and child paths will result in erroneous results."
