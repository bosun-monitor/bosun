package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"bosun.org/cmd/scollector/collectors"
	"bosun.org/cmd/scollector/conf"
	"github.com/BurntSushi/toml"
)

var devMode = flag.Bool("dev", false, "Dev mode. Use html from file-system instead of embedded copy.")

// to embed static again: go:generate esc -modtime 0 -o=static.go -prefix=static static

func main() {
	flag.Parse()
	fs := FS(*devMode)
	http.Handle("/", http.FileServer(fs))
	http.HandleFunc("/test", TestMib)
	http.HandleFunc("/toml", Toml)
	http.ListenAndServe(":8888", nil)
}

func TestMib(w http.ResponseWriter, r *http.Request) {
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	mib := conf.MIB{}
	err = json.Unmarshal(buf, &mib)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	snmp := conf.SNMP{}
	err = json.Unmarshal(buf, &snmp)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	md, err := collectors.GenericSnmp(snmp, mib)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 500)
		return
	}

	mdJson, err := json.MarshalIndent(md, "", "  ")
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write(mdJson)
}

func Toml(w http.ResponseWriter, r *http.Request) {
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	mib := conf.MIB{}
	err = json.Unmarshal(buf, &mib)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	meta := &struct{ Name string }{}
	err = json.Unmarshal(buf, meta)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	toToml := struct {
		MIBs map[string]conf.MIB
	}{MIBs: map[string]conf.MIB{meta.Name: mib}}

	toml.NewEncoder(w).Encode(toToml)
}
