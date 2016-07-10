package web

import (
	"testing"
	"time"

	"bosun.org/cmd/bosun/conf/native"
)

func TestErrorTemplate(t *testing.T) {
	c, err := native.NewNativeConf("", `
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
