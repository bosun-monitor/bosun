package collectors

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"net/http"
	"strconv"
	"time"

	analytics "bosun.org/_third_party/google.golang.org/api/analytics/v3"

	"bosun.org/_third_party/golang.org/x/net/context"
	"bosun.org/_third_party/golang.org/x/oauth2"
	"bosun.org/_third_party/golang.org/x/oauth2/google"
	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	registerInit(func(c *conf.Conf) {
		for _, g := range c.GoogleAnalytics {
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return c_google_analytics(g.ClientID, g.Secret, g.Token, g.Sites)
				},
				name:     "c_google_analytics",
				Interval: time.Minute * 1,
			})
		}
	})
}

func c_google_analytics(clientid string, secret string, tokenstr string, sites []conf.GoogleAnalyticsSite) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint

	c, err := analyticsClient(clientid, secret, tokenstr)
	if err != nil {
		return nil, err
	}
	svc, err := analytics.New(c)
	if err != nil {
		return nil, err
	}

	for _, site := range sites {
		call := svc.Data.Realtime.Get("ga:"+site.Profile, "rt:pageviews").Dimensions("rt:minutesAgo")
		data, err := call.Do()
		if err != nil {
			return md, err
		}

		// If no offset was specified, the minute we care about is '1', or the most
		// recently gathered, completed datapoint. Minute '0' is the current minute,
		// and as such is incomplete.
		offset := site.Offset
		if offset == 0 {
			offset = 1
		}
		time := time.Now().Add(time.Duration(-1*offset) * time.Minute).Unix()
		pageviews := 0
		// Iterates through the response data and returns the time slice we
		// actually care about when we find it.
		for _, row := range data.Rows {
			// row == [2]string{"0", "123"}
			// First item is the minute, second is the data (pageviews in this case)
			minute, err := strconv.Atoi(row[0])
			if err != nil {
				return md, fmt.Errorf("Error parsing GA data: %s", err)
			}
			if minute == offset {
				if pageviews, err = strconv.Atoi(row[1]); err != nil {
					return md, fmt.Errorf("Error parsing GA data: %s", err)
				}
				break
			}
		}
		AddTS(&md, "google.analytics.realtime.pageviews", time, pageviews, opentsdb.TagSet{"site": site.Name}, metadata.Gauge, metadata.Count, "Number of pageviews tracked by GA in one minute")
	}

	return md, err
}

// analyticsClient() takes in a clientid, secret, and a base64'd gob representing the cached oauth token.
// Generating the token is left as an exercise to the reader. (TODO)
func analyticsClient(clientid string, secret string, tokenstr string) (*http.Client, error) {
	ctx := context.Background()

	config := &oauth2.Config{
		ClientID:     clientid,
		ClientSecret: secret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{analytics.AnalyticsScope},
	}

	token := new(oauth2.Token)
	// Decode the base64'd gob
	by, err := base64.StdEncoding.DecodeString(tokenstr)
	if err != nil {
		return nil, err
	}
	b := bytes.Buffer{}
	b.Write(by)
	d := gob.NewDecoder(&b)
	err = d.Decode(&token)

	return config.Client(ctx, token), nil
}
