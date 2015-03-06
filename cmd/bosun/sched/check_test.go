package sched

import (
	"testing"

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
			ak: &Event{Status: StWarning},
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
