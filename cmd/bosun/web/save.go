package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"bosun.org/cmd/bosun/conf"
	"github.com/MiniProfiler/go/miniprofiler"
)

func SaveConfig(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	data := struct {
		Config  string
		User    string // should come from auth
		Diff    string
		Message string
		Other   []string
	}{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		serveError(w, err)
		return
	}
	err := schedule.RuleConf.SaveRawText(data.Config, data.Diff, data.User, data.Message, data.Other...)
	if err != nil {
		serveError(w, err)
		return
	}
	fmt.Fprint(w, "save successful")
}

func DiffConfig(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	data := struct {
		Config  string
		User    string // should come from auth
		Message string
		Other   []string
	}{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		serveError(w, err)
		return
	}
	diff, err := schedule.RuleConf.RawDiff(data.Config)
	if err != nil {
		serveError(w, err)
		return
	}
	fmt.Fprint(w, diff)
}

func ConfigRunningHash(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	hash := schedule.RuleConf.GetHash()
	return struct {
		Hash string
	}{
		hash,
	}, nil
}

func BulkEdit(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	bulkEdit := conf.BulkEditRequest{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&bulkEdit); err != nil {
		serveError(w, err)
		return
	}
	err := schedule.RuleConf.BulkEdit(bulkEdit)
	if err != nil {
		serveError(w, err)
		return
	}
	fmt.Fprint(w, "edit successful")
}

func SaveEnabled(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.SystemConf.SaveEnabled(), nil
}
