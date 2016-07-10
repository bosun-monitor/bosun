package sched

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"bosun.org/cmd/bosun/conf/native"
	"bosun.org/models"
	"bosun.org/opentsdb"
)

func TestCheckFlapping(t *testing.T) {
	defer setup()()
	c, err := native.NewNativeConf("", `
		template t {
			subject = 1
		}
		notification n {
			print = true
		}
		alert a {
			warnNotification = n
			warn = 1
			critNotification = n
			crit = 1
			template = t
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := initSched(c)
	ak := models.NewAlertKey("a", nil)
	r := &RunHistory{
		Events: map[models.AlertKey]*models.Event{
			ak: {Status: models.StWarning},
		},
	}
	hasNots := func() bool {
		defer func() {
			s.pendingNotifications = nil
		}()
		if len(s.pendingNotifications) != 1 {
			return false
		}
		for k, v := range s.pendingNotifications {
			if k.Name != "n" || len(v) != 1 || v[0].Alert != "a" {
				return false
			}
			return true
		}
		return false
	}

	type stateTransition struct {
		S          models.Status
		ExpectNots bool
	}
	transitions := []stateTransition{
		{models.StWarning, true},
		{models.StNormal, false},
		{models.StWarning, false},
		{models.StNormal, false},
		{models.StCritical, true},
		{models.StWarning, false},
		{models.StCritical, false},
	}

	for i, trans := range transitions {
		r.Events[ak].Status = trans.S
		s.RunHistory(r)
		has := hasNots()
		if has && !trans.ExpectNots {
			t.Fatalf("unexpected notifications for transition %d.", i)
		} else if !has && trans.ExpectNots {
			t.Fatalf("expected notifications for transition %d.", i)
		}
	}
	r.Events[ak].Status = models.StNormal
	s.RunHistory(r)
	// Close the alert, so it should notify next time.
	if err := s.ActionByAlertKey("", "", models.ActionClose, ak); err != nil {
		t.Fatal(err)
	}
	r.Events[ak].Status = models.StWarning
	s.RunHistory(r)
	if !hasNots() {
		t.Fatal("expected notification")
	}
}

func TestCheckSilence(t *testing.T) {
	defer setup()()
	done := make(chan bool, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		done <- true
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	c, err := native.NewNativeConf("", fmt.Sprintf(`
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
	s, err := initSched(c)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddSilence(utcNow().Add(-time.Hour), utcNow().Add(time.Hour), "a", "", false, true, "", "user", "message")
	if err != nil {
		t.Fatal(err)
	}
	check(s, utcNow())
	s.CheckNotifications()
	select {
	case <-done:
		t.Fatal("silenced notification was sent")
	case <-time.After(time.Second * 2):
		// Timeout *probably* means the silence worked
	}
}

func TestIncidentIds(t *testing.T) {
	defer setup()()
	c, err := native.NewNativeConf("", `
		alert a {
			crit = 1
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := initSched(c)
	ak := models.NewAlertKey("a", nil)
	r := &RunHistory{
		Events: map[models.AlertKey]*models.Event{
			ak: {Status: models.StWarning},
		},
	}
	expect := func(id int64) {
		incident, err := s.DataAccess.State().GetLatestIncident(ak)
		if err != nil {
			t.Fatal(err)
		}
		if incident.Id != id {
			t.Fatalf("Expeted incident id %d. Got %d.", id, incident.Id)
		}
	}
	s.RunHistory(r)
	expect(1)

	r.Events[ak].Status = models.StNormal
	s.RunHistory(r)
	expect(1)

	r.Events[ak].Status = models.StWarning
	s.RunHistory(r)
	expect(1)

	r.Events[ak].Status = models.StNormal
	s.RunHistory(r)
	err = s.ActionByAlertKey("", "", models.ActionClose, ak)
	if err != nil {
		t.Fatal(err)
	}
	r.Events[ak].Status = models.StWarning
	s.RunHistory(r)
	expect(2)
}

func TestCheckNotify(t *testing.T) {
	defer setup()()
	nc := make(chan string)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		nc <- string(b)
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	c, err := native.NewNativeConf("", fmt.Sprintf(`
		template t {
			subject = {{.Last.Status}}
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
	s, err := initSched(c)
	if err != nil {
		t.Fatal(err)
	}
	check(s, utcNow())
	s.CheckNotifications()
	select {
	case r := <-nc:
		if r != "warning" {
			t.Fatalf("expected warning, got %v", r)
		}
	case <-time.After(time.Second):
		t.Fatal("failed to receive notification before timeout")
	}
}

func TestCheckNotifyUnknown(t *testing.T) {
	defer setup()()
	nc := make(chan string, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		nc <- string(b)
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	c, err := native.NewNativeConf("", fmt.Sprintf(`
		minGroupSize = 2
		template t {
			subject = {{.Name}}: {{.Group | len}} unknown alerts
		}
		unknownTemplate = t
		notification n {
			post = http://%s/
		}
		alert a {
			template = t
			critNotification = n
			crit = 1
		}
	`, u.Host))
	if err != nil {
		t.Fatal(err)
	}
	s, err := initSched(c)
	if err != nil {
		t.Fatal(err)
	}
	r := &RunHistory{
		Events: map[models.AlertKey]*models.Event{
			models.NewAlertKey("a", opentsdb.TagSet{"h": "x"}): {Status: models.StUnknown},
			models.NewAlertKey("a", opentsdb.TagSet{"h": "y"}): {Status: models.StUnknown},
		},
	}
	s.RunHistory(r)
	s.CheckNotifications()
	s.sendUnknownNotifications()
	gotExpected := false
Loop:
	for {
		select {
		case r := <-nc:
			if r == "a: 2 unknown alerts" {
				gotExpected = true
			} else {
				t.Fatalf("unexpected: %v", r)
			}
		// TODO: remove this silly timeout-based test
		case <-time.After(time.Second):
			break Loop
		}
	}
	if !gotExpected {
		t.Errorf("didn't get expected result")
	}
}

// TestCheckNotifyUnknownDefault tests the default unknownTemplate.
func TestCheckNotifyUnknownDefault(t *testing.T) {
	defer setup()()
	nc := make(chan string, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		nc <- string(b)
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	c, err := native.NewNativeConf("", fmt.Sprintf(`
		minGroupSize = 2
		template t {
			subject = template
		}
		notification n {
			post = http://%s/
		}
		alert a {
			template = t
			critNotification = n
			crit = 1
		}
	`, u.Host))
	if err != nil {
		t.Fatal(err)
	}
	s, err := initSched(c)
	if err != nil {
		t.Fatal(err)
	}
	r := &RunHistory{
		Events: map[models.AlertKey]*models.Event{
			models.NewAlertKey("a", opentsdb.TagSet{"h": "x"}): {Status: models.StUnknown},
			models.NewAlertKey("a", opentsdb.TagSet{"h": "y"}): {Status: models.StUnknown},
		},
	}
	s.RunHistory(r)
	s.CheckNotifications()
	s.sendUnknownNotifications()
	gotExpected := false
Loop:
	for {
		select {
		case r := <-nc:
			if r == "a: 2 unknown alerts" {
				gotExpected = true
			} else {
				t.Fatalf("unexpected: %v", r)
			}
		// TODO: remove this silly timeout-based test
		case <-time.After(time.Second):
			break Loop
		}
	}
	if !gotExpected {
		t.Errorf("didn't get expected result")
	}
}

func TestCheckNotifyLog(t *testing.T) {
	defer setup()()
	nc := make(chan string, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := ioutil.ReadAll(r.Body)
		nc <- string(b)
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	c, err := native.NewNativeConf("", fmt.Sprintf(`
		template t {
			subject = {{.Alert.Name}}
		}
		notification n {
			post = http://%s/
		}
		alert a {
			template = t
			critNotification = n
			crit = 1
		}
		alert b {
			template = t
			critNotification = n
			crit = 1
			log = true
		}
	`, u.Host))
	if err != nil {
		t.Fatal(err)
	}
	s, err := initSched(c)
	if err != nil {
		t.Fatal(err)
	}
	check(s, utcNow())
	s.CheckNotifications()
	gotA := false
	gotB := false
Loop:
	for {
		select {
		case r := <-nc:
			if r == "a" && !gotA {
				gotA = true
			} else if r == "b" && !gotB {
				gotB = true
			} else {
				t.Errorf("unexpected: %v", r)
			}
		// TODO: remove this silly timeout-based test
		case <-time.After(time.Second):
			break Loop
		}
	}
	if !gotA {
		t.Errorf("didn't get expected a")
	}
	if !gotB {
		t.Errorf("didn't get expected b")
	}
	status, err := s.DataAccess.State().GetAllOpenIncidents()
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range status {
		switch st.AlertKey {
		case "a{}":
			if !st.Open {
				t.Errorf("expected a to be open")
			}
		case "b{}":
			if st.Open {
				t.Errorf("expected b to be closed")
			}
		default:
			t.Errorf("unexpected alert key %s", st.AlertKey)
		}
	}
}
