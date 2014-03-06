package web

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// Graph takes an OpenTSDB request data structure and queries OpenTSDB. Use the
// json parameter to pass JSON. Use the b64 parameter to pass base64-encoded
// JSON.
func Graph(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var oreq opentsdb.Request
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
	err := json.Unmarshal(j, &oreq)
	if err != nil {
		return nil, err
	}
	ads_v := r.FormValue("autods")
	if ads_v != "" {
		ads_i, err := strconv.ParseInt(ads_v, 10, 64)
		if err != nil {
			return nil, err
		}
		if err := Autods(&oreq, ads_i); err != nil {
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

func ParseAbsTime(s string) (time.Time, error) {
	var t time.Time
	t_formats := [4]string{
		"2006/01/02-15:04:05",
		"2006/01/02-15:04",
		"2006/01/02-15",
		"2006/01/02",
	}
	for _, f := range t_formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return t, err
	}
	return time.Unix(i, 0), nil
}

func ParseTime(v interface{}) (time.Time, error) {
	now := time.Now().UTC()
	switch i := v.(type) {
	case string:
		if i != "" {
			if strings.HasSuffix(i, "-ago") {
				s := strings.TrimSuffix(i, "-ago")
				d, err := expr.ParseDuration(s)
				if err != nil {
					return now, err
				}
				return now.Add(-d), nil
			} else {
				return ParseAbsTime(i)
			}
		} else {
			return now, nil
		}
	case int:
		return time.Unix(int64(i), 0), nil
	case int64:
		return time.Unix(i, 0), nil
	default:
		return time.Time{}, errors.New("type must be string, int, or int64")
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
	var end time.Time
	if r.End != nil {
		end, err = ParseTime(r.End)
		if err != nil {
			return t, err
		}
	} else {
		end = time.Now()
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
