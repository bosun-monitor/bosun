package web

import (
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/hashicorp/raft"
	"net/http"
)

type ClusterStat struct {
	State       raft.RaftState
	StateString string
	Nodes       []ClusterNode
	Stats       map[string]string
}

type ClusterNode struct {
	Address raft.ServerAddress
	ID      raft.ServerID
	State   string
}

func ClusterStatus(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var cs ClusterStat
	cs.Nodes = make([]ClusterNode, 0)

	cfuture := schedule.RaftInstance.GetConfiguration()

	if err := cfuture.Error(); err != nil {
		return nil, err
	}

	configuration := cfuture.Configuration()

	leaderAddress := schedule.RaftInstance.Leader()

	for _, server := range configuration.Servers {
		state := "Follower"
		if server.Address == leaderAddress {
			state = "Leader"
		}
		cs.Nodes = append(cs.Nodes, ClusterNode{Address: server.Address, State: state, ID: server.ID})
	}
	cs.Stats = schedule.RaftInstance.Stats()
	cs.State = schedule.RaftInstance.State()
	cs.StateString = schedule.RaftInstance.State().String()
	return cs, nil
}
