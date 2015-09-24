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

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	svg "bosun.org/_third_party/github.com/ajstarks/svgo"
	"bosun.org/_third_party/github.com/bradfitz/slice"
	"bosun.org/_third_party/github.com/gorilla/mux"
	"bosun.org/_third_party/github.com/vdobler/chart"
	"bosun.org/_third_party/github.com/vdobler/chart/svgg"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/cmd/bosun/sched"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
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
	if ads_v := r.FormValue("autods"); ads_v != "" {
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
	if start == "" && end == "" {
		s, sok := oreq.Start.(int64)
		e, eok := oreq.End.(int64)
		if sok && eok {
			start = fmt.Sprintf("%vs", e-s)
		}
	}
	m_units := make(map[string]string)
	for i, q := range oreq.Queries {
		if ar[i] {

			meta, err := schedule.MetadataMetrics(q.Metric)
			if err != nil {
				return nil, err
			}
			if meta.Unit != "" {
				m_units[q.Metric] = meta.Unit
			}
			if meta.Rate != "" {
				switch meta.Rate {
				case metadata.Gauge:
					// ignore
				case metadata.Rate:
					q.Rate = true
				case metadata.Counter:
					q.Rate = true
					q.RateOptions = opentsdb.RateOptions{
						Counter:    true,
						ResetValue: 1,
					}
				default:
					return nil, fmt.Errorf("unknown metadata rate: %s", meta.Rate)
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
		h := schedule.Conf.TSDBHost
		if h == "" {
			err = fmt.Errorf("tsdbHost not set")
			return
		}
		tr, err = oreq.Query(h)
	})
	if err != nil {
		return nil, err
	}
	cs, err := makeChart(tr, m_units)
	if err != nil {
		return nil, err
	}
	if _, present := r.Form["png"]; present {
		c := chart.ScatterChart{
			Title: fmt.Sprintf("%v - %v", oreq.Start, queries),
		}
		c.XRange.Time = true
		if min, err := strconv.ParseFloat(r.FormValue("min"), 64); err == nil {
			c.YRange.MinMode.Fixed = true
			c.YRange.MinMode.Value = min
		}
		if max, err := strconv.ParseFloat(r.FormValue("max"), 64); err == nil {
			c.YRange.MaxMode.Fixed = true
			c.YRange.MaxMode.Value = max
		}
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
	return struct {
		Queries []string
		Series  []*chartSeries
	}{
		queries,
		cs,
	}, nil
}

// ExprGraph returns an svg graph.
// The basename of the requested svg file should be a base64 encoded expression.
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
	e, err := expr.New(q, schedule.Conf.Funcs())
	if err != nil {
		return nil, err
	} else if e.Root.Return() != parse.TypeSeriesSet {
		return nil, fmt.Errorf("egraph: requires an expression that returns a series")
	}
	// it may not strictly be necessary to recreate the contexts each time, but we do to be safe
	tsdbContext := schedule.Conf.TSDBContext()
	graphiteContext := schedule.Conf.GraphiteContext()
	ls := schedule.Conf.LogstashElasticHosts
	influx := schedule.Conf.InfluxConfig
	res, _, err := e.Execute(tsdbContext, graphiteContext, ls, influx, cacheObj, t, now, autods, false, schedule.Search, nil, nil)
	if err != nil {
		return nil, err
	}
	if err := schedule.ExprSVG(t, w, 800, 600, "", res.Results); err != nil {
		return nil, err
	}
	return nil, nil
}

func makeChart(r opentsdb.ResponseSet, m_units map[string]string) ([]*chartSeries, error) {
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
				Name:   name,
				Metric: resp.Metric,
				Tags:   resp.Tags,
				Data:   dps,
				Unit:   m_units[resp.Metric],
			})
		}
	}
	return series, nil
}

type chartSeries struct {
	Name   string
	Metric string
	Tags   opentsdb.TagSet
	Data   [][2]float64
	Unit   string
}
