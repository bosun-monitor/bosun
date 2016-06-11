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
	if s.Conf == nil {
		return fmt.Errorf("sched: nil configuration")
	}
	s.nc = make(chan interface{}, 1)
	if s.Conf.Ping {
		go s.PingHosts()
	}
	go s.dispatchNotifications()
	go s.updateCheckContext()
	for _, a := range s.Conf.Alerts {
		go s.RunAlert(a)
	}
	return nil
}
func (s *Schedule) updateCheckContext() {
	for {
		ctx := &checkContext{utcNow(), cache.New(0)}
		s.ctx = ctx
		time.Sleep(s.Conf.CheckFrequency)
		s.Lock("CollectStates")
		s.CollectStates()
		s.Unlock()
	}
}
func (s *Schedule) RunAlert(a *conf.Alert) {
	s.checksRunning.Add(1)
	defer s.checksRunning.Done()
	for {
		wait := time.After(s.Conf.CheckFrequency * time.Duration(a.RunEvery))
		s.checkAlert(a)
		s.LastCheck = utcNow()
		select {
		case <-wait:
		case <-s.runnerContext.Done(): // for closing
			slog.Infoln("stoping alert routine for %v", a.Name)
			return
		}
	}
}

func (s *Schedule) checkAlert(a *conf.Alert) {
	checkTime := s.ctx.runTime
	checkCache := s.ctx.checkCache
	rh := s.NewRunHistory(checkTime, checkCache)
	s.CheckAlert(nil, rh, a)

	start := utcNow()
	s.RunHistory(rh)
	slog.Infof("runHistory on %s took %v\n", a.Name, time.Since(start))
}
