package sched // import "bosun.org/cmd/bosun/sched"

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"bosun.org/_third_party/github.com/boltdb/bolt"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/_third_party/github.com/bradfitz/slice"
	"bosun.org/_third_party/github.com/tatsushid/go-fastping"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/search"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	gob.Register(expr.Number(0))
	gob.Register(expr.Scalar(0))
}

type Schedule struct {
	sync.Mutex

	Conf          *conf.Conf
	status        States
	Notifications map[expr.AlertKey]map[string]time.Time
	Silence       map[string]*Silence
	Group         map[time.Time]expr.AlertKeys
	Metadata      map[metadata.Metakey]*Metavalue
	Search        *search.Search

	LastCheck     time.Time
	nc            chan interface{}
	notifications map[*conf.Notification][]*State
	metalock      sync.Mutex
	checkRunning  chan bool
	db            *bolt.DB
}

func (s *Schedule) TimeLock(t miniprofiler.Timer) {
	t.Step("lock", func(t miniprofiler.Timer) {
		s.Lock()
	})
}

type Metavalue struct {
	Time  time.Time
	Value interface{}
}

func (s *Schedule) PutMetadata(k metadata.Metakey, v interface{}) {
	s.metalock.Lock()
	s.Metadata[k] = &Metavalue{time.Now().UTC(), v}
	s.Save()
	s.metalock.Unlock()
}

type MetadataMetric struct {
	Unit        string `json:",omitempty"`
	Type        string `json:",omitempty"`
	Description []*MetadataDescription
}

type MetadataDescription struct {
	Tags opentsdb.TagSet `json:",omitempty"`
	Text string
}

func (s *Schedule) MetadataMetrics(metric string) map[string]*MetadataMetric {
	s.metalock.Lock()
	m := make(map[string]*MetadataMetric)
	for k, mv := range s.Metadata {
		tags := k.TagSet()
		delete(tags, "host")
		if k.Metric == "" || (metric != "" && k.Metric != metric) {
			continue
		}
		val, _ := mv.Value.(string)
		if val == "" {
			continue
		}
		if m[k.Metric] == nil {
			m[k.Metric] = &MetadataMetric{
				Description: make([]*MetadataDescription, 0),
			}
		}
		e := m[k.Metric]
	Switch:
		switch k.Name {
		case "unit":
			e.Unit = val
		case "rate":
			e.Type = val
		case "desc":
			for _, v := range e.Description {
				if v.Text == val {
					v.Tags = v.Tags.Intersection(tags)
					break Switch
				}
			}
			e.Description = append(e.Description, &MetadataDescription{
				Text: val,
				Tags: tags,
			})
		}
	}
	s.metalock.Unlock()
	return m
}

func (s *Schedule) GetMetadata(metric string, subset opentsdb.TagSet) []metadata.Metasend {
	s.metalock.Lock()
	ms := make([]metadata.Metasend, 0)
	for k, mv := range s.Metadata {
		if metric != "" && k.Metric != metric {
			continue
		}
		if !k.TagSet().Subset(subset) {
			continue
		}
		ms = append(ms, metadata.Metasend{
			Metric: k.Metric,
			Tags:   k.TagSet(),
			Name:   k.Name,
			Value:  mv.Value,
			Time:   mv.Time,
		})
	}
	s.metalock.Unlock()
	return ms
}

type States map[expr.AlertKey]*State

type StateTuple struct {
	NeedAck bool
	Active  bool
	Status  Status
}

// GroupStates groups by NeedAck, Active, and Status.
func (states States) GroupStates() map[StateTuple]States {
	r := make(map[StateTuple]States)
	for ak, st := range states {
		t := StateTuple{
			st.NeedAck,
			st.IsActive(),
			st.AbnormalStatus(),
		}
		if _, present := r[t]; !present {
			r[t] = make(States)
		}
		r[t][ak] = st
	}
	return r
}

// GroupSets returns slices of TagSets, grouped by most common ancestor. Those
// with no shared ancestor are grouped by alert name.
func (states States) GroupSets() map[string]expr.AlertKeys {
	type Pair struct {
		k, v string
	}
	groups := make(map[string]expr.AlertKeys)
	seen := make(map[*State]bool)
	for {
		counts := make(map[Pair]int)
		for _, s := range states {
			if seen[s] {
				continue
			}
			for k, v := range s.Group {
				counts[Pair{k, v}]++
			}
		}
		if len(counts) == 0 {
			break
		}
		max := 0
		var pair Pair
		for p, c := range counts {
			if c > max {
				max = c
				pair = p
			}
		}
		if max == 1 {
			break
		}
		var group expr.AlertKeys
		for _, s := range states {
			if seen[s] {
				continue
			}
			if s.Group[pair.k] != pair.v {
				continue
			}
			seen[s] = true
			group = append(group, s.AlertKey())
		}
		if len(group) > 0 {
			groups[fmt.Sprintf("{%s=%s}", pair.k, pair.v)] = group
		}
	}
	// alerts
	for {
		if len(seen) == len(states) {
			break
		}
		var group expr.AlertKeys
		for _, s := range states {
			if seen[s] {
				continue
			}
			if group == nil || s.AlertKey().Name() == group[0].Name() {
				group = append(group, s.AlertKey())
				seen[s] = true
			}
		}
		if len(group) > 0 {
			groups[group[0].Name()] = group
		}
	}
	return groups
}

type StateGroup struct {
	Active   bool `json:",omitempty"`
	Status   Status
	Subject  string        `json:",omitempty"`
	Len      int           `json:",omitempty"`
	Alert    string        `json:",omitempty"`
	AlertKey expr.AlertKey `json:",omitempty"`
	Ago      string        `json:",omitempty"`
	Children []*StateGroup `json:",omitempty"`
}

type StateGroups struct {
	Groups struct {
		NeedAck      []*StateGroup `json:",omitempty"`
		Acknowledged []*StateGroup `json:",omitempty"`
	}
	TimeAndDate []int
	Silenced    map[expr.AlertKey]Silence
}

func (s *Schedule) MarshalGroups(T miniprofiler.Timer, filter string) (*StateGroups, error) {
	t := StateGroups{
		TimeAndDate: s.Conf.TimeAndDate,
		Silenced:    s.Silenced(),
	}
	s.TimeLock(T)
	defer s.Unlock()
	status := make(States)
	matches, err := makeFilter(filter)
	if err != nil {
		return nil, err
	}
	for k, v := range s.status {
		if !v.Open {
			continue
		}
		a := s.Conf.Alerts[k.Name()]
		if a == nil {
			return nil, fmt.Errorf("unknown alert %s", k.Name())
		}
		if matches(s.Conf, a, v) {
			status[k] = v
		}
	}
	var groups map[StateTuple]States
	T.Step("GroupStates", func(T miniprofiler.Timer) {
		groups = status.GroupStates()
	})
	T.Step("groups", func(T miniprofiler.Timer) {
		for tuple, states := range groups {
			var grouped []*StateGroup
			switch tuple.Status {
			case StWarning, StCritical, StUnknown, StError:
				var sets map[string]expr.AlertKeys
				T.Step(fmt.Sprintf("GroupSets (%d): %v", len(states), tuple), func(T miniprofiler.Timer) {
					sets = states.GroupSets()
				})
				for name, group := range sets {
					g := StateGroup{
						Active:  tuple.Active,
						Status:  tuple.Status,
						Subject: fmt.Sprintf("%s - %s", tuple.Status, name),
						Len:     len(group),
					}
					for _, ak := range group {
						st := s.status[ak]
						g.Children = append(g.Children, &StateGroup{
							Active:   tuple.Active,
							Status:   tuple.Status,
							AlertKey: ak,
							Alert:    ak.Name(),
							Subject:  string(st.Subject),
							Ago:      marshalTime(st.Last().Time),
						})
					}
					grouped = append(grouped, &g)
				}
			default:
				continue
			}
			if tuple.NeedAck {
				t.Groups.NeedAck = append(t.Groups.NeedAck, grouped...)
			} else {
				t.Groups.Acknowledged = append(t.Groups.Acknowledged, grouped...)
			}
		}
	})
	gsort := func(grp []*StateGroup) func(i, j int) bool {
		return func(i, j int) bool {
			a := grp[i]
			b := grp[j]
			if a.Active && !b.Active {
				return true
			} else if !a.Active && b.Active {
				return false
			}
			if a.Status != b.Status {
				return a.Status > b.Status
			}
			if a.AlertKey != b.AlertKey {
				return a.AlertKey < b.AlertKey
			}
			return a.Subject < b.Subject
		}
	}
	slice.Sort(t.Groups.NeedAck, gsort(t.Groups.NeedAck))
	slice.Sort(t.Groups.Acknowledged, gsort(t.Groups.Acknowledged))
	return &t, nil
}

func marshalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	b, _ := t.MarshalText()
	return string(b)
}

var DefaultSched = &Schedule{}

// Load loads a configuration into the default schedule.
func Load(c *conf.Conf) error {
	return DefaultSched.Load(c)
}

// Run runs the default schedule.
func Run() error {
	return DefaultSched.Run()
}

func (s *Schedule) Init(c *conf.Conf) error {
	var err error
	s.Conf = c
	s.Silence = make(map[string]*Silence)
	s.Group = make(map[time.Time]expr.AlertKeys)
	s.Metadata = make(map[metadata.Metakey]*Metavalue)
	s.status = make(States)
	s.Search = search.NewSearch()
	s.checkRunning = make(chan bool, 1)
	if c.StateFile != "" {
		s.db, err = bolt.Open(c.StateFile, 0600, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Schedule) Load(c *conf.Conf) error {
	if err := s.Init(c); err != nil {
		return err
	}
	if s.db == nil {
		return nil
	}
	return s.RestoreState()
}

func Close() {
	DefaultSched.Close()
}

func (s *Schedule) Close() {
	s.save()
	if s.db != nil {
		s.db.Close()
	}
}

// RestoreState restores notification and alert state from the file on disk.
func (s *Schedule) RestoreState() error {
	log.Println("RestoreState")
	start := time.Now()
	s.Lock()
	defer s.Unlock()
	s.Search.Lock()
	defer s.Search.Unlock()
	s.Notifications = nil
	decode := func(name string, dst interface{}) error {
		var data []byte
		err := s.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(dbBucket))
			if b == nil {
				return fmt.Errorf("unknown bucket: %v", dbBucket)
			}
			data = b.Get([]byte(name))
			return nil
		})
		if err != nil {
			return err
		}
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return err
		}
		defer gr.Close()
		return gob.NewDecoder(gr).Decode(dst)
	}
	if err := decode(dbMetric, &s.Search.Metric); err != nil {
		log.Println(dbMetric, err)
	}
	if err := decode(dbTagk, &s.Search.Tagk); err != nil {
		log.Println(dbTagk, err)
	}
	if err := decode(dbTagv, &s.Search.Tagv); err != nil {
		log.Println(dbTagv, err)
	}
	if err := decode(dbMetricTags, &s.Search.MetricTags); err != nil {
		log.Println(dbMetricTags, err)
	}
	notifications := make(map[expr.AlertKey]map[string]time.Time)
	if err := decode(dbNotifications, &notifications); err != nil {
		log.Println(dbNotifications, err)
	}
	if err := decode(dbSilence, &s.Silence); err != nil {
		log.Println(dbSilence, err)
	}
	status := make(States)
	if err := decode(dbStatus, &status); err != nil {
		log.Println(dbStatus, err)
	}
	for ak, st := range status {
		if a, present := s.Conf.Alerts[ak.Name()]; !present {
			log.Println("sched: alert no longer present, ignoring:", ak)
			continue
		} else if s.Conf.Squelched(a, st.Group) {
			log.Println("sched: alert now squelched:", ak)
			continue
		} else {
			t := a.Unknown
			if t == 0 {
				t = s.Conf.CheckFrequency
			}
			if t == 0 && st.Last().Status == StUnknown {
				st.Append(&Event{Status: StNormal})
			}
		}
		s.status[ak] = st
		for name, t := range notifications[ak] {
			n, present := s.Conf.Notifications[name]
			if !present {
				log.Println("sched: notification not present during restore:", name)
				continue
			}
			s.AddNotification(ak, n, t)
		}
	}
	if err := decode(dbMetadata, &s.Metadata); err != nil {
		log.Println(dbMetadata, err)
	}
	s.Search.Copy()
	log.Println("RestoreState done in", time.Since(start))
	return nil
}

var savePending bool

func (s *Schedule) Save() {
	go func() {
		s.Lock()
		defer s.Unlock()
		if savePending {
			return
		}
		savePending = true
		time.AfterFunc(time.Second*5, s.save)
	}()
}

type counterWriter struct {
	written int
	w       io.Writer
}

func (c *counterWriter) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	c.written += n
	return n, err
}

const (
	dbBucket        = "bindata"
	dbMetric        = "metric"
	dbTagk          = "tagk"
	dbTagv          = "tagv"
	dbMetricTags    = "metrictags"
	dbNotifications = "notifications"
	dbSilence       = "silence"
	dbStatus        = "status"
	dbMetadata      = "metadata"
)

func (s *Schedule) save() {
	defer func() {
		savePending = false
	}()
	if s.db == nil {
		return
	}
	store := map[string]interface{}{
		dbMetric:        s.Search.Read.Metric,
		dbTagk:          s.Search.Read.Tagk,
		dbTagv:          s.Search.Read.Tagv,
		dbMetricTags:    s.Search.Read.MetricTags,
		dbNotifications: s.Notifications,
		dbSilence:       s.Silence,
		dbStatus:        s.status,
		dbMetadata:      s.Metadata,
	}
	tostore := make(map[string][]byte)
	for name, data := range store {
		f := new(bytes.Buffer)
		gz := gzip.NewWriter(f)
		cw := &counterWriter{w: gz}
		enc := gob.NewEncoder(cw)
		if err := enc.Encode(data); err != nil {
			log.Printf("error saving %s: %v", name, err)
			return
		}
		if err := gz.Flush(); err != nil {
			log.Printf("gzip flush error saving %s: %v", name, err)
		}
		if err := gz.Close(); err != nil {
			log.Printf("gzip close error saving %s: %v", name, err)
		}
		tostore[name] = f.Bytes()
		log.Printf("wrote %s: %v", name, conf.ByteSize(cw.written))
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(dbBucket))
		if err != nil {
			return err
		}
		for name, data := range tostore {
			if err := b.Put([]byte(name), data); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("save db update error: %v", err)
		return
	}
	log.Println("save to db complete")
}

func (s *Schedule) Run() error {
	s.nc = make(chan interface{}, 1)
	if s.Conf.Ping {
		go s.PingHosts()
	}
	go s.Poll()
	for {
		if s.Conf == nil {
			return fmt.Errorf("sched: nil configuration")
		}
		if s.Conf.CheckFrequency < time.Second {
			return fmt.Errorf("sched: frequency must be > 1 second")
		}
		wait := time.After(s.Conf.CheckFrequency)
		log.Println("starting check")
		now := time.Now()
		dur, err := s.Check(nil, now)
		if err != nil {
			log.Println(err)
		}
		log.Printf("check took %v\n", dur)
		s.LastCheck = now
		<-wait
	}
}

const pingFreq = time.Second * 15

func (s *Schedule) PingHosts() {
	for range time.Tick(pingFreq) {
		hosts := s.Search.TagValuesByTagKey("host", s.Conf.PingDuration)
		for _, host := range hosts {
			go pingHost(host)
		}
	}
}

func pingHost(host string) {
	p := fastping.NewPinger()
	tags := opentsdb.TagSet{"dst_host": host}
	resolved := 0
	defer func() {
		collect.Put("ping.resolved", tags, resolved)
	}()
	ra, err := net.ResolveIPAddr("ip4:icmp", host)
	if err != nil {
		return
	}
	resolved = 1
	p.AddIPAddr(ra)
	p.MaxRTT = time.Second * 5
	timeout := 1
	p.OnRecv = func(addr *net.IPAddr, t time.Duration) {
		collect.Put("ping.rtt", tags, float64(t)/float64(time.Millisecond))
		timeout = 0
	}
	if err := p.Run(); err != nil {
		log.Print(err)
	}
	collect.Put("ping.timeout", tags, timeout)
}

type State struct {
	*Result

	// Most recent last.
	History      []Event  `json:",omitempty"`
	Actions      []Action `json:",omitempty"`
	Touched      time.Time
	Alert        string // helper data since AlertKeys don't serialize to JSON well
	Tags         string // string representation of Group
	Group        opentsdb.TagSet
	Subject      string
	Body         string
	EmailBody    []byte             `json:"-"`
	EmailSubject []byte             `json:"-"`
	Attachments  []*conf.Attachment `json:"-"`
	NeedAck      bool
	Open         bool
	Forgotten    bool
	Unevaluated  bool
}

func (s *State) AlertKey() expr.AlertKey {
	return expr.NewAlertKey(s.Alert, s.Group)
}

func (s *State) Status() Status {
	return s.Last().Status
}

// AbnormalEvent returns the most recent non-normal event, or nil if none found.
func (s *State) AbnormalEvent() *Event {
	for i := len(s.History) - 1; i >= 0; i-- {
		if ev := s.History[i]; ev.Status > StNormal {
			return &ev
		}
	}
	return nil
}

// AbnormalStatus returns the most recent non-normal status, or StNone if none
// found.
func (s *State) AbnormalStatus() Status {
	ev := s.AbnormalEvent()
	if ev != nil {
		return ev.Status
	}
	return StNone
}

func (s *State) IsActive() bool {
	return s.Status() > StNormal
}

func (s *State) Action(user, message string, t ActionType) {
	s.Actions = append(s.Actions, Action{
		User:    user,
		Message: message,
		Type:    t,
		Time:    time.Now().UTC(),
	})
}

func (s *Schedule) Action(user, message string, t ActionType, ak expr.AlertKey) error {
	s.Lock()
	defer func() {
		s.Unlock()
		s.Save()
	}()
	st := s.status[ak]
	if st == nil {
		return fmt.Errorf("no such alert key: %v", ak)
	}
	ack := func() {
		delete(s.Notifications, ak)
		st.NeedAck = false
	}
	isUnknown := st.AbnormalStatus() == StUnknown
	isError := st.AbnormalStatus() == StError
	switch t {
	case ActionAcknowledge:
		if !st.NeedAck {
			return fmt.Errorf("alert already acknowledged")
		}
		if !st.Open {
			return fmt.Errorf("cannot acknowledge closed alert")
		}
		ack()
	case ActionClose:
		if st.NeedAck {
			ack()
		}
		if st.IsActive() && !isError {
			return fmt.Errorf("cannot close active alert")
		}
		st.Open = false
	case ActionForget:
		if !isUnknown {
			return fmt.Errorf("can only forget unknowns")
		}
		if st.NeedAck {
			ack()
		}
		st.Open = false
		st.Forgotten = true
		delete(s.status, ak)
	default:
		return fmt.Errorf("unknown action type: %v", t)
	}
	st.Action(user, message, t)
	// Would like to also track the alert group, but I believe this is impossible because any character
	// that could be used as a delimiter could also be a valid tag key or tag value character
	if err := collect.Add("actions", opentsdb.TagSet{"user": user, "alert": ak.Name(), "type": t.String()}, 1); err != nil {
		log.Println(err)
	}
	return nil
}

func (s *State) Touch() {
	s.Touched = time.Now().UTC()
	s.Forgotten = false
}

// Append appends status to the history if the status is different than the
// latest status. Returns the previous status.
func (s *State) Append(event *Event) Status {
	last := s.Last()
	if len(s.History) == 0 || s.Last().Status != event.Status {
		event.Time = time.Now().UTC()
		s.History = append(s.History, *event)
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
	Warn, Crit, Error *Result
	Status            Status
	Time              time.Time
	Unevaluated       bool
}

type Result struct {
	*expr.Result
	Expr string
}

type Status int

const (
	StNone Status = iota
	StNormal
	StWarning
	StCritical
	StUnknown
	StError
)

func (s Status) String() string {
	switch s {
	case StNormal:
		return "normal"
	case StWarning:
		return "warning"
	case StCritical:
		return "critical"
	case StUnknown:
		return "unknown"
	case StError:
		return "error"
	default:
		return "none"
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s Status) IsNormal() bool   { return s == StNormal }
func (s Status) IsWarning() bool  { return s == StWarning }
func (s Status) IsCritical() bool { return s == StCritical }
func (s Status) IsUnknown() bool  { return s == StUnknown }
func (s Status) IsError() bool    { return s == StError }

type Action struct {
	User    string
	Message string
	Time    time.Time
	Type    ActionType
}

type ActionType int

const (
	ActionNone ActionType = iota
	ActionAcknowledge
	ActionClose
	ActionForget
)

func (a ActionType) String() string {
	switch a {
	case ActionAcknowledge:
		return "Acknowledged"
	case ActionClose:
		return "Closed"
	case ActionForget:
		return "Forgotten"
	default:
		return "none"
	}
}

func (a ActionType) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

func (s *Schedule) Host(filter string) map[string]*HostData {
	s.metalock.Lock()
	res := make(map[string]*HostData)
	for k, mv := range s.Metadata {
		tags := k.TagSet()
		if k.Metric != "" || tags["host"] == "" {
			continue
		}
		e := res[tags["host"]]
		if e == nil {
			host := fmt.Sprintf("{host=%s}", tags["host"])
			e = &HostData{
				Name:       tags["host"],
				Metrics:    s.Search.MetricsByTagPair("host", tags["host"]),
				Interfaces: make(map[string]*HostInterface),
			}
			e.CPU.Processors = make(map[string]string)
			if v, err := s.Search.GetLast("os.cpu", host, true); err != nil {
				e.CPU.Used = v
			}
			e.Memory.Modules = make(map[string]string)
			if v, err := s.Search.GetLast("os.mem.total", host, false); err != nil {
				e.Memory.Total = int64(v)
			}
			if v, err := s.Search.GetLast("os.mem.used", host, false); err != nil {
				e.Memory.Used = int64(v)
			}
			res[tags["host"]] = e
		}
		var iface *HostInterface
		if name := tags["iface"]; name != "" {
			if e.Interfaces[name] == nil {
				h := new(HostInterface)
				itag := opentsdb.TagSet{
					"host":  tags["host"],
					"iface": name,
				}
				intag := opentsdb.TagSet{"direction": "in"}.Merge(itag).String()
				if v, err := s.Search.GetLast("os.net.bytes", intag, true); err != nil {
					h.Inbps = int64(v) * 8
				}
				outtag := opentsdb.TagSet{"direction": "out"}.Merge(itag).String()
				if v, err := s.Search.GetLast("os.net.bytes", outtag, true); err != nil {
					h.Outbps = int64(v) * 8
				}
				e.Interfaces[name] = h
			}
			iface = e.Interfaces[name]
		}
		switch val := mv.Value.(type) {
		case string:
			switch k.Name {
			case "addr":
				if iface != nil {
					iface.IPAddresses = append(iface.IPAddresses, val)
				}
			case "description":
				if iface != nil {
					iface.Description = val
				}
			case "mac":
				if iface != nil {
					iface.MAC = val
				}
			case "manufacturer":
				e.Manufacturer = val
			case "master":
				if iface != nil {
					iface.Master = val
				}
			case "memory":
				if name := tags["name"]; name != "" {
					e.Memory.Modules[name] = val
				}
			case "model":
				e.Model = val
			case "name":
				if iface != nil {
					iface.Name = val
				}
			case "processor":
				if name := tags["name"]; name != "" {
					e.CPU.Processors[name] = val
				}
			case "serialNumber":
				e.SerialNumber = val
			case "version":
				e.OS.Version = val
			case "versionCaption", "uname":
				e.OS.Caption = val
			}
		case float64:
			switch k.Name {
			case "memoryTotal":
				e.Memory.Total = int64(val)
			case "speed":
				if iface != nil {
					iface.LinkSpeed = int64(val)
				}
			}
		}
	}
	s.metalock.Unlock()
	return res
}

type HostInterface struct {
	Description string   `json:",omitempty"`
	IPAddresses []string `json:",omitempty"`
	Inbps       int64    `json:",omitempty"`
	LinkSpeed   int64    `json:",omitempty"`
	MAC         string   `json:",omitempty"`
	Master      string   `json:",omitempty"`
	Name        string   `json:",omitempty"`
	Outbps      int64    `json:",omitempty"`
}

type HostData struct {
	CPU struct {
		Logical    int64             `json:",omitempty"`
		Physical   int64             `json:",omitempty"`
		Used       float64           `json:",omitempty"`
		Processors map[string]string `json:",omitempty"`
	}
	Interfaces   map[string]*HostInterface
	LastBoot     int64  `json:",omitempty"`
	LastUpdate   int64  `json:",omitempty"`
	Manufacturer string `json:",omitempty"`
	Memory       struct {
		Modules map[string]string `json:",omitempty"`
		Total   int64             `json:",omitempty"`
		Used    int64             `json:",omitempty"`
	}
	Metrics []string
	Model   string `json:",omitempty"`
	Name    string `json:",omitempty"`
	OS      struct {
		Caption string `json:",omitempty"`
		Version string `json:",omitempty"`
	}
	SerialNumber string `json:",omitempty"`
}
