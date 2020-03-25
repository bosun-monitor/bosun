package fsm

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule"
	promstat "bosun.org/collect/prometheus"
	"bosun.org/slog"
	"github.com/hashicorp/raft"
)

const (
	ACTION_APPLY_CONFIG = 0
	ACTION_APPLY_RULES  = iota
)

type ClusterCommand struct {
	Cmd  uint8
	Data string
}

type FSM struct {
	sync.Mutex

	Rules       string
	SetRules    func(*rule.Conf)
	SysProvider conf.SystemConfProvider
	Reload      func(*rule.Conf) error
}

func (fsm *FSM) Apply(l *raft.Log) interface{} {
	var a ClusterCommand
	fsm.Lock()
	defer fsm.Unlock()

	if err := json.Unmarshal(l.Data, &a); err != nil {
		slog.Errorf("decode raft log err %v", err)
		return err
	}

	fsm.Rules = a.Data

	return fsm.handleAction(&a)
}

func (fsm *FSM) Snapshot() (raft.FSMSnapshot, error) {
	promstat.ClusterSnapshots.Inc()
	fsm.Lock()
	snap := &Snapshot{
		Rules: fsm.Rules,
	}
	fsm.Unlock()
	return snap, nil
}

func (fsm *FSM) Restore(snap io.ReadCloser) error {
	defer snap.Close()

	d := json.NewDecoder(snap)
	var res Snapshot

	if err := d.Decode(&res); err != nil {
		return err
	}

	fsm.Lock()
	fsm.Rules = res.Rules
	c, err := rule.NewConf(fsm.SysProvider.GetRuleFilePath(), fsm.SysProvider.EnabledBackends(), fsm.SysProvider.GetRuleVars(), fsm.Rules)
	if err != nil {
		slog.Errorf("Error while reload cluster rule config. Error while parse config: %v", err)
	}
	fsm.SetRules(c)
	fsm.Unlock()

	return nil
}

func (fsm *FSM) handleAction(a *ClusterCommand) error {
	switch a.Cmd {
	case ACTION_APPLY_CONFIG:
		slog.Warning("Skip ro apply new config: not supported")
		return fmt.Errorf("APPLY_CONFIG command isn't supported yet")
	case ACTION_APPLY_RULES:
		slog.Infoln("Apply new rules")
		c, err := rule.NewConf(fsm.SysProvider.GetRuleFilePath(), fsm.SysProvider.EnabledBackends(), fsm.SysProvider.GetRuleVars(), fsm.Rules)
		if err != nil {
			slog.Errorf("Error while reload cluster rule config. Error while parse config: %v", err)
			return err
		}
		if err := fsm.Reload(c); err != nil {
			slog.Errorf("Error while reload cluster rule config: %v", err)
			return err
		}
		return nil
	default:
		slog.Warningf("Unknown raft command: %v", a)
		return fmt.Errorf("Unknown raft command")
	}
}
