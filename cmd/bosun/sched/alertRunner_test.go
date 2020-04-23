package sched

import (
	"testing"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule"
	"bosun.org/host"
	"bosun.org/util"
)

func TestAlertRunner_ClusterLeader(t *testing.T) {
	defer setup()()
	hm, err := host.NewManager(false)
	if err != nil {
		t.Error(err)
	}
	util.SetHostManager(hm)
	c, err := rule.NewConf("", conf.EnabledBackends{}, nil, `
			alert a {
				crit = 1
			}
		`)
	if err != nil {
		t.Fatal(err)
	}
	s, err := initSched(&conf.SystemConf{}, c, utcNow(), &clusterTest{leader: true})
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan *checkContext, 1)
	chs := []alertCh{{ch: ch, modulo: 1, shift: 0}}
	go s.RunLoop(chs)
	for {
		select {
		case <-ch:
			return
		case <-time.After(time.Millisecond):
			t.Fatal("check wasn't run within 1 second")
		}
	}
}

func TestAlertRunner_ClusterFollower(t *testing.T) {
	defer setup()()
	hm, err := host.NewManager(false)
	if err != nil {
		t.Error(err)
	}
	util.SetHostManager(hm)
	c, err := rule.NewConf("", conf.EnabledBackends{}, nil, `
			alert a {
				crit = 1
			}
		`)
	if err != nil {
		t.Fatal(err)
	}
	s, err := initSched(&conf.SystemConf{}, c, utcNow(), &clusterTest{})
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan *checkContext, 1)
	chs := []alertCh{{ch: ch, modulo: 1, shift: 0}}
	go s.RunLoop(chs)
	for {
		select {
		case <-ch:
			t.Fatal("check was run but should not be while node in follower state")
		case <-time.After(time.Millisecond):
			return
		}
	}
}
