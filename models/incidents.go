package models

import (
	"encoding/json"
	"math"
	"time"

	"bosun.org/opentsdb"
)

type IncidentState struct {
	// Since IncidentState is embedded into a template's Context these fields
	// are available to users. Changes to this object should be reflected
	// in Bosun's documentation and changes that might break user's teamplates.
	// need to be considered.
	Id       int64
	Start    time.Time
	End      *time.Time
	AlertKey AlertKey
	Alert    string // helper data since AlertKeys don't serialize to JSON well
	Tags     string // string representation of Group

	*Result

	// Most recent last.
	Events  []Event  `json:",omitempty"`
	Actions []Action `json:",omitempty"`

	Subject string

	NeedAck bool
	Open    bool

	Unevaluated bool

	CurrentStatus Status
	WorstStatus   Status

	LastAbnormalStatus Status
	LastAbnormalTime   int64
}

type RenderedTemplates struct {
	Subject      string
	Body         string
	EmailBody    []byte
	EmailSubject []byte
	Custom       map[string]string
	Attachments  []*Attachment
}

func (r *RenderedTemplates) Get(name string) string {
	if name == "subject" {
		return r.Subject
	}
	if name == "body" {
		return r.Body
	}
	if name == "emailBody" {
		if r.EmailBody != nil {
			return string(r.EmailBody)
		}
		return r.Body
	}
	if name == "emailSubject" {
		if r.EmailSubject != nil {
			return string(r.EmailSubject)
		}
		return r.Subject
	}
	if t, ok := r.Custom[name]; ok {
		return t
	}
	//TODO if not exist maybe panic? We should really validate that things will be where we are looking
	return ""
}

func (r *RenderedTemplates) GetDefault(name string, defaultName string) string {
	if name == "" {
		name = defaultName
	}
	return r.Get(name)
}

func (s *IncidentState) Group() opentsdb.TagSet {
	return s.AlertKey.Group()
}

func (s *IncidentState) Last() Event {
	if len(s.Events) == 0 {
		return Event{}
	}
	return s.Events[len(s.Events)-1]
}

func (s *IncidentState) IsActive() bool {
	return s.CurrentStatus > StNormal
}

type Event struct {
	Warn, Crit  *Result `json:",omitempty"`
	Status      Status
	Time        time.Time
	Unevaluated bool
}

type EventsByTime []Event

func (a EventsByTime) Len() int           { return len(a) }
func (a EventsByTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a EventsByTime) Less(i, j int) bool { return a[i].Time.Before(a[j].Time) }

// custom float type to support json marshalling of NaN
type Float float64

func (m Float) MarshalJSON() ([]byte, error) {
	if math.IsNaN(float64(m)) {
		return []byte("null"), nil
	}
	return json.Marshal(float64(m))
}

func (m *Float) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		*m = Float(math.NaN())
		return nil
	}
	var f float64
	err := json.Unmarshal(b, &f)
	*m = Float(f)
	return err
}

type Result struct {
	Computations `json:",omitempty"`
	Value        Float
	Expr         string
}

type Computations []Computation

type Computation struct {
	Text  string
	Value interface{}
}

type FuncType int

func (f FuncType) String() string {
	switch f {
	case TypeNumberSet:
		return "number"
	case TypeString:
		return "string"
	case TypeSeriesSet:
		return "series"
	case TypeScalar:
		return "scalar"
	case TypeESQuery:
		return "esquery"
	case TypeESIndexer:
		return "esindexer"
	case TypeNumberExpr:
		return "numberexpr"
	case TypeSeriesExpr:
		return "seriesexpr"
	case TypePrefix:
		return "prefix"
	case TypeTable:
		return "table"
	case TypeVariantSet:
		return "variantSet"
	default:
		return "unknown"
	}
}

const (
	TypeString FuncType = iota
	TypePrefix
	TypeScalar
	TypeNumberSet
	TypeSeriesSet
	TypeESQuery
	TypeESIndexer
	TypeNumberExpr
	TypeSeriesExpr // No implmentation yet
	TypeTable
	TypeVariantSet
	TypeUnexpected
)

type Status int

const (
	StNone Status = iota
	StNormal
	StWarning
	StCritical
	StUnknown
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
	default:
		return "none"
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Status) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case `"normal"`:
		*s = StNormal
	case `"warning"`:
		*s = StWarning
	case `"critical"`:
		*s = StCritical
	case `"unknown"`:
		*s = StUnknown
	default:
		*s = StNone
	}
	return nil
}

func (s Status) IsNormal() bool   { return s == StNormal }
func (s Status) IsWarning() bool  { return s == StWarning }
func (s Status) IsCritical() bool { return s == StCritical }
func (s Status) IsUnknown() bool  { return s == StUnknown }

type Action struct {
	// These are available to users via the template language. Changes here
	// should be reflected in the documentation
	User       string
	Message    string
	Time       time.Time
	Type       ActionType
	Deadline   *time.Time `json:",omitempty"`
	Fullfilled bool
	Cancelled  bool
}

type ActionType int // Available to users in templates, document changes in Bosun docs

const (
	ActionNone ActionType = iota
	ActionAcknowledge
	ActionClose
	ActionForget
	ActionForceClose
	ActionPurge
	ActionNote
	ActionDelayedClose
	ActionCancelClose
)

func (a ActionType) String() string {
	switch a {
	case ActionAcknowledge:
		return "Acknowledged"
	case ActionClose:
		return "Closed"
	case ActionForget:
		return "Forgotten"
	case ActionForceClose:
		return "ForceClosed"
	case ActionPurge:
		return "Purged"
	case ActionNote:
		return "Note"
	case ActionDelayedClose:
		return "DelayedClose"
	case ActionCancelClose:
		return "CancelClose"
	default:
		return "none"
	}
}

func (a ActionType) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

func (a *ActionType) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case `"Acknowledged"`:
		*a = ActionAcknowledge
	case `"Closed"`:
		*a = ActionClose
	case `"Forgotten"`:
		*a = ActionForget
	case `"Purged"`:
		*a = ActionPurge
	case `"ForceClosed"`:
		*a = ActionForceClose
	case `"Note"`:
		*a = ActionNote
	case `"DelayedClose"`:
		*a = ActionDelayedClose
	case `"CancelClose"`:
		*a = ActionCancelClose
	default:
		*a = ActionNone
	}
	return nil
}

type Attachment struct {
	Data        []byte
	Filename    string
	ContentType string
}
