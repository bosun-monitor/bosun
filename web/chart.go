package web

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/color"
	"net/http"
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
	"github.com/StackExchange/bosun/sched"
)

// Graph takes an OpenTSDB request data structure and queries OpenTSDB. Use the
// json parameter to pass JSON. Use the b64 parameter to pass base64-encoded
// JSON.
func Graph(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	j := []byte(r.FormValue("json"))
	if bs := r.FormValue("b64"); bs != "" {
		b, err := base64.StdEncoding.DecodeString(bs)
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
	queries := make([]string, len(oreq.Queries))
	var start, end string
	if s, ok := oreq.Start.(string); ok && strings.Contains(s, "-ago") {
		start = strings.TrimSuffix(s, "-ago")
	}
	if s, ok := oreq.End.(string); ok && strings.Contains(s, "-ago") {
		end = strings.TrimSuffix(s, "-ago")
	}
	var metrics []string
	for i, q := range oreq.Queries {
		if ar[i] {
			ms := schedule.GetMetadata(q.Metric, nil)
			metrics = append(metrics, q.Metric)
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
		queries[i] = fmt.Sprintf(`q("%v", "%v", "%v")`, q, start, end)
		if err := schedule.Search.Expand(q); err != nil {
			return nil, err
		}
	}
	var tr opentsdb.ResponseSet
	b, _ := json.MarshalIndent(oreq, "", "  ")
	t.StepCustomTiming("tsdb", "query", string(b), func() {
		tr, err = oreq.Query(schedule.Conf.TsdbHost)
	})
	if err != nil {
		return nil, err
	}
	cs, err := makeChart(tr)
	if err != nil {
		return nil, err
	}
	if _, present := r.Form["png"]; present {
		c := chart.ScatterChart{
			Title: fmt.Sprintf("%v - %v", oreq.Start, oreq.End),
		}
		c.XRange.Time = true
		for ri, r := range cs {
			pts := make([]chart.EPoint, len(r.Data))
			for idx, v := range r.Data {
				pts[idx].X = v[0]
				pts[idx].Y = v[1]
			}
			slice.Sort(pts, func(i, j int) bool {
				return pts[i].X < pts[j].X
			})
			c.AddData(r.Name, pts, chart.PlotStyleLinesPoints, sched.Autostyle(ri))
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
		return nil, nil
	}
	var y_labels []string
	for _, m := range metrics {
		ms := schedule.GetMetadata(m, nil)
		for _, md := range ms {
			if md.Name == "unit" {
				if v, ok := md.Value.(string); ok {
					y_labels = append(y_labels, v)
				}
			}
		}
	}
	return struct {
		Queries []string
		Series  []*chartSeries
		YLabels []string
	}{
		queries,
		cs,
		y_labels,
	}, nil
}

func ExprGraph(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	bs := vars["bs"]
	b, err := base64.StdEncoding.DecodeString(bs)
	if err != nil {
		return nil, err
	}
	q := string(b)
	if len(q) == 0 {
		return nil, fmt.Errorf("missing expression")
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
	e, err := expr.New(q)
	if err != nil {
		return nil, err
	} else if e.Root.Return() != parse.TYPE_SERIES {
		return nil, fmt.Errorf("egraph: requires an expression that returns a series")
	}
	res, _, err := e.Execute(opentsdb.NewCache(schedule.Conf.TsdbHost, schedule.Conf.ResponseLimit), t, now, autods, false, schedule.Search, schedule.Lookups, nil)
	if err != nil {
		return nil, err
	}
	if err := schedule.ExprGraph(t, w, res.Results, q, now); err != nil {
		return nil, err
	}
	return nil, nil
}

func makeChart(r opentsdb.ResponseSet) ([]*chartSeries, error) {
	var series []*chartSeries
	for _, resp := range r {
		dps := make([][2]float64, 0)
		for k, v := range resp.DPS {
			ki, err := strconv.ParseInt(k, 10, 64)
			if err != nil {
				return nil, err
			}
			dps = append(dps, [2]float64{float64(ki), float64(v)})
		}
		if len(dps) > 0 {
			slice.Sort(dps, func(i, j int) bool {
				return dps[i][0] < dps[j][0]
			})
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
	Name string
	Data [][2]float64
}
