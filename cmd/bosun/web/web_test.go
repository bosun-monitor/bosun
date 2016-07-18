package web

import (
	"testing"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule"
)

func TestErrorTemplate(t *testing.T) {
	c, err := rule.NewConf("", conf.EnabledBackends{}, `
		template t {
			body = {{.Eval "invalid"}}
		}
		alert a {
			template = t
			crit = 1
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = procRule(nil, c, c.Alerts["a"], time.Time{}, false, "", "")
	if err != nil {
		t.Fatal(err)
	}
}
