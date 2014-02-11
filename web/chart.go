package web

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/scollector/opentsdb"
)

func Query(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	ts := make(opentsdb.TagSet)
	tags := strings.Split(r.FormValue("tags"), ",")
	for i := 0; i < len(tags); i += 2 {
		if i+1 < len(tags) {
			ts[tags[i]] = tags[i+1]
		}
	}
	var oro opentsdb.RateOptions
	oro.Counter = r.FormValue("counter") == "true"
	if r.FormValue("cmax") != "" {
		cmax, err := strconv.Atoi(r.FormValue("cmax"))
		if err != nil {
			serveError(w, err)
		}
		oro.CounterMax = cmax
	}
	if r.FormValue("creset") != "" {
		creset, err := strconv.Atoi(r.FormValue("creset"))
		if err != nil {
			serveError(w, err)
		}
		oro.ResetValue = creset
	}
	oq := opentsdb.Query{
		Aggregator:  r.FormValue("aggregator"),
		Metric:      r.FormValue("metric"),
		Tags:        ts,
		Rate:        r.FormValue("rate") == "true",
		Downsample:  r.FormValue("downsample"),
		RateOptions: oro,
	}

	oreq := opentsdb.Request{
		Start:   r.FormValue("start"),
		End:     r.FormValue("end"),
		Queries: []*opentsdb.Query{&oq},
	}
	tr, err := oreq.Query(tsdbHost)
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

func Chart(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	q := &url.URL{
		Scheme:   "http",
		Host:     tsdbHost,
		Path:     "/api/query",
		RawQuery: r.URL.RawQuery,
	}
	resp, err := http.Get(q.String())
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Println("bad status", err, resp.StatusCode)
		return
	}
	b, _ := ioutil.ReadAll(resp.Body)
	var tr opentsdb.ResponseSet
	if err := json.Unmarshal(b, &tr); err != nil {
		log.Println("bad json", err)
		return
	}
	qr := chart(tr)
	tqx := r.FormValue("tqx")
	qr.ReqId = strings.Split(tqx, ":")[1]
	b, _ = json.Marshal(qr)
	w.Write(b)
}

func rickchart(r opentsdb.ResponseSet) ([]*RickSeries, error) {
	//This currently does a mod operation to limit DPs returned to 3000, will want to refactor this
	//into something smarter
	max_dp := 3000
	series := make([]*RickSeries, len(r))
	for i, resp := range r {
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
		for k, v := range resp.Tags {
			id = append(id, fmt.Sprintf("%v=%v", k, v))
		}
		series[i] = &RickSeries{
			Name: strings.Join(id, ","),
			Data: dps,
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

func chart(r opentsdb.ResponseSet) *QueryResponse {
	cols := make([]Col, 1+len(r))
	cols[0].Id = "date"
	cols[0].Type = "datetime"
	rowkeys := make(map[string]bool)
	for i, resp := range r {
		for k := range resp.DPS {
			rowkeys[k] = true
		}
		id := ""
		for k, v := range resp.Tags {
			id += fmt.Sprintf("%v:%v ", k, v)
		}
		cols[i+1] = Col{
			Label: id,
			Type:  "number",
		}
	}
	rowstrs := make([]string, len(rowkeys))
	i := 0
	for k := range rowkeys {
		rowstrs[i] = k
		i++
	}
	sort.Strings(rowstrs)
	rows := make([]Row, len(rowkeys))
	prev := make(map[int]interface{})
	for i, k := range rowstrs {
		row := &rows[i]
		row.Cells = make([]Cell, len(cols))
		row.Cells[0].Value = toJsonDate(k)
		for j, resp := range r {
			if v, ok := resp.DPS[k]; ok {
				row.Cells[j+1].Value = v
				prev[j] = v
			} else {
				row.Cells[j+1].Value = prev[j]
			}
		}
	}

	dt := DataTable{
		Cols: cols,
		Rows: rows,
	}

	qr := QueryResponse{
		Status:  "ok",
		Version: "0.6",
		Table:   dt,
	}
	return &qr
}

func toJsonDate(d string) string {
	var i int64
	var err error
	if i, err = strconv.ParseInt(d, 10, 64); err != nil {
		return d
	}
	t := time.Unix(i, 0)
	return fmt.Sprintf("Date(%d, %d, %d, %d, %d, %d)",
		t.Year(),
		t.Month()-1,
		t.Day(),
		t.Hour(),
		t.Minute(),
		t.Second(),
	)
}

type QueryResponse struct {
	ReqId   string    `json:"reqId"`
	Status  string    `json:"status"`
	Version string    `json:"version"`
	Table   DataTable `json:"table"`
}

type DataTable struct {
	Cols []Col       `json:"cols"`
	Rows []Row       `json:"rows"`
	P    interface{} `json:"p,omitempty"`
}

type Col struct {
	Id    string      `json:"id,omitempty"`
	Label string      `json:"label,omitempty"`
	Type  string      `json:"type"`
	P     interface{} `json:"p,omitempty"`
}

type Row struct {
	Cells []Cell `json:"c"`
}

type Cell struct {
	Value  interface{} `json:"v,omitempty"`
	Format string      `json:"f,omitempty"`
	P      interface{} `json:"p,omitempty"`
}
