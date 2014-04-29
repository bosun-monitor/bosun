package sched

import (
	"crypto/sha1"
	"fmt"
	"sort"
	"time"

	"github.com/StackExchange/tsaf/_third_party/github.com/StackExchange/scollector/opentsdb"
)

type Silence struct {
	Start, End  time.Time
	Alert, Tags string
	match       map[string]string
}

func (s *Silence) Silenced(alert string, tags opentsdb.TagSet) bool {
	now := time.Now()
	if now.Before(s.Start) || now.After(s.End) {
		return false
	}
	return s.Matches(alert, tags)
}

func (s *Silence) Matches(alert string, tags opentsdb.TagSet) bool {
	if s.Alert != "" && s.Alert != alert {
		return false
	}
	for k, pattern := range s.match {
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
	fmt.Fprintf(h, "%s%s%s{%s}", s.Start, s.End, s.Alert, s.Tags)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Silenced returns all currently silenced AlertKeys and the time they will be
// unsilenced.
func (s *Schedule) Silenced() map[AlertKey]time.Time {
	aks := make(map[AlertKey]time.Time)
	for _, si := range s.Silence {
		for ak, st := range s.Status {
			if si.Silenced(ak.Name, st.Group) {
				if aks[ak].Before(si.End) {
					aks[ak] = si.End
				}
				break
			}
		}
	}
	return aks
}

func (s *Schedule) AddSilence(start, end time.Time, alert, tagList string, confirm bool, edit string) (AlertKeys, error) {
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
		Start: start,
		End:   end,
		Alert: alert,
		match: make(map[string]string),
	}
	if tagList != "" {
		tags, err := opentsdb.ParseTags(tagList)
		if err != nil {
			return nil, err
		} else if len(tags) == 0 {
			return nil, fmt.Errorf("empty text")
		}
		si.Tags = tagList
		for k, v := range tags {
			_, err := Match(v, "")
			if err != nil {
				return nil, err
			}
			si.match[k] = v
		}
	}
	if confirm {
		s.Lock()
		delete(s.Silence, edit)
		s.Silence[si.ID()] = si
		s.Unlock()
		s.Save()
		return nil, nil
	}
	aks := make(AlertKeys, 0)
	for ak, st := range s.Status {
		if si.Matches(ak.Name, st.Group) {
			aks = append(aks, ak)
		}
	}
	sort.Sort(aks)
	return aks, nil
}

func (s *Schedule) ClearSilence(id string) error {
	s.Lock()
	delete(s.Silence, id)
	s.Unlock()
	s.Save()
	return nil
}
