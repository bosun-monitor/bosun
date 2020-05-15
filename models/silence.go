package models

import (
	"crypto/sha1"
	"fmt"
	"time"

	"bosun.org/opentsdb"
	"bosun.org/util"
)

// Silence is a struct representing silencing conditions
type Silence struct {
	Start, End time.Time
	Alert      string
	Tags       opentsdb.TagSet
	TagString  string
	Forget     bool
	User       string
	Message    string
}

// Silenced returns whether the receiver silences the given alert with tag set at the given time
func (s *Silence) Silenced(now time.Time, alert string, tags opentsdb.TagSet) bool {
	if !s.ActiveAt(now) {
		return false
	}
	return s.Matches(alert, tags)
}

// ActiveAt returns whether the receiver is active at the given time
func (s *Silence) ActiveAt(now time.Time) bool {
	if now.Before(s.Start) || now.After(s.End) {
		return false
	}
	return true
}

// Matches returns whether the receiver matches the given alert with tag set
func (s *Silence) Matches(alert string, tags opentsdb.TagSet) bool {
	if s.Alert != "" && s.Alert != alert {
		return false
	}
	for k, pattern := range s.Tags {
		tagv, ok := tags[k]
		if !ok {
			return false
		}
		matched, _ := util.Match(pattern, tagv)
		if !matched {
			return false
		}
	}
	return true
}

// ID returns the SHA-1 hash over start, end, alert and tags
func (s Silence) ID() string {
	h := sha1.New()
	fmt.Fprintf(h, "%s|%s|%s%s", s.Start, s.End, s.Alert, s.Tags)
	return fmt.Sprintf("%x", h.Sum(nil))
}
