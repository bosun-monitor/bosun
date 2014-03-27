package web

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/expr"
)

// Graph takes an OpenTSDB request data structure and queries OpenTSDB. Use the
// json parameter to pass JSON. Use the b64 parameter to pass base64-encoded
// JSON.
func Graph(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	j := []byte(r.FormValue("json"))
	if bs := r.FormValue("b64"); bs != "" {
		b, err := base64.URLEncoding.DecodeString(bs)
		if err != nil {
			return nil, err
		}
		j = b
	}
	if len(j) == 0 {
		return nil, fmt.Errorf("either json or b64 required")
	}
	oreq, err := opentsdb.RequestFromJSON(j)
	if err != nil {
		return nil, err
	}
	ads_v := r.FormValue("autods")
	if ads_v != "" {
		ads_i, err := strconv.ParseInt(ads_v, 10, 64)
		if err != nil {
			return nil, err
		}
		if err := oreq.AutoDownsample(ads_i); err != nil {
			return nil, err
		}
	}
	for _, q := range oreq.Queries {
		if err := expr.ExpandSearch(q); err != nil {
			return nil, err
		}
	}
	if _, present := r.Form["png"]; present {
		u := url.URL{
			Scheme:   "http",
			Host:     schedule.Conf.TsdbHost,
			Path:     "/q",
			RawQuery: oreq.String() + "&png",
		}
		resp, err := http.Get(u.String())
		if err != nil {
			return nil, err
		}
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		_, err = io.Copy(w, resp.Body)
		return nil, err
	}
	var tr opentsdb.ResponseSet
	q, _ := url.QueryUnescape(oreq.String())
	t.StepCustomTiming("tsdb", "query", q, func() {
		tr, err = oreq.Query(schedule.Conf.TsdbHost)
	})
	if err != nil {
		return nil, err
	}
	return rickchart(tr)
}

var q_re = regexp.MustCompile(`"([^"]+)"\s*,\s*"([^"]+)"`)
var r_re = regexp.MustCompile(`^(.*?")[^"]+(".*)`)

func ExprGraph(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	q := r.FormValue("q")
	qs := q_re.FindStringSubmatch(q)
	if qs == nil {
		return nil, errors.New("couldn't parse out query")
	}
	pq, err := opentsdb.ParseQuery(qs[1])
	oreq := opentsdb.Request{
		Start:   qs[2] + "-ago",
		Queries: []*opentsdb.Query{pq},
	}
	ads_v := r.FormValue("autods")
	if ads_v != "" {
		ads_i, err := strconv.ParseInt(ads_v, 10, 64)
		if err != nil {
			return nil, err
		}
		if err := oreq.AutoDownsample(ads_i); err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	n_qs := r_re.ReplaceAllString(q, "${1}"+oreq.Queries[0].String()+"${2}")
	e, err := expr.New(n_qs)
	if err != nil {
		return nil, err
	}
	res, _, err := e.Execute(opentsdb.Host(schedule.Conf.TsdbHost), t)
	if err != nil {
		return nil, err
	}
	return rickexpr(res, n_qs)
}

func rickexpr(r []*expr.Result, q string) ([]*RickSeries, error) {
	var series []*RickSeries
	for _, res := range r {
		dps := make([]RickDP, 0)
		var rv expr.Series
		var ok bool
		if rv, ok = res.Value.(expr.Series); !ok {
			return series, errors.New("expr must return a series")
		}
		for k, v := range rv {
			ki, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				return nil, err
			}
			dps = append(dps, RickDP{
				X: ki,
				Y: v,
			})
		}
		if len(dps) > 0 {
			sort.Sort(ByX(dps))
			name := q
			var id []string
			for k, v := range res.Group {
				id = append(id, fmt.Sprintf("%v=%v", k, v))
			}
			if len(id) > 0 {
				name = fmt.Sprintf("%s{%s}", name, strings.Join(id, ","))
			}
			series = append(series, &RickSeries{
				Name: name,
				Data: dps,
			})
		}
	}
	return series, nil
}

func rickchart(r opentsdb.ResponseSet) ([]*RickSeries, error) {
	var series []*RickSeries
	for _, resp := range r {
		dps := make([]RickDP, 0)
		for k, v := range resp.DPS {
			ki, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				return nil, err
			}
			dps = append(dps, RickDP{
				X: ki,
				Y: v,
			})
		}
		if len(dps) > 0 {
			sort.Sort(ByX(dps))
			name := resp.Metric
			var id []string
			for k, v := range resp.Tags {
				id = append(id, fmt.Sprintf("%v=%v", k, v))
			}
			if len(id) > 0 {
				name = fmt.Sprintf("%s{%s}", name, strings.Join(id, ","))
			}
			series = append(series, &RickSeries{
				Name: name,
				Data: dps,
			})
		}
	}
	return series, nil
}

type RickSeries struct {
	Name string   `json:"name"`
	Data []RickDP `json:"data"`
}

type RickDP struct {
	X int64          `json:"x"`
	Y opentsdb.Point `json:"y"`
}

type ByX []RickDP

func (a ByX) Len() int           { return len(a) }
func (a ByX) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByX) Less(i, j int) bool { return a[i].X < a[j].X }
