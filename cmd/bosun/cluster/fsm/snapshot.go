package fsm

import (
	"encoding/json"

	"bosun.org/slog"
	"github.com/hashicorp/raft"
)

type Snapshot struct {
	Rules string
}

func (snap *Snapshot) Persist(sink raft.SnapshotSink) error {
	data, _ := json.Marshal(snap)
	_, err := sink.Write(data)
	if err != nil {
		sink.Cancel()
	}
	return err
}

func (snap *Snapshot) Release() {
	slog.Info("Release snapshot")
}
