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

	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/sched"
	"bosun.org/metadata"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
	svg "github.com/ajstarks/svgo"
	"github.com/bosun-monitor/annotate"
	"github.com/bradfitz/slice"
	"github.com/gorilla/mux"
	"github.com/vdobler/chart"
	"github.com/vdobler/chart/svgg"
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
	var startT, endT time.Time
	if s, ok := oreq.Start.(string); ok && strings.Contains(s, "-ago") {
		startT, err = opentsdb.ParseTime(s)
		if err != nil {
			return nil, err
		}
		start = strings.TrimSuffix(s, "-ago")
	}
	if s, ok := oreq.End.(string); ok && strings.Contains(s, "-ago") {
		endT, err = opentsdb.ParseTime(s)
		if err != nil {
			return nil, err
		}
		end = strings.TrimSuffix(s, "-ago")
	}
	if start == "" && end == "" {
		s, sok := oreq.Start.(int64)
		e, eok := oreq.End.(int64)
		if sok && eok {
			start = fmt.Sprintf("%vs", e-s)
			startT = time.Unix(s, 0)
			endT = time.Unix(e, 0)
			if err != nil {
				return nil, err
			}
		}
	}
	if endT.Equal(time.Time{}) {
		endT = time.Now().UTC()
	}
	m_units := make(map[string]string)
	for i, q := range oreq.Queries {
		if ar[i] {

			meta, err := schedule.MetadataMetrics(q.Metric)
			if err != nil {
				return nil, err
			}
			if meta == nil {
				return nil, fmt.Errorf("no metadata for %s: cannot use auto rate", q)
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
		if !schedule.SystemConf.GetTSDBContext().Version().FilterSupport() {
			if err := schedule.Search.Expand(q); err != nil {
				return nil, err
			}
		}
	}
	var tr opentsdb.ResponseSet
	b, _ := json.MarshalIndent(oreq, "", "  ")
	t.StepCustomTiming("tsdb", "query", string(b), func() {
		h := schedule.SystemConf.GetTSDBHost()
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
	var a []annotate.Annotation
	if schedule.SystemConf.AnnotateEnabled() {
		a, err = annotateBackend.GetAnnotations(&startT, &endT)
		if err != nil {
			return nil, err
		}
	}
	return struct {
		Queries     []string
		Series      []*chartSeries
		Annotations []annotate.Annotation
	}{
		queries,
		cs,
		a,
	}, nil
}

// ExprGraph returns an svg graph.
// The basename of the requested svg file should be a base64 encoded expression.
func ExprGraph(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	bs := vars["bs"]
	format := vars["format"]
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
	e, err := expr.New(q, schedule.RuleConf.GetFuncs(schedule.SystemConf.EnabledBackends()))
	if err != nil {
		return nil, err
	} else if e.Root.Return() != models.TypeSeriesSet {
		return nil, fmt.Errorf("egraph: requires an expression that returns a series")
	}
	// it may not strictly be necessary to recreate the contexts each time, but we do to be safe
	backends := &expr.Backends{
		TSDBContext:     schedule.SystemConf.GetTSDBContext(),
		GraphiteContext: schedule.SystemConf.GetGraphiteContext(),
		InfluxConfig:    schedule.SystemConf.GetInfluxContext(),
		LogstashHosts:   schedule.SystemConf.GetLogstashContext(),
		ElasticHosts:    schedule.SystemConf.GetElasticContext(),
	}
	providers := &expr.BosunProviders{
		Cache:     cacheObj,
		Search:    schedule.Search,
		Squelched: nil,
		History:   nil,
	}
	res, _, err := e.Execute(backends, providers, t, now, autods, false)
	if err != nil {
		return nil, err
	}
	switch format {
	case "svg":
		if err := schedule.ExprSVG(t, w, 800, 600, "", res.Results); err != nil {
			return nil, err
		}
	case "png":
		if err := schedule.ExprPNG(t, w, 800, 600, "", res.Results); err != nil {
			return nil, err
		}
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
