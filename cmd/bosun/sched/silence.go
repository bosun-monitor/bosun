package sched

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/opentsdb"
)

type Silence struct {
	Start, End time.Time
	Alert      string
	Tags       opentsdb.TagSet
	Forget     bool
	User       string
	Message    string
}

func (s *Silence) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Start, End time.Time
		Alert      string
		Tags       string
		Forget     bool
		User       string
		Message    string
	}{
		Start:   s.Start,
		End:     s.End,
		Alert:   s.Alert,
		Tags:    s.Tags.Tags(),
		Forget:  s.Forget,
		User:    s.User,
		Message: s.Message,
	})
}

func (s *Silence) Silenced(now time.Time, alert string, tags opentsdb.TagSet) bool {
	if !s.ActiveAt(now) {
		return false
	}
	return s.Matches(alert, tags)
}

func (s *Silence) ActiveAt(now time.Time) bool {
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
		matched, _ := Match(pattern, tagv)
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

// Silenced returns all currently silenced AlertKeys and the time they will be
// unsilenced.
func (s *Schedule) Silenced() map[expr.AlertKey]Silence {
	aks := make(map[expr.AlertKey]Silence)
	now := time.Now()
	silenceLock.RLock()
	defer silenceLock.RUnlock()
	for _, si := range s.Silence {
		if !si.ActiveAt(now) {
			continue
		}
		s.Lock("Silence")
		for ak := range s.status {
			if si.Silenced(now, ak.Name(), ak.Group()) {
				if aks[ak].End.Before(si.End) {
					aks[ak] = *si
				}
			}
		}
		s.Unlock()
	}
	return aks
}

var silenceLock = sync.RWMutex{}

func (s *Schedule) AddSilence(start, end time.Time, alert, tagList string, forget, confirm bool, edit, user, message string) (map[expr.AlertKey]bool, error) {
	if start.IsZero() || end.IsZero() {
		return nil, fmt.Errorf("both start and end must be specified")
	}
	if start.After(end) {
		return nil, fmt.Errorf("start time must be before end time")
	}
	if time.Since(end) > 0 {
		return nil, fmt.Errorf("end time must be in the future")
	}
	if alert == "" && tagList == "" {
		return nil, fmt.Errorf("must specify either alert or tags")
	}
	si := &Silence{
		Start:   start,
		End:     end,
		Alert:   alert,
		Tags:    make(opentsdb.TagSet),
		Forget:  forget,
		User:    user,
		Message: message,
	}
	if tagList != "" {
		tags, err := opentsdb.ParseTags(tagList)
		if err != nil && tags == nil {
			return nil, err
		}
		si.Tags = tags
	}
	silenceLock.Lock()
	defer silenceLock.Unlock()
	if confirm {
		delete(s.Silence, edit)
		s.Silence[si.ID()] = si
		return nil, nil
	}
	aks := make(map[expr.AlertKey]bool)
	for ak := range s.status {
		if si.Matches(ak.Name(), ak.Group()) {
			aks[ak] = s.status[ak].IsActive()
		}
	}
	return aks, nil
}

func (s *Schedule) ClearSilence(id string) error {
	silenceLock.Lock()
	defer silenceLock.Unlock()
	delete(s.Silence, id)
	return nil
}
