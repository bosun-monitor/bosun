package web

import (
	"encoding/json"
	"fmt"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/scollector/opentsdb"
	//"github.com/StackExchange/tsaf/expr"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

func Query(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	var oreq opentsdb.Request
	err := json.Unmarshal([]byte(r.FormValue("json")), &oreq)
	if err != nil {
		serveError(w, err)
		return
	}
	log.Println(oreq)
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
		sort.Sort(ByX(dps))
		var id []string
		id = append(id, fmt.Sprintf("(%v)", resp.Metric))
		for k, v := range resp.Tags {
			id = append(id, fmt.Sprintf("%v=%v", k, v))
		}
		if len(dps) > 0 {
			series = append(series, &RickSeries{
				Name: strings.Join(id, ","),
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
