package models

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"bosun.org/opentsdb"
)

// IncidentState is the state of an incident
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

	LastAbnormalTime Epoch

	PreviousIds []int64 // A list to the previous IncidentIds for the same alert key (alertname+tagset)
	NextId      int64   // The id of the next Incident Id for the same alert key, only added once a future incident has been created

	// set of notifications we have already sent alerts to during the lifetime of the incident
	Notifications []string
}

// SetNotified marks the notification name as "active" for this incident.
// All future actions and unknown notifications will go to all "active" notifications
// it returns true if the set was changed (and needs resaving)
func (is *IncidentState) SetNotified(not string) bool {
	for _, n := range is.Notifications {
		if n == not {
			return false
		}
	}
	is.Notifications = append(is.Notifications, not)
	return true
}

// Epoch is a wrapper around `time.Time` to allow custom (un-)marshalling
type Epoch struct {
	time.Time
}

// MarshalJSON is a custom JSON marshaller converting the Epoch to a unix timestamp in UTC
func (t Epoch) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%v", t.UTC().Unix())), nil
}

// UnmarshalJSON is a custom JSON unmarshaller from a unix timestamp in UTC
func (t *Epoch) UnmarshalJSON(b []byte) (err error) {
	if len(b) == 0 {
		t.Time = time.Time{}
		return
	}
	epoch, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return err
	}
	t.Time = time.Unix(epoch, 0)
	return
}

// RenderedTemplates is a template that has been rendered through the rendering engine, i.e. all variables have been
// filled in with concrete values
type RenderedTemplates struct {
	Subject      string
	Body         string
	EmailBody    []byte
	EmailSubject []byte
	Custom       map[string]string
	Attachments  []*Attachment
}

// Get returns a variable defined in the template
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
	return ""
}

// GetDefault returns the value of a variable in the receiver, filling in the default value if the variable cannot be
// found
func (r *RenderedTemplates) GetDefault(name string, defaultName string) string {
	if name == "" {
		name = defaultName
	}
	return r.Get(name)
}

// Group returns the group of the alert
func (is *IncidentState) Group() opentsdb.TagSet {
	return is.AlertKey.Group()
}

// Last returns the most recent event
func (is *IncidentState) Last() Event {
	if len(is.Events) == 0 {
		return Event{}
	}
	return is.Events[len(is.Events)-1]
}

// IsActive returns whether the state is worse than normal
func (is *IncidentState) IsActive() bool {
	return is.CurrentStatus > StNormal
}

// Event is the result of an evaluation of an alert
type Event struct {
	Warn        *Result `json:",omitempty"`
	Crit        *Result `json:",omitempty"`
	Status      Status
	Time        time.Time
	Unevaluated bool
}

// EventsByTime is a sortable slice of `Event`s
type EventsByTime []Event

func (a EventsByTime) Len() int           { return len(a) }
func (a EventsByTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a EventsByTime) Less(i, j int) bool { return a[i].Time.Before(a[j].Time) }

// Float is a custom float type to support json marshalling of NaN
type Float float64

// MarshalJSON is a custom JSON marshaller
func (m Float) MarshalJSON() ([]byte, error) {
	if math.IsNaN(float64(m)) {
		return []byte("null"), nil
	}
	return json.Marshal(float64(m))
}

// UnmarshalJSON is a custom JSON unmarshaller
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

// Result is the result of a number of computations and its final value
type Result struct {
	Computations `json:",omitempty"`
	Value        Float
	Expr         string
}

// Computations is a slice of `Computation`s
type Computations []Computation

// Computation is a computation and its outcome that has been made during the evaluation of expressions
type Computation struct {
	Text  string
	Value interface{}
}

// FuncType is the type of a function in the Bosun language
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
	case TypeAzureResourceList:
		return "azureResources"
	case TypeAzureAIApps:
		return "azureAIApps"
	case TypeInfo:
		return "info"
	default:
		return "unknown"
	}
}

// Constants for the different types of functions known in the Bosun language
const (
	TypeString FuncType = iota
	TypePrefix
	TypeScalar
	TypeNumberSet
	TypeSeriesSet
	TypeESQuery
	TypeESIndexer
	TypeNumberExpr
	TypeSeriesExpr // No implementation yet
	TypeTable
	TypeVariantSet
	TypeAzureResourceList
	TypeAzureAIApps
	TypeInfo
	TypeUnexpected
)

// Status is an enumeration for the different states an alert can take
type Status int

// Constants for the different alert states
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

// MarshalJSON is a custom JSON marshaller
func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON is a custom JSON unmarshaller
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

// Action represents an action triggered on the web interface by a user
//
// Examples include acknowledging or closing alerts
type Action struct {
	// These are available to users via the template language. Changes here
	// should be reflected in the documentation
	User      string
	Message   string
	Time      time.Time
	Type      ActionType
	Deadline  *time.Time `json:",omitempty"`
	Fulfilled bool
	Cancelled bool
}

// ActionType is an enumeration available to users in templates, document changes in Bosun docs
type ActionType int

// Constants for the various types of actions a user can take
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

//ActionShortNames is a map of keys we use in config file (notifications mostly) to reference action types
var ActionShortNames = map[string]ActionType{
	"Ack":          ActionAcknowledge,
	"Close":        ActionClose,
	"Forget":       ActionForget,
	"ForceClose":   ActionForceClose,
	"Purge":        ActionPurge,
	"Note":         ActionNote,
	"DelayedClose": ActionDelayedClose,
	"CancelClose":  ActionCancelClose,
}

// HumanString gives a better human readable form than the default stringer, which we can't change due to marshalling compatibility now
func (a ActionType) HumanString() string {
	switch a {
	case ActionAcknowledge:
		return "Acknowledged"
	case ActionClose:
		return "Closed"
	case ActionForget:
		return "Forgot"
	case ActionForceClose:
		return "Force Closed"
	case ActionPurge:
		return "Purged"
	case ActionNote:
		return "Commented On"
	case ActionDelayedClose:
		return "Delayed Closed"
	case ActionCancelClose:
		return "Canceled Close"
	default:
		return "none"
	}
}

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

// MarshalJSON is a custom JSON marshaller
func (a ActionType) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON is a custom JSON unmarshaller
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

// Attachment represents an attachment to e.g. an email
type Attachment struct {
	Data        []byte
	Filename    string
	ContentType string
}
