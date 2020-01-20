package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"bosun.org/cmd/bosun/cluster"
	"bosun.org/slog"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/hashicorp/raft"
)

type ClusterOpResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type ClusterMember struct {
	Address  string
	Suffrage raft.ServerSuffrage
}

type ClusterStat struct {
	State string
	Nodes []ClusterNode
	Stats map[string]string
}

type ClusterNode struct {
	Address raft.ServerAddress
	State   string
}

func ClusterStatus(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var cs ClusterStat
	cs.Nodes = make([]ClusterNode, 0)

	cfuture := schedule.RaftInstance.Instance.GetConfiguration()

	if err := cfuture.Error(); err != nil {
		return nil, err
	}

	configuration := cfuture.Configuration()

	leaderAddress := schedule.RaftInstance.Instance.Leader()

	for _, server := range configuration.Servers {
		state := "Follower"
		if server.Address == leaderAddress {
			state = "Leader"
		}
		cs.Nodes = append(cs.Nodes, ClusterNode{Address: server.Address, State: state})
	}
	cs.Stats = schedule.RaftInstance.Instance.Stats()
	cs.State = schedule.RaftInstance.Instance.State().String()
	return cs, nil
}

func ClusterChangeMasterTo(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var data struct {
		Address string
	}
	j := json.NewDecoder(r.Body)
	if err := j.Decode(&data); err != nil {
		return nil, err
	}

	res := ClusterOpResult{Status: "ok"}
	f := schedule.RaftInstance.Instance.LeadershipTransferToServer(raft.ServerID(data.Address), raft.ServerAddress(data.Address))
	if f.Error() != nil {
		res.Status = "error"
		res.Error = f.Error().Error()
	}
	return res, nil
}

func ClusterRecover(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var data struct {
		Members []ClusterMember
	}

	j := json.NewDecoder(r.Body)
	if err := j.Decode(&data); err != nil {
		return nil, err
	}

	configuration := raft.Configuration{Servers: make([]raft.Server, 0)}

	for _, member := range data.Members {
		configuration.Servers = append(configuration.Servers, raft.Server{
			Suffrage: member.Suffrage, Address: raft.ServerAddress(member.Address), ID: raft.ServerID(member.Address)})
	}

	schedule.RaftInstance.Instance.Shutdown()

	slog.Infof("Start to recover cluster with configuration: %#v", configuration)
	if err := raft.RecoverCluster(schedule.RaftInstance.Config, &cluster.FSM{},
		schedule.RaftInstance.Db, schedule.RaftInstance.Db, schedule.RaftInstance.Snapshots,
		schedule.RaftInstance.Transport, configuration); err != nil {
		return nil, fmt.Errorf("recovery failed: %v", err)
	}

	var err error
	schedule.RaftInstance.Instance, err = raft.NewRaft(
		schedule.RaftInstance.Config, &cluster.FSM{}, schedule.RaftInstance.Db,
		schedule.RaftInstance.Db, schedule.RaftInstance.Snapshots, schedule.RaftInstance.Transport)

	for _, member := range schedule.RaftInstance.Serf.Members() {
		slog.Infof("Remove serf member: %s", member.Name)
		if err := schedule.RaftInstance.Serf.RemoveFailedNode(member.Name); err != nil {
			slog.Errorf("Error while remove serf member %s: %#v", member.Name, err)
		}
	}
	schedule.RaftInstance.RestartWatch <- true

	return nil, err
}
