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
	type alertCh struct {
		ch     chan<- *checkContext
		modulo int
		shift  int // used to distribute alert runs
	}
	chs := []alertCh{}

	// Every alert gets a small shift in time.
	// This way the alerts with the same period are not fired
	// simultaneously, but are distributed.
	circular_shifts := make(map[int]int) // the map is *run period* -> *time shift to add*
	for _, a := range s.RuleConf.GetAlerts() {
		ch := make(chan *checkContext, 1)
		re := a.RunEvery
		if re == 0 {
			re = s.SystemConf.GetDefaultRunEvery()
		}
		go s.runAlert(a, ch)

		if s.SystemConf.GetAlertCheckDistribution() == "simple" { // only apply shifts if the respective option is set
			chs = append(chs, alertCh{ch: ch, modulo: re, shift: circular_shifts[re]})
		} else {
			// there are no shifts if option is off
			chs = append(chs, alertCh{ch: ch, modulo: re, shift: 0})
		}

		// the shifts for a given period range 0..(period - 1)
		circular_shifts[re] = (circular_shifts[re] + 1) % re
	}
	i := 0
	for {
		select {
		case <-s.runnerContext.Done():
			return nil
		default:
		}
		ctx := &checkContext{utcNow(), cache.New("alerts", 0)}
		s.LastCheck = utcNow()
		for _, a := range chs {
			if (i+a.shift)%a.modulo != 0 {
				continue
			}
			// Put on channel. If that fails, the alert is backed up pretty bad.
			// Because channel is buffered size 1, it will continue as soon as it finishes.
			// Master scheduler will never block here.
			select {
			case a.ch <- ctx:
			default:
			}
		}
		i++
		time.Sleep(s.SystemConf.GetCheckFrequency())
		s.Lock("CollectStates")
		s.CollectStates()
		s.Unlock()
	}
}

func (s *Schedule) runAlert(a *conf.Alert, ch <-chan *checkContext) {
	// Add to waitgroup for running alert
	s.checksRunning.Add(1)
	// ensure when an alert is done it is removed from the wait group
	defer s.checksRunning.Done()
	for {
		select {
		case <-s.runnerContext.Done():
			slog.Infof("Stopping alert routine for %v\n", a.Name)
			return
		case ctx := <-ch:
			s.checkAlert(a, ctx)

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
