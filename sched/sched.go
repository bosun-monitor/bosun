package sched

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/expr"
)

type Schedule struct {
	sync.Mutex

	Conf          *conf.Conf
	Status        map[AlertKey]*State
	Notifications map[AlertKey]map[string]time.Time
	Silence       []*Silence

	cache     *opentsdb.Cache
	runStates map[AlertKey]Status
	nc        chan interface{}
}

type Silence struct {
	Start, End time.Time
	Text       string
	match      map[string]*regexp.Regexp
}

func (s *Silence) Silenced(tags opentsdb.TagSet) bool {
	now := time.Now()
	if now.Before(s.Start) || now.After(s.End) {
		return false
	}
	for k, re := range s.match {
		tagv, ok := tags[k]
		if !ok || !re.MatchString(tagv) {
			return false
		}
	}
	return true
}

// Silenced returns all currently silenced AlertKeys and the time they will
// be unsilenced.
func (s *Schedule) Silenced() map[AlertKey]time.Time {
	aks := make(map[AlertKey]time.Time)
	for _, si := range s.Silence {
		for ak, st := range s.Status {
			if si.Silenced(st.Group) {
				if aks[ak].Before(si.End) {
					aks[ak] = si.End
				}
				break
			}
		}
	}
	return aks
}

func (s *Schedule) AddSilence(start, end time.Time, text string, confirm bool) ([]AlertKey, error) {
	if start.IsZero() || end.IsZero() {
		return nil, fmt.Errorf("both start and end must be specified")
	}
	if start.After(end) {
		return nil, fmt.Errorf("start time must be before end time")
	}
	tags, err := opentsdb.ParseTags(text)
	if err != nil {
		return nil, err
	} else if len(tags) == 0 {
		return nil, fmt.Errorf("empty text")
	}
	si := Silence{
		Start: start,
		End:   end,
		Text:  text,
		match: make(map[string]*regexp.Regexp),
	}
	for k, v := range tags {
		re, err := regexp.Compile(v)
		if err != nil {
			return nil, err
		}
		si.match[k] = re
	}
	if confirm {
		s.Lock()
		s.Silence = append(s.Silence, &si)
		s.Unlock()
		return nil, nil
	}
	var aks []AlertKey
	for ak, st := range s.Status {
		if si.Silenced(st.Group) {
			aks = append(aks, ak)
			break
		}
	}
	return aks, nil
}

func (s *Schedule) MarshalJSON() ([]byte, error) {
	t := struct {
		Alerts map[string]*conf.Alert
		Status map[string]*State
	}{
		s.Conf.Alerts,
		make(map[string]*State),
	}
	for k, v := range s.Status {
		if v.Last().Status < stWarning {
			continue
		}
		t.Status[k.String()] = v
	}
	return json.Marshal(&t)
}

var DefaultSched = &Schedule{}

// Loads a configuration into the default schedule
func Load(c *conf.Conf) {
	DefaultSched.Load(c)
}

// Runs the default schedule.
func Run() error {
	return DefaultSched.Run()
}

func (s *Schedule) Load(c *conf.Conf) {
	s.Conf = c
	s.RestoreState()
}

// Restores notification and alert state from the file on disk.
func (s *Schedule) RestoreState() {
	s.Lock()
	defer s.Unlock()
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost)
	s.Notifications = nil
	s.Status = make(map[AlertKey]*State)
	f, err := os.Open(s.Conf.StateFile)
	if err != nil {
		log.Println(err)
		return
	}
	dec := gob.NewDecoder(f)
	notifications := make(map[AlertKey]map[string]time.Time)
	if err := dec.Decode(&notifications); err != nil {
		log.Println(err)
		return
	}
	for {
		var ak AlertKey
		var st State
		if err := dec.Decode(&ak); err == io.EOF {
			break
		} else if err != nil {
			log.Println(err)
			return
		}
		if err := dec.Decode(&st); err != nil {
			log.Println(err)
			return
		}
		if a, present := s.Conf.Alerts[ak.Name]; !present {
			log.Println("sched: alert no longer present, ignoring:", ak)
			continue
		} else if a.Squelched(st.Group) {
			log.Println("sched: alert now squelched:", ak)
			continue
		} else {
			t := a.Unknown
			if t == 0 {
				t = s.Conf.Unknown
			}
			if t == 0 && st.Last().Status == stUnknown {
				st.Append(stNormal)
			}
		}
		s.Status[ak] = &st
		for name, t := range notifications[ak] {
			n, present := s.Conf.Notifications[name]
			if !present {
				log.Println("sched: notification not present during restore:", name)
				continue
			}
			s.AddNotification(ak, n, t)
		}
	}
}

func (s *Schedule) Save() {
	s.Lock()
	defer s.Unlock()
	f, err := os.Create(s.Conf.StateFile)
	if err != nil {
		log.Println(err)
		return
	}
	enc := gob.NewEncoder(f)
	if err := enc.Encode(s.Notifications); err != nil {
		log.Println(err)
		return
	}
	for k, v := range s.Status {
		enc.Encode(k)
		enc.Encode(v)
	}
	if err := f.Close(); err != nil {
		log.Println(err)
		return
	}
	log.Println("sched: wrote state to", s.Conf.StateFile)
}

func (s *Schedule) Run() error {
	go func() {
		for {
			time.Sleep(time.Minute)
			s.Save()
		}
	}()
	s.nc = make(chan interface{}, 1)
	go s.Poll()
	for {
		wait := time.After(s.Conf.CheckFrequency)
		if s.Conf.CheckFrequency < time.Second {
			return fmt.Errorf("sched: frequency must be > 1 second")
		}
		if s.Conf == nil {
			return fmt.Errorf("sched: nil configuration")
		}
		start := time.Now()
		log.Printf("starting run at %v\n", start)
		s.Check()
		log.Printf("run at %v took %v\n", start, time.Since(start))
		<-wait
	}
}

// Poll dispatches notification checks when needed.
func (s *Schedule) Poll() {
	var timeout time.Duration
	for {
		// Wait for one of these two.
		select {
		case <-time.After(timeout):
		case <-s.nc:
		}
		timeout = s.CheckNotifications()
	}
}

// CheckNotifications processes past notification events. It returns the
func (s *Schedule) CheckNotifications() time.Duration {
	s.Lock()
	defer s.Unlock()
	timeout := time.Hour
	notifications := s.Notifications
	s.Notifications = nil
	for ak, ns := range notifications {
		for name, t := range ns {
			n, present := s.Conf.Notifications[name]
			if !present {
				continue
			}
			remaining := t.Add(n.Timeout).Sub(time.Now())
			if remaining > 0 {
				if remaining < timeout {
					timeout = remaining
				}
				s.AddNotification(ak, n, t)
				continue
			}
			st, present := s.Status[ak]
			if !present {
				continue
			}
			a, present := s.Conf.Alerts[ak.Name]
			if !present {
				continue
			}
			s.Notify(st, a, n)
			if n.Timeout < timeout {
				timeout = n.Timeout
			}
		}
	}
	return timeout
}

func (s *Schedule) Check() {
	s.Lock()
	defer s.Unlock()
	s.runStates = make(map[AlertKey]Status)
	s.cache = opentsdb.NewCache(s.Conf.TsdbHost)
	for _, a := range s.Conf.Alerts {
		s.CheckAlert(a)
	}
	s.CheckUnknown()
	checkNotify := false
	for ak, status := range s.runStates {
		state := s.Status[ak]
		last := state.Append(status)
		a := s.Conf.Alerts[ak.Name]
		if status > stNormal {
			var subject = new(bytes.Buffer)
			if err := s.ExecuteSubject(subject, a, state); err != nil {
				log.Println(err)
			}
			state.Subject = subject.String()
		}
		// On state increase, clear old notifications and notify current.
		// On state decrease, and if the old alert was already acknowledged, notify current.
		// If the old alert was not acknowledged, do nothing.
		// Do nothing if state did not change.
		notify := func(notifications map[string]*conf.Notification) {
			for _, n := range notifications {
				s.Notify(state, a, n)
				checkNotify = true
			}
		}
		notifyCurrent := func() {
			switch status {
			case stCritical, stUnknown:
				notify(a.CritNotification)
			case stWarning:
				notify(a.WarnNotification)
			}
		}
		clearOld := func() {
			delete(s.Notifications, ak)
		}
		if status > last {
			clearOld()
			notifyCurrent()
		} else if status < last {
			if _, hasOld := s.Notifications[ak]; hasOld {
				notifyCurrent()
			}
		}
	}
	if checkNotify {
		s.nc <- true
	}
}

func (s *Schedule) CheckUnknown() {
	for ak, st := range s.Status {
		t := s.Conf.Alerts[ak.Name].Unknown
		if t == 0 {
			t = s.Conf.Unknown
		}
		if t == 0 {
			continue
		}
		if time.Since(st.Touched) < t {
			continue
		}
		s.runStates[ak] = stUnknown
	}
}

func (s *Schedule) CheckAlert(a *conf.Alert) {
	crits := s.CheckExpr(a, a.Crit, stCritical, nil)
	warns := s.CheckExpr(a, a.Warn, stWarning, crits)
	log.Printf("checking alert %v: %v crits, %v warns", a.Name, len(crits), len(warns))
}

func (s *Schedule) CheckExpr(a *conf.Alert, e *expr.Expr, checkStatus Status, ignore []AlertKey) (alerts []AlertKey) {
	if e == nil {
		return
	}
	results, _, err := e.Execute(s.cache, nil)
	if err != nil {
		// todo: do something here?
		log.Println(err)
		return
	}
Loop:
	for _, r := range results {
		if a.Squelched(r.Group) {
			continue
		}
		ak := AlertKey{a.Name, r.Group.String()}
		for _, v := range ignore {
			if ak == v {
				continue Loop
			}
		}
		state := s.Status[ak]
		if state == nil {
			state = &State{
				Group: r.Group,
			}
			s.Status[ak] = state
		}
		state.Touch()
		status := checkStatus
		state.Computations = r.Computations
		if r.Value.(expr.Number) != 0 {
			state.Expr = e.String()
			alerts = append(alerts, ak)
		} else {
			status = stNormal
		}
		if status > s.runStates[ak] {
			s.runStates[ak] = status
		}
	}
	return
}

func (s *Schedule) Notify(st *State, a *conf.Alert, n *conf.Notification) {
	subject := new(bytes.Buffer)
	if err := s.ExecuteSubject(subject, a, st); err != nil {
		log.Println(err)
	}
	body := new(bytes.Buffer)
	if err := s.ExecuteBody(body, a, st); err != nil {
		log.Println(err)
	}
	n.Notify(subject.Bytes(), body.Bytes(), s.Conf.EmailFrom, s.Conf.SmtpHost)
	if n.Next == nil {
		return
	}
	s.AddNotification(AlertKey{Name: a.Name, Group: st.Group.String()}, n, time.Now().UTC())
}

func (s *Schedule) AddNotification(ak AlertKey, n *conf.Notification, started time.Time) {
	if s.Notifications == nil {
		s.Notifications = make(map[AlertKey]map[string]time.Time)
	}
	if s.Notifications[ak] == nil {
		s.Notifications[ak] = make(map[string]time.Time)
	}
	s.Notifications[ak][n.Name] = started
	}
}

type AlertKey struct {
	Name  string
	Group string
}

func (a AlertKey) String() string {
	return a.Name + a.Group
}

type State struct {
	// Most recent event last.
	History      []Event
	Touched      time.Time
	Expr         string
	Group        opentsdb.TagSet
	Computations expr.Computations
	Subject      string
}

func (s *Schedule) Acknowledge(ak AlertKey) {
	s.Lock()
	delete(s.Notifications, ak)
	s.Unlock()
}

func (s *State) Touch() {
	s.Touched = time.Now().UTC()
}

// Appends status to the history if the status is different than the latest
// status. Returns the previous status.
func (s *State) Append(status Status) Status {
	last := s.Last()
	if len(s.History) == 0 || s.Last().Status != status {
		s.History = append(s.History, Event{status, time.Now().UTC()})
	}
	return last.Status
}

func (s *State) Last() Event {
	if len(s.History) == 0 {
		return Event{}
	}
	return s.History[len(s.History)-1]
}

type Event struct {
	Status Status
	Time   time.Time
}

type Status int

const (
	stNone Status = iota
	stNormal
	stWarning
	stCritical
	stUnknown
)

func (s Status) String() string {
	switch s {
	case stNormal:
		return "normal"
	case stWarning:
		return "warning"
	case stCritical:
		return "critical"
	case stUnknown:
		return "unknown"
	default:
		return "none"
	}
}
