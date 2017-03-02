package sched

import (
	"fmt"
	"sync"
	"time"

	"bosun.org/cmd/bosun/cache"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/collect"
	"bosun.org/opentsdb"
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

var checkContextLock = sync.RWMutex{}

func (s *Schedule) updateCheckContext() {
	for {
		ctx := &checkContext{utcNow(), cache.New(0)}
		checkContextLock.Lock()
		s.ctx = ctx
		checkContextLock.Unlock()
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
	var lastCheckTime time.Time
	// Calcaulate runEvery based on system default and override if an alert has a
	// custom runEvery
	runEvery := s.SystemConf.GetDefaultRunEvery()
	if a.RunEvery != 0 {
		runEvery = a.RunEvery
	}
	for {
		var ctx *checkContext
		for {
			checkContextLock.RLock()
			ctx = s.ctx
			checkContextLock.RUnlock()
			//make sure the context has actually changed
			if ctx.runTime != lastCheckTime {
				lastCheckTime = ctx.runTime
				break
			}
			//if the timing is just wrong, we could hit this twice in the same interval. One second wait should be enough
			//to desync us from the context changing routine.
			collect.Add("check.context_not_changed", opentsdb.TagSet{"alert": a.Name}, 1)
			time.Sleep(time.Second)
		}

		wait := time.After(s.SystemConf.GetCheckFrequency() * time.Duration(runEvery))
		s.checkAlert(a, ctx)
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

func (s *Schedule) checkAlert(a *conf.Alert, ctx *checkContext) {
	rh := s.NewRunHistory(ctx.runTime, ctx.checkCache)
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
