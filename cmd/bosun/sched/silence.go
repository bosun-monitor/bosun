package sched

import (
	"fmt"
	"time"

	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

// Silenced returns all currently silenced AlertKeys and the time they will be
// unsilenced.
func (s *Schedule) Silenced() map[models.AlertKey]models.Silence {
	aks := make(map[models.AlertKey]models.Silence)

	now := time.Now()
	silences, err := s.DataAccess.Silence().GetActiveSilences()
	if err != nil {
		slog.Error("Error fetching silences.", err)
		return nil
	}
	for _, si := range silences {
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

func (s *Schedule) AddSilence(start, end time.Time, alert, tagList string, forget, confirm bool, edit, user, message string) (map[models.AlertKey]bool, error) {
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
	si := &models.Silence{
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
		si.TagString = tags.Tags()
	}
	if confirm {
		if edit != "" {
			if err := s.DataAccess.Silence().DeleteSilence(edit); err != nil {
				return nil, err
			}
		}
		if err := s.DataAccess.Silence().DeleteSilence(si.ID()); err != nil {
			return nil, err
		}
		if err := s.DataAccess.Silence().AddSilence(si); err != nil {
			return nil, err
		}
		return nil, nil
	}
	aks := make(map[models.AlertKey]bool)
	for ak := range s.status {
		if si.Matches(ak.Name(), ak.Group()) {
			aks[ak] = s.status[ak].IsActive()
		}
	}
	return aks, nil
}

func (s *Schedule) ClearSilence(id string) error {
	return s.DataAccess.Silence().DeleteSilence(id)
}
