package sched

import (
	"fmt"
	"time"

	"bosun.org/cmd/bosun/cache"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/slog"
)

// Run should be called once (and only once) to start all schedule activity.
func (s *Schedule) Run() error {
	if s.RuleConf == nil || s.SystemConf == nil {
		return fmt.Errorf("sched: nil configuration")
	}
	s.nc = make(chan interface{}, 1)
	go s.dispatchNotifications()
	go s.updateCheckContext()
	for _, a := range s.RuleConf.GetAlerts() {
		go s.RunAlert(a)
	}
	return nil
}

func (s *Schedule) updateCheckContext() {
	for {
		ctx := &checkContext{utcNow(), cache.New(0)}
		s.ctx = ctx
		time.Sleep(s.SystemConf.GetCheckFrequency())
		s.Lock("CollectStates")
		s.CollectStates()
		s.Unlock()
	}
}

func (s *Schedule) RunAlert(a *conf.Alert) {
	// Add to waitgroup for running alert
	s.checksRunning.Add(1)
	// ensure when an alert is done it is removed from the wait group
	defer s.checksRunning.Done()
	for {
		// Calcaulate runEvery based on system default and override if an alert has a
		// custom runEvery
		runEvery := s.SystemConf.GetDefaultRunEvery()
		if a.RunEvery != 0 {
			runEvery = a.RunEvery
		}
		wait := time.After(s.SystemConf.GetCheckFrequency() * time.Duration(runEvery))
		s.checkAlert(a)
		s.LastCheck = utcNow()
		select {
		case <-wait:
		case <-s.runnerContext.Done():
			// If an alert is waiting we cancel it
			slog.Infof("Stopping alert routine for %v\n", a.Name)
			return
		}
	}
}

func (s *Schedule) checkAlert(a *conf.Alert) {
	checkTime := s.ctx.runTime
	checkCache := s.ctx.checkCache
	rh := s.NewRunHistory(checkTime, checkCache)
	// s.CheckAlert will return early if the schedule has been closed
	cancelled := s.CheckAlert(nil, rh, a)
	if cancelled {
		// Don't runHistory for the alert if expression evaluation has been cancelled
		return
	}
	start := utcNow()
	s.RunHistory(rh)
	slog.Infof("runHistory on %s took %v\n", a.Name, time.Since(start))
}
