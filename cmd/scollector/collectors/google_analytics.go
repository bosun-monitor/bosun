package collectors

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	analytics "google.golang.org/api/analytics/v3"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const descActiveUsers = "Number of unique users actively visiting the site."

type multiError []error

func (m multiError) Error() string {
	var fullErr string
	for _, err := range m {
		fullErr = fmt.Sprintf("%s\n%s", fullErr, err)
	}
	return fullErr
}

func init() {
	registerInit(func(c *conf.Conf) {
		for _, g := range c.GoogleAnalytics {
			collectors = append(collectors, &IntervalCollector{
				F: func() (opentsdb.MultiDataPoint, error) {
					return c_google_analytics(g.ClientID, g.Secret, g.Token, g.JSONToken, g.Sites)
				},
				name:     "c_google_analytics",
				Interval: time.Minute * 1,
			})
		}
	})
}

func c_google_analytics(clientid string, secret string, tokenstr string, jsonToken string, sites []conf.GoogleAnalyticsSite) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var mErr multiError

	c, err := googleAPIClient(clientid, secret, tokenstr, jsonToken, []string{analytics.AnalyticsScope})
	if err != nil {
		return nil, err
	}
	svc, err := analytics.New(c)
	if err != nil {
		return nil, err
	}

	// dimension: max records we want to fetch
	// "source" has a very long tail so we limit it to something sane
	// TODO: Dimensions we want and associated attributes should eventually be
	// setup in configuration.
	dimensions := map[string]int{"browser": -1, "trafficType": -1, "source": 10, "deviceCategory": -1, "operatingSystem": -1}
	for _, site := range sites {
		getPageviews(&md, svc, site)
		if site.Detailed {
			if err = getActiveUsers(&md, svc, site); err != nil {
				mErr = append(mErr, err)
			}
			for dimension, topN := range dimensions {
				if err = getActiveUsersByDimension(&md, svc, site, dimension, topN); err != nil {
					mErr = append(mErr, err)
				}
			}
		}
	}

	if len(mErr) == 0 {
		return md, nil
	} else {
		return md, mErr
	}
}

type kv struct {
	key   string
	value int
}

type kvList []kv

func (p kvList) Len() int           { return len(p) }
func (p kvList) Less(i, j int) bool { return p[i].value < p[j].value }
func (p kvList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func getActiveUsersByDimension(md *opentsdb.MultiDataPoint, svc *analytics.Service, site conf.GoogleAnalyticsSite, dimension string, topN int) error {
	call := svc.Data.Realtime.Get("ga:"+site.Profile, "rt:activeusers").Dimensions("rt:" + dimension)
	data, err := call.Do()
	if err != nil {
		return err
	}
	tags := opentsdb.TagSet{"site": site.Name}
	rows := make(kvList, len(data.Rows))
	for i, row := range data.Rows {
		// key will always be an string of the dimension we care about.
		// For example, 'Chrome' would be a key for the 'browser' dimension.
		key, _ := opentsdb.Clean(row[0])
		if key == "" {
			key = "__blank__"
		}
		value, err := strconv.Atoi(row[1])
		if err != nil {
			return fmt.Errorf("Error parsing GA data: %s", err)
		}
		rows[i] = kv{key: key, value: value}
	}
	sort.Sort(sort.Reverse(rows))
	if topN != -1 && topN < len(rows) {
		topRows := make(kvList, topN)
		topRows = rows[:topN]
		rows = topRows
	}

	for _, row := range rows {
		Add(md, "google.analytics.realtime.activeusers.by_"+dimension, row.value, opentsdb.TagSet{dimension: row.key}.Merge(tags), metadata.Gauge, metadata.ActiveUsers, descActiveUsers)
	}
	return nil
}

func getActiveUsers(md *opentsdb.MultiDataPoint, svc *analytics.Service, site conf.GoogleAnalyticsSite) error {
	call := svc.Data.Realtime.Get("ga:"+site.Profile, "rt:activeusers")
	data, err := call.Do()
	if err != nil {
		return err
	}
	tags := opentsdb.TagSet{"site": site.Name}
	if len(data.Rows) < 1 || len(data.Rows[0]) < 1 {
		return fmt.Errorf("no active user data in response for site %v", site.Name)
	}
	value, err := strconv.Atoi(data.Rows[0][0])
	if err != nil {
		return fmt.Errorf("Error parsing GA data: %s", err)
	}

	Add(md, "google.analytics.realtime.activeusers", value, tags, metadata.Gauge, metadata.ActiveUsers, descActiveUsers)
	return nil
}

func getPageviews(md *opentsdb.MultiDataPoint, svc *analytics.Service, site conf.GoogleAnalyticsSite) error {
	call := svc.Data.Realtime.Get("ga:"+site.Profile, "rt:pageviews").Dimensions("rt:minutesAgo")
	data, err := call.Do()
	if err != nil {
		return err
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
			return fmt.Errorf("Error parsing GA data: %s", err)
		}
		if minute == offset {
			if pageviews, err = strconv.Atoi(row[1]); err != nil {
				return fmt.Errorf("Error parsing GA data: %s", err)
			}
			break
		}
	}
	AddTS(md, "google.analytics.realtime.pageviews", time, pageviews, opentsdb.TagSet{"site": site.Name}, metadata.Gauge, metadata.Count, "Number of pageviews tracked by GA in one minute")
	return nil
}

// googleAPIClient() takes in a clientid, secret, a base64'd gob representing
// the cached oauth token, and a list of oauth scopes.  Generating the token is
// left as an exercise to the reader.
// Or use a base 64 encoded service account json key. Provide json key OR oauth client info.
func googleAPIClient(clientid string, secret string, tokenstr string, jsonToken string, scopes []string) (*http.Client, error) {

	if jsonToken != "" && clientid+secret+tokenstr != "" {
		return nil, fmt.Errorf("For google, provide a json token OR oauth client info and token. Not both")
	}

	ctx := context.Background()
	if jsonToken != "" {
		by, err := base64.StdEncoding.DecodeString(jsonToken)
		if err != nil {
			return nil, err
		}
		config, err := google.JWTConfigFromJSON(by, scopes...)
		if err != nil {
			return nil, err
		}
		return config.Client(ctx), nil
	}

	config := &oauth2.Config{
		ClientID:     clientid,
		ClientSecret: secret,
		Endpoint:     google.Endpoint,
		Scopes:       scopes,
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
