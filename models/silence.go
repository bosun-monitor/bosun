package models

import (
	"crypto/sha1"
	"fmt"
	"time"

	"bosun.org/opentsdb"
	"bosun.org/util"
)

type Silence struct {
	Start, End time.Time
	PeriodTimeIntStart, PeriodTimeIntEnd  int
	Alert      string
	Tags       opentsdb.TagSet
	TagString  string
	Forget     bool
	User       string
	Message    string
}

func (s *Silence) Silenced(now time.Time, alert string, tags opentsdb.TagSet) bool {
	if !s.ActiveAt(now) {
		return false
	}
	return s.Matches(alert, tags)
}

func (s *Silence) ActiveAt(now time.Time) bool {
	// period silence judge
	if s.PeriodTimeIntStart + s.PeriodTimeIntEnd > 0 {  // at leat one of them is greater than 0
		nowHMInt := now.Hour() * 10000 + now.Minute() * 100 + now.Second()
		if s.PeriodTimeIntStart < s.PeriodTimeIntEnd {
			if nowHMInt > s.PeriodTimeIntStart && nowHMInt < s.PeriodTimeIntEnd {
				return true
			} else {
				return false
			}
		} else {
			if nowHMInt > s.PeriodTimeIntStart || nowHMInt < s.PeriodTimeIntEnd {
				return true
			} else {
				return false
			}
		}
	}

	if now.Before(s.Start) || now.After(s.End) {
		return false
	}
	return true
}

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

func (s Silence) ID() string {
	h := sha1.New()
	fmt.Fprintf(h, "%s|%s|%s%s", s.Start, s.End, s.Alert, s.Tags)
	return fmt.Sprintf("%x", h.Sum(nil))
}
