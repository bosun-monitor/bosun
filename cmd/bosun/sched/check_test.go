package sched

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
)

func TestCheckFlapping(t *testing.T) {
	s := new(Schedule)
	c, err := conf.New("", `
		notification n {
			print = true
		}
		alert a {
			warnNotification = n
			critNotification = n
			warn = 1
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	c.StateFile = ""
	s.Init(c)
	ak := expr.NewAlertKey("a", nil)
	r := &RunHistory{
		Events: map[expr.AlertKey]*Event{
			ak: {Status: StWarning},
		},
	}
	hasNots := func() bool {
		defer func() {
			s.notifications = nil
		}()
		if len(s.notifications) != 1 {
			return false
		}
		for k, v := range s.notifications {
			if k.Name != "n" || len(v) != 1 || v[0].Alert != "a" {
				return false
			}
			return true
		}
		return false
	}
	s.RunHistory(r)
	if !hasNots() {
		t.Fatalf("expected notification: %v", s.notifications)
	}
	r.Events[ak].Status = StNormal
	s.RunHistory(r)
	if hasNots() {
		t.Fatal("unexpected notification")
	}
	r.Events[ak].Status = StWarning
	s.RunHistory(r)
	if hasNots() {
		t.Fatal("unexpected notification")
	}
	r.Events[ak].Status = StNormal
	s.RunHistory(r)
	if hasNots() {
		t.Fatal("unexpected notification")
	}
	r.Events[ak].Status = StCritical
	s.RunHistory(r)
	if !hasNots() {
		t.Fatal("expected notification")
	}
	r.Events[ak].Status = StNormal
	s.RunHistory(r)
	if hasNots() {
		t.Fatal("unexpected notification")
	}
	s.RunHistory(r)
	// Close the alert, so it should notify next time.
	if err := s.Action("", "", ActionClose, ak); err != nil {
		t.Fatal(err)
	}
	r.Events[ak].Status = StWarning
	s.RunHistory(r)
	if !hasNots() {
		t.Fatal("expected notification")
	}
}

func TestCheckSilence(t *testing.T) {
	s := new(Schedule)
	done := make(chan bool, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		done <- true
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	c, err := conf.New("", fmt.Sprintf(`
		template t {
			subject = "test"
			body = "test"
		}
		notification n {
			post = http://%s/
		}
		alert a {
			template = t
			warnNotification = n
			warn = 1
		}
	`, u.Host))
	if err != nil {
		t.Fatal(err)
	}
	err = s.Init(c)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddSilence(time.Now().Add(-time.Hour), time.Now().Add(time.Hour), "a", "", false, true, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Check(nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s.CheckNotifications()
	select {
	case <-done:
		t.Fatal("silenced notification was sent")
	case <-time.After(time.Second * 2):
		// Timeout *probably* means the silence worked
	}
}
