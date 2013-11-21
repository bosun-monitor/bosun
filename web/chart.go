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

	"github.com/StackExchange/tsaf/opentsdb"
)

func Chart(w http.ResponseWriter, r *http.Request) {
	q, _ := url.Parse(TSDBHttp)
	q.Path = "/api/query"
	q.RawQuery = r.URL.RawQuery
	resp, err := http.Get(q.String())
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Fatal("bad status", err, resp.StatusCode)
	}
	b, _ := ioutil.ReadAll(resp.Body)
	var tr opentsdb.ResponseSet
	if err := json.Unmarshal(b, &tr); err != nil {
		log.Fatal("bad json", err)
	}
	qr := chart(tr)
	tqx := r.FormValue("tqx")
	qr.ReqId = strings.Split(tqx, ":")[1]

	b, _ = json.Marshal(qr)
	w.Write(b)
}

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
