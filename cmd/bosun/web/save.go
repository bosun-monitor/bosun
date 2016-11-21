package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"bosun.org/cmd/bosun/conf"
	"github.com/MiniProfiler/go/miniprofiler"
)

func SaveConfig(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	data := struct {
		Config  string
		Diff    string
		Message string
		Other   []string
	}{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	err := schedule.RuleConf.SaveRawText(data.Config, data.Diff, getUsername(r), data.Message, data.Other...)
	if err != nil {
		return nil, err
	}
	fmt.Fprint(w, "save successful")
	return nil, nil
}

func DiffConfig(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	data := struct {
		Config  string
		Message string
		User    string
		Other   []string
	}{}
	data.User = getUsername(r)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	diff, err := schedule.RuleConf.RawDiff(data.Config)
	if err != nil {
		return nil, err
	}
	fmt.Fprint(w, diff)
	return nil, nil
}

func ConfigRunningHash(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	hash := schedule.RuleConf.GetHash()
	return struct {
		Hash string
	}{
		hash,
	}, nil
}

func BulkEdit(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	bulkEdit := conf.BulkEditRequest{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&bulkEdit); err != nil {
		return nil, err
	}
	err := schedule.RuleConf.BulkEdit(bulkEdit)
	if err != nil {
		return nil, err
	}
	fmt.Fprint(w, "edit successful")
	return nil, nil
}

func SaveEnabled(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.SystemConf.SaveEnabled(), nil
}
