package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
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
	for _, q := range oreq.Queries {
		if err := expr.ExpandSearch(q); err != nil {
			serveError(w, err)
			return
		}
	}
	Autods(&oreq, 200)
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

//These should be moved to the opentsdb package probably
var reltime = map[string]time.Duration{
	"s": time.Second,
	"m": time.Minute,
	"h": time.Hour,
	"d": time.Hour * 24,
	"w": time.Hour * 24 * 7,
	//Meh, a month is "30 days"
	"n": time.Hour * 24 * 7 * 30,
	"y": time.Hour * 24 * 7 * 30 * 365,
}

var reltime_m = regexp.MustCompile(`(\d+)([s,m,h,d,w,n,y])-ago`)

func ParseRelTime(s string) (time.Duration, error) {
	m := reltime_m.FindStringSubmatch(s)
	if len(m) != 3 {
		return 0, errors.New("Invalid Relative Time")
	}
	i, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return time.Duration(i) * reltime[m[2]], nil
}

func ParseAbsTime(s string) (time.Time, error) {
	var t time.Time
	t, err := time.Parse("2006/01/02-15:04:05", s)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006/01/02-15:04", s)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006/01/02-15", s)
	if err == nil {
		return t, nil
	}
	t, err = time.Parse("2006/01/02", s)
	if err == nil {
		return t, nil
	}
	var i int64
	i, err = strconv.ParseInt(s, 10, 64)
	if err != nil {
		return t, err
	}
	t = time.Unix(i, 0)
	return t, nil
}

func GetDuration(r opentsdb.Request) (time.Duration, error) {
	start := time.Now()
	end := time.Now()
	var err error
	var sd time.Duration
	var ed time.Duration
	if r.End != nil {
		re := r.End.(string)
		if re != "" {
			if strings.Contains(re, "-ago") {
				ed, err = ParseRelTime(re)
				if err != nil {
					return ed, err
				}
				end = end.Add(-ed)
			} else {
				end, err = ParseAbsTime(re)
				fmt.Println(end)
				if err != nil {
					return ed, err
				}
			}
		}
	}
	if r.Start != nil {
		rs := r.Start.(string)
		if rs == "" {
			return sd, errors.New("Start Time Must be Provided")
		}
		if strings.Contains(rs, "-ago") {
			sd, err = ParseRelTime(rs)
			if err != nil {
				return sd, err
			}
			start = start.Add(-sd)
		} else {
			start, err = ParseAbsTime(rs)
			if err != nil {
				return sd, err
			}
		}
	}
	return end.Sub(start), nil
}

func Autods(r *opentsdb.Request, p int64) error {
	d, err := GetDuration(*r)
	if err != nil {
		return err
	}
	fmt.Println(int64(d / time.Second))
	return nil

}

func rickchart(r opentsdb.ResponseSet) ([]*RickSeries, error) {
	//This currently does a mod operation to limit DPs returned to 3000, will want to refactor this
	//into something smarter
	max_dp := 3000
	var series []*RickSeries
	for _, resp := range r {
		dps_mod := 1
		if len(resp.DPS) > max_dp {
			dps_mod = (len(resp.DPS) + max_dp) / max_dp
		}
		dps := make([]RickDP, 0)
		j := 0
		for k, v := range resp.DPS {
			if j%dps_mod == 0 {
				ki, err := strconv.ParseInt(k, 10, 64)
				if err != nil {
					return nil, err
				}
				dps = append(dps, RickDP{
					X: ki,
					Y: v,
				})
			}
			j += 1
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
