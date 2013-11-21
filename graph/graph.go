package main

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	tsdb "github.com/StackExchange/tsaf/opentsdb"
)

var templates *template.Template

func init() {
	var err error

	templates, err = template.New("").ParseFiles(
		"base.html",
	)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	http.HandleFunc("/", base)
	http.HandleFunc("/chart", chart)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func base(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "base.html", nil)
}

func chart(w http.ResponseWriter, r *http.Request) {
	q := "http://ny-devtsdb02.ds.stackexchange.com:4242/api/query?start=30s-ago&m=sum:cpu.test{host=*}"
	//q := "http://localhost:4242/api/query?start=30s-ago&m=sum:cpu.test{host=*}"
	resp, err := http.Get(q)
	if err != nil || resp.StatusCode != http.StatusOK {
		log.Fatal("bad status", err, resp.StatusCode)
	}
	b, _ := ioutil.ReadAll(resp.Body)
	var tr tsdb.ResponseSet
	if err := json.Unmarshal(b, &tr); err != nil {
		log.Fatal("bad json", err)
	}
	qr := Chart(tr)
	tqx := r.FormValue("tqx")
	qr.ReqId = strings.Split(tqx, ":")[1]

	b, _ = json.Marshal(qr)
	w.Write(b)
}
