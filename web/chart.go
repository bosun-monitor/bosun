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

func Query(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	var oreq opentsdb.Request
	err := json.Unmarshal([]byte(r.FormValue("json")), &oreq)
	if err != nil {
		serveError(w, err)
		return
	}
	err = Autods(&oreq, 200)
	if err != nil {
		serveError(w, err)
		return
	}
	for _, q := range oreq.Queries {
		if err := expr.ExpandSearch(q); err != nil {
			serveError(w, err)
			return
		}
	}
	var tr opentsdb.ResponseSet
	q, _ := url.QueryUnescape(oreq.String())
	t.StepCustomTiming("tsdb", "query", q, func() {
		tr, err = tsdbHost.Query(oreq)
	})
	if err != nil {
		serveError(w, err)
		return
	}
	qr, err := rickchart(tr)
	if err != nil {
		serveError(w, err)
		return
	}
	b, err := json.Marshal(qr)
	if err != nil {
		serveError(w, err)
		return
	}
	w.Write(b)
}

func ParseAbsTime(s string) (time.Time, error) {
	var t time.Time
	t_formats := [...]string{
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

func ParseTime(v interface{}) (t time.Time, err error) {
	switch i := v.(type) {
	case string:
		if i != "" {
			if strings.Contains(i, "-ago") {
				s := strings.Split(i, "-ago")
				now := time.Now()
				d, err := expr.ParseDuration(s[0])
				if err != nil {
					return now, err
				}
				return now.Add(-d), nil
			} else {
				a, err := ParseAbsTime(i)
				if err != nil {
					return t, err
				}
				return a, nil
			}
		}
	case int, int64:
		return time.Unix(i.(int64), 0), nil
	default:
		return t, errors.New("Type must be string, int, or int64")
	}
	return
}

func GetDuration(r *opentsdb.Request) (t time.Duration, err error) {
	switch r.Start.(type) {
	case string:
		if r.Start.(string) == "" {
			return t, errors.New("Start Time Must be Provided")
		}
	}
	start, err := ParseTime(r.Start)
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
	ds := fmt.Sprintf("%vs-avg", int64(d.Seconds()))
	fmt.Println(ds)
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
