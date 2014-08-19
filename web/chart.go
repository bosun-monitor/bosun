package web

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	svg "github.com/StackExchange/bosun/_third_party/github.com/ajstarks/svgo"
	"github.com/StackExchange/bosun/_third_party/github.com/bradfitz/slice"
	"github.com/StackExchange/bosun/_third_party/github.com/gorilla/mux"
	"github.com/StackExchange/bosun/_third_party/github.com/vdobler/chart"
	"github.com/StackExchange/bosun/_third_party/github.com/vdobler/chart/svgg"
	"github.com/StackExchange/bosun/expr"
	"github.com/StackExchange/bosun/expr/parse"
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
		ads_i, err := strconv.Atoi(ads_v)
		if err != nil {
			return nil, err
		}
		if err := oreq.AutoDownsample(ads_i); err != nil {
			return nil, err
		}
	}
	ar := make(map[int]bool)
	for _, v := range r.Form["autorate"] {
		if i, err := strconv.Atoi(v); err == nil {
			ar[i] = true
		}
	}
	for i, q := range oreq.Queries {
		if err := schedule.Search.Expand(q); err != nil {
			return nil, err
		}
		if ar[i] {
			ms := schedule.GetMetadata(q.Metric, nil)
			q.Rate = true
			q.RateOptions = opentsdb.RateOptions{
				Counter:    true,
				ResetValue: 1,
			}
			for _, m := range ms {
				if m.Name == "rate" {
					switch m.Value {
					case metadata.Gauge:
						q.Rate = false
						q.RateOptions.Counter = false
					case metadata.Rate:
						q.RateOptions.Counter = false
					}
					break
				}
			}
		}
	}
	if _, present := r.Form["png"]; present {
		u := url.URL{
			Scheme:   "http",
			Host:     schedule.Conf.TsdbHost,
			Path:     "/q",
			RawQuery: oreq.String() + "&png",
		}
		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
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
	b, _ := json.MarshalIndent(oreq, "", "  ")
	t.StepCustomTiming("tsdb", "query", string(b), func() {
		tr, err = oreq.Query(schedule.Conf.TsdbHost)
	})
	if err != nil {
		return nil, err
	}
	chart, err := makeChart(tr)
	if err != nil {
		return nil, err
	}
	return struct {
		Queries []string
		Series  []*chartSeries
	}{
		QFromR(oreq),
		chart,
	}, nil
}

func QFromR(req *opentsdb.Request) []string {
	queries := make([]string, len(req.Queries))
	var start, end string
	if s, ok := req.Start.(string); ok && strings.Contains(s, "-ago") {
		start = strings.TrimSuffix(s, "-ago")
	}
	if s, ok := req.End.(string); ok && strings.Contains(s, "-ago") {
		end = strings.TrimSuffix(s, "-ago")
	}
	for i, q := range req.Queries {
		queries[i] = fmt.Sprintf(`q("%v", "%v", "%v")`, q, start, end)
	}
	return queries
}

func ExprGraph(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	bs := vars["bs"]
	b, err := base64.URLEncoding.DecodeString(bs)
	if err != nil {
		return nil, err
	}
	q := string(b)
	if len(q) == 0 {
		return nil, fmt.Errorf("missing expression")
	}
	e, err := expr.New(q)
	if err != nil {
		return nil, err
	} else if e.Root.Return() != parse.TYPE_SERIES {
		return nil, fmt.Errorf("egraph: requires an expression that returns a series")
	}
	autods := 1000
	if a := r.FormValue("autods"); a != "" {
		i, err := strconv.Atoi(a)
		if err != nil {
			return nil, err
		}
		autods = i
	}
	now := time.Now().UTC()
	if n := r.FormValue("now"); n != "" {
		i, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			return nil, err
		}
		now = time.Unix(i, 0).UTC()
	}
	res, _, err := e.Execute(opentsdb.NewCache(schedule.Conf.TsdbHost, schedule.Conf.ResponseLimit), t, now, autods, false, schedule.Search, schedule.Lookups)
	if err != nil {
		return nil, err
	}
	c := chart.ScatterChart{
		Title: fmt.Sprintf("%s - %s", q, now.Format(time.RFC1123)),
	}
	c.XRange.Time = true
	for ri, r := range res {
		rv := r.Value.(expr.Series)
		pts := make([]chart.EPoint, len(rv))
		idx := 0
		for k, v := range rv {
			i, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				return nil, err
			}
			//names[idx] = time.Unix(i, 0).Format("02 Jan 15:04")
			pts[idx].X = float64(i)
			pts[idx].Y = float64(v)
			idx++
		}
		slice.Sort(pts, func(i, j int) bool {
			return pts[i].X < pts[j].X
		})
		c.AddData(r.Group.String(), pts, chart.PlotStyleLinesPoints, chart.AutoStyle(ri, false))
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	white := color.RGBA{0xff, 0xff, 0xff, 0xff}
	const width = 800
	const height = 600
	s := svg.New(w)
	s.Start(width, height)
	s.Rect(0, 0, width, height, "fill: #ffffff")
	sgr := svgg.AddTo(s, 0, 0, width, height, "", 12, white)
	c.Plot(sgr)
	s.End()
	return nil, err
}

func makeChart(r opentsdb.ResponseSet) ([]*chartSeries, error) {
	var series []*chartSeries
	for _, resp := range r {
		dps := make([]datapoint, 0)
		for k, v := range resp.DPS {
			ki, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				return nil, err
			}
			dps = append(dps, datapoint{
				X: ki,
				Y: v,
			})
		}
		if len(dps) > 0 {
			sort.Sort(ByX(dps))
			name := resp.Metric
			if len(resp.Tags) > 0 {
				name += resp.Tags.String()
			}
			series = append(series, &chartSeries{
				Name: name,
				Data: dps,
			})
		}
	}
	return series, nil
}

type chartSeries struct {
	Name string      `json:"name"`
	Data []datapoint `json:"data"`
}

type datapoint struct {
	X int64          `json:"x"`
	Y opentsdb.Point `json:"y"`
}

type ByX []datapoint

func (a ByX) Len() int           { return len(a) }
func (a ByX) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByX) Less(i, j int) bool { return a[i].X < a[j].X }
