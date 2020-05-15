package models

import (
	"fmt"
	"strings"

	"bosun.org/opentsdb"
)

// AlertKey is the string representation of an Alert name with its tags which uniquely identifies an alert
type AlertKey string

// ParseAlertKey parses an `AlertKey` from a string
func ParseAlertKey(a string) (ak AlertKey, err error) {
	ak = AlertKey(a)
	defer func() {
		e := recover()
		if e != nil {
			err = fmt.Errorf("%v", e)
		}
	}()
	ak.Group()
	return
}

// NewAlertKey creates a new `AlertKey` from a name and a set of tags
func NewAlertKey(name string, group opentsdb.TagSet) AlertKey {
	return AlertKey(name + group.String())
}

// Name returns the name of the alert (without its group)
func (a AlertKey) Name() string {
	return strings.SplitN(string(a), "{", 2)[0]
}

// Group returns the tagset of this alert key. Will panic if a is not a valid
// AlertKey. OpenTSDB tag validation errors are ignored.
func (a AlertKey) Group() opentsdb.TagSet {
	sp := strings.SplitN(string(a), "{", 2)
	if len(sp) < 2 {
		panic(fmt.Errorf("invalid alert key %s", a))
	}
	s := sp[1]
	s = s[:len(s)-1]
	if s == "" {
		return nil
	}
	g, err := opentsdb.ParseTags(s)
	if g == nil && err != nil {
		panic(err)
	}
	return g
}

// AlertKeys is a sortable slice of `AlertKey`s
type AlertKeys []AlertKey

func (a AlertKeys) Len() int           { return len(a) }
func (a AlertKeys) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a AlertKeys) Less(i, j int) bool { return a[i] < a[j] }
