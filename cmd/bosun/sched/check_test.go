package sched

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/opentsdb"
)

func TestCheckFlapping(t *testing.T) {

	c, err := conf.New("", `
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
	ak := expr.NewAlertKey("a", nil)
	r := &RunHistory{
		Events: map[expr.AlertKey]*Event{
			ak: {Status: StWarning},
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
	s.RunHistory(r)
	if !hasNots() {
		t.Fatalf("expected notification: %v", s.pendingNotifications)
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
	s, err := initSched(c)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AddSilence(time.Now().Add(-time.Hour), time.Now().Add(time.Hour), "a", "", false, true, "", "user", "message")
	if err != nil {
		t.Fatal(err)
	}
	check(s, time.Now())
	s.CheckNotifications()
	select {
	case <-done:
		t.Fatal("silenced notification was sent")
	case <-time.After(time.Second * 2):
		// Timeout *probably* means the silence worked
	}
}

func TestIncidentIds(t *testing.T) {
	c, err := conf.New("", `
		alert a {
			crit = 1
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := initSched(c)
	ak := expr.NewAlertKey("a", nil)
	r := &RunHistory{
		Events: map[expr.AlertKey]*Event{
			ak: {Status: StWarning},
		},
	}
	expect := func(id uint64) {
		if s.status[ak].Last().IncidentId != id {
			t.Fatalf("Expeted incident id %d. Got %d.", id, s.status[ak].Last().IncidentId)
		}
	}
	s.RunHistory(r)
	expect(1)

	r.Events[ak].Status = StNormal
	r.Events[ak].IncidentId = 0
	s.RunHistory(r)
	expect(1)

	r.Events[ak].Status = StWarning
	r.Events[ak].IncidentId = 0
	s.RunHistory(r)
	expect(1)

	r.Events[ak].Status = StNormal
	r.Events[ak].IncidentId = 0
	s.RunHistory(r)
	err = s.Action("", "", ActionClose, ak)
	if err != nil {
		t.Fatal(err)
	}
	r.Events[ak].Status = StWarning
	r.Events[ak].IncidentId = 0
	s.RunHistory(r)
	expect(2)
}

func TestCheckNotify(t *testing.T) {
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
	c, err := conf.New("", fmt.Sprintf(`
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
	check(s, time.Now())
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
	c, err := conf.New("", fmt.Sprintf(`
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
		Events: map[expr.AlertKey]*Event{
			expr.NewAlertKey("a", opentsdb.TagSet{"h": "x"}): {Status: StUnknown},
			expr.NewAlertKey("a", opentsdb.TagSet{"h": "y"}): {Status: StUnknown},
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
	c, err := conf.New("", fmt.Sprintf(`
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
		Events: map[expr.AlertKey]*Event{
			expr.NewAlertKey("a", opentsdb.TagSet{"h": "x"}): {Status: StUnknown},
			expr.NewAlertKey("a", opentsdb.TagSet{"h": "y"}): {Status: StUnknown},
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
	c, err := conf.New("", fmt.Sprintf(`
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
	check(s, time.Now())
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
	for ak, st := range s.status {
		switch ak {
		case "a{}":
			if !st.Open {
				t.Errorf("expected a to be open")
			}
		case "b{}":
			if st.Open {
				t.Errorf("expected b to be closed")
			}
		default:
			t.Errorf("unexpected alert key %s", ak)
		}
	}
}

// TestCheckCritUnknownEmpty checks that if an alert goes normal -> crit ->
// unknown, it's body and subject are empty. This is because we should not
// keep around the crit template renders if we are unknown.
func TestCheckCritUnknownEmpty(t *testing.T) {
	c, err := conf.New("", `
		template t {
			subject = 1
			body = 2
		}
		alert a {
			crit = 1
			template = t
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := initSched(c)
	ak := expr.NewAlertKey("a", nil)
	r := &RunHistory{
		Events: map[expr.AlertKey]*Event{
			ak: {Status: StNormal},
		},
	}
	verify := func(empty bool) {
		st := s.GetStatus(ak)
		if empty {
			if st.Body != "" || st.Subject != "" {
				t.Fatalf("expected empty body and subject")
			}
		} else {
			if st.Body != "<html><head></head><body>2</body></html>" || st.Subject != "1" {
				t.Fatalf("expected body and subject")
			}
		}
	}
	s.RunHistory(r)
	verify(true)
	r.Events[ak].Status = StCritical
	s.RunHistory(r)
	verify(false)
	r.Events[ak].Status = StUnknown
	s.RunHistory(r)
	verify(true)
	r.Events[ak].Status = StNormal
	s.RunHistory(r)
	verify(true)
}
