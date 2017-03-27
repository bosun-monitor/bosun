package annotate

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	glob "github.com/ryanuber/go-glob"
)

type RFC3339 struct {
	time.Time
}

func (t RFC3339) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.Format(time.RFC3339) + `"`), nil
}

func (t *RFC3339) UnmarshalJSON(b []byte) (err error) {
	if b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	}
	if len(b) == 0 {
		t.Time = time.Time{}
		return
	}
	t.Time, err = time.Parse(time.RFC3339, string(b))
	return
}

type Epoch struct {
	time.Time
}

func (t Epoch) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%v", t.UTC().Unix())), nil
}

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

func NewAnnotation(id string, start, end time.Time, user, owner, source, host, category, url, message string) (a Annotation) {
	a.Id = id
	a.StartDate.Time = start
	a.EndDate.Time = end
	a.CreationUser = user
	a.Owner = owner
	a.Source = source
	a.Category = category
	a.Url = url
	a.Message = message
	a.Host = host
	return
}

type Annotation struct {
	AnnotationFields
	StartDate RFC3339
	EndDate   RFC3339
}

type EpochAnnotation struct {
	AnnotationFields
	StartDate Epoch
	EndDate   Epoch
}

// AnnotationsByStartID is a Type to sort by start time then by id
type AnnotationsByStartID Annotations

func (b AnnotationsByStartID) Len() int      { return len(b) }
func (b AnnotationsByStartID) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b AnnotationsByStartID) Less(i, j int) bool {
	if b[i].StartDate.Time.Before(b[j].StartDate.Time) {
		return true
	}
	if b[i].StartDate.Time.After(b[j].StartDate.Time) {
		return false
	}
	return b[i].Id < b[j].Id
}

func emptyOrGlob(userVal, fieldVal string) bool {
	if userVal == "empty" && fieldVal == "" {
		return true
	}

	return glob.Glob(strings.ToLower(userVal), strings.ToLower(fieldVal))
}

// Ask makes it so annotations can be filtered in memory using
// github.com/kylebrandt/boolq
func (a Annotation) Ask(filter string) (bool, error) {
	sp := strings.SplitN(filter, ":", 2)
	if len(sp) != 2 {
		return false, fmt.Errorf("bad filter, filter must be in k:v format, got %v", filter)
	}
	key := sp[0]
	value := sp[1]
	switch key {
	case "owner":
		return emptyOrGlob(value, a.Owner), nil
	case "user":
		return emptyOrGlob(value, a.CreationUser), nil
	case "host":
		return emptyOrGlob(value, a.Host), nil
	case "category":
		return emptyOrGlob(value, a.Category), nil
	case "url":
		return emptyOrGlob(value, a.Url), nil
	case "message":
		return emptyOrGlob(value, a.Message), nil
	default:
		return false, fmt.Errorf("invalid keyword: %s", key)
	}
}

func (ea *EpochAnnotation) AsAnnotation() (a Annotation) {
	a.AnnotationFields = ea.AnnotationFields
	a.StartDate.Time = ea.StartDate.Time
	a.EndDate.Time = ea.EndDate.Time
	return
}

func (a *Annotation) AsEpochAnnotation() (ea EpochAnnotation) {
	ea.AnnotationFields = a.AnnotationFields
	ea.StartDate.Time = a.StartDate.Time
	ea.EndDate.Time = a.EndDate.Time
	return
}

type AnnotationFields struct {
	Id           string
	Message      string
	CreationUser string
	Url          string
	Source       string
	Host         string
	Owner        string
	Category     string
}

const (
	Message      = "Message"
	StartDate    = "StartDate"
	EndDate      = "EndDate"
	Source       = "Source"
	Host         = "Host"
	CreationUser = "CreationUser"
	Owner        = "Owner"
	Category     = "Category"
	Url          = "Url"
)

type Annotations []Annotation
type EpochAnnotations []EpochAnnotation

func (as Annotations) AsEpochAnnotations() EpochAnnotations {
	eas := make(EpochAnnotations, len(as))
	for i, a := range as {
		eas[i] = a.AsEpochAnnotation()
	}
	return eas
}

func (a *Annotation) SetNow() {
	a.StartDate.Time = time.Now()
	a.EndDate = a.StartDate
}

func (a *Annotation) IsTimeNotSet() bool {
	t := time.Time{}
	return a.StartDate.Equal(t) || a.EndDate.Equal(t)
}

func (a *Annotation) IsOneTimeSet() bool {
	t := time.Time{}
	return (a.StartDate.Equal(t) && !a.EndDate.Equal(t)) || (!a.StartDate.Equal(t) && a.EndDate.Equal(t))
}

// Match Times Sets Both times to the greater of the two times
func (a *Annotation) MatchTimes() {
	if a.StartDate.After(a.EndDate.Time) {
		a.EndDate = a.StartDate
		return
	}
	a.StartDate = a.EndDate
}

func (a *Annotation) ValidateTime() error {
	t := time.Time{}
	if a.StartDate.Equal(t) {
		return fmt.Errorf("StartDate is not set")
	}
	if a.EndDate.Equal(t) {
		return fmt.Errorf("StartDate is not set")
	}
	if a.EndDate.Before(a.StartDate.Time) {
		return fmt.Errorf("EndDate is before StartDate")
	}
	return nil
}
