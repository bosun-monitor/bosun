package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/expr"
)

func Query(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var oreq opentsdb.Request
	err := json.Unmarshal([]byte(r.FormValue("json")), &oreq)
	if err != nil {
		return nil, err
	}
	ads_v := r.FormValue("autods")
	if ads_v != "" {
		ads_i, err := strconv.ParseInt(ads_v, 10, 64)
		if err != nil {
			serveError(w, err)
			return
		}
		err = Autods(&oreq, ads_i)
	}
	if err != nil {
		serveError(w, err)
		return
	}
	for _, q := range oreq.Queries {
		if err := expr.ExpandSearch(q); err != nil {
			return nil, err
		}
	}
	var tr opentsdb.ResponseSet
	q, _ := url.QueryUnescape(oreq.String())
	t.StepCustomTiming("tsdb", "query", q, func() {
		tr, err = tsdbHost.Query(oreq)
	})
	if err != nil {
		return nil, err
	}
	qr, err := rickchart(tr)
	if err != nil {
		return nil, err
	}
	return qr, nil
}

func ParseAbsTime(s string) (time.Time, error) {
	var t time.Time
	t_formats := [4]string{
		"2006/01/02-15:04:05",
		"2006/01/02-15:04",
		"2006/01/02-15",
		"2006/01/02",
	}
	for _, f := range t_formats {
		t, err := time.Parse(f, s)
		if err == nil {
			return t, nil
		}
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return t, err
	}
	t = time.Unix(i, 0)
	return t, nil
}

func ParseTime(v interface{}) (time.Time, error) {
	switch i := v.(type) {
	case string:
		if i != "" {
			if strings.HasSuffix(i, "-ago") {
				s := strings.TrimSuffix(i, "-ago")
				d, err := expr.ParseDuration(s)
				if err != nil {
					return time.Now().UTC(), err
				}
				return time.Now().UTC().Add(-d), nil
			} else {
				a, err := ParseAbsTime(i)
				if err != nil {
					return time.Now().UTC(), err
				}
				return a, nil
			}
		} else {
			return time.Now().UTC(), nil
		}
	case int:
		return time.Unix(int64(i), 0), nil
	case int64:
		return time.Unix(i, 0), nil
	default:
		return time.Now().UTC(), errors.New("type must be string, int, or int64")
	}
}

func GetDuration(r *opentsdb.Request) (time.Duration, error) {
	var t time.Duration
	if v, ok := r.Start.(string); ok && v == "" {
		return t, errors.New("start time must be provided")
	}
	start, err := ParseTime(r.Start)
	if err != nil {
		return t, err
	}
	end := time.Now()
	if r.End != nil {
		end, err = ParseTime(r.End)
		if err != nil {
			return t, err
		}
	}
	return end.Sub(start), nil
}

func Autods(r *opentsdb.Request, l int64) error {
	if l == 0 {
		return errors.New("tsaf: target length must be > 0")
	}
	cd, err := GetDuration(r)
	if err != nil {
		return err
	}
	d := cd / time.Duration(l)
	if d < time.Second*15 {
		return nil
	}
	ds := fmt.Sprintf("%ds-avg", d/time.Second)
	for _, q := range r.Queries {
		q.Downsample = ds
	}
	return nil
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
