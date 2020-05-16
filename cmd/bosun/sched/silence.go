package sched

import (
	"fmt"
	"time"

	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

// SilenceTester is a function to preview to determine if the given alert key is silenced at the current time
type SilenceTester func(models.AlertKey) *models.Silence

// Silenced returns a function that will determine if the given alert key is silenced at the current time.
// A function is returned to avoid needing to enumerate all alert keys unnecessarily.
func (s *Schedule) Silenced() SilenceTester {
	now := utcNow()
	silences, err := s.DataAccess.Silence().GetActiveSilences()
	if err != nil {
		slog.Error("Error fetching silences.", err)
		return nil
	}
	return func(ak models.AlertKey) *models.Silence {
		var lastEnding *models.Silence
		for _, si := range silences {
			if !si.ActiveAt(now) {
				continue
			}
			if si.Silenced(now, ak.Name(), ak.Group()) {
				if lastEnding == nil || lastEnding.End.Before(si.End) {
					lastEnding = si
				}
			}
		}
		return lastEnding
	}
}

// AddSilence adds a scheduled silence for a given alert with tags
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
	open, err := s.DataAccess.State().GetAllOpenIncidents()
	if err != nil {
		return nil, err
	}
	for _, inc := range open {
		if si.Matches(inc.Alert, inc.AlertKey.Group()) {
			aks[inc.AlertKey] = true
		}
	}
	return aks, nil
}

// ClearSilence removes the silence with the given ID
func (s *Schedule) ClearSilence(id string) error {
	return s.DataAccess.Silence().DeleteSilence(id)
}
