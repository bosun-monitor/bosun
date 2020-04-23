package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"bosun.org/cmd/bosun/cluster"
	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/hashicorp/raft"
)

type ClusterOpResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type ClusterStat struct {
	State string
	Nodes []ClusterNode
	Stats map[string]string
}

type ClusterNode struct {
	Address raft.ServerAddress
	Id      raft.ServerID
	State   string
}

func ClusterStatus(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if schedule.RaftInstance == nil {
		return nil, errors.New("Cluster isn't enabled")
	}
	var cs ClusterStat
	cs.Nodes = make([]ClusterNode, 0)

	cfuture := schedule.RaftInstance.GetConfiguration()

	if err := cfuture.Error(); err != nil {
		return nil, err
	}

	configuration := cfuture.Configuration()

	leaderAddress := schedule.RaftInstance.GetLeader()

	for _, server := range configuration.Servers {
		state := "Follower"
		if server.Address == leaderAddress {
			state = "Leader"
		}
		cs.Nodes = append(cs.Nodes, ClusterNode{Address: server.Address, State: state, Id: server.ID})
	}
	cs.Stats = schedule.RaftInstance.GetStat()
	cs.State = schedule.RaftInstance.State().String()
	return cs, nil
}

func ClusterChangeMasterTo(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var data cluster.ClusterNode
	j := json.NewDecoder(r.Body)
	if err := j.Decode(&data); err != nil {
		return nil, err
	}

	if data.Id == "" {
		data.Id = raft.ServerID(data.Address)
	}

	res := ClusterOpResult{Status: "ok"}
	lead := schedule.RaftInstance.GetLeader()

	if err := schedule.RaftInstance.ChangeMaster(&data); err != nil {
		res.Status = "error"
		res.Error = err.Error()
	}

	e := "Timeout was reached (10s). It is possible that something went wrong"
	for i := 0; i < 10; i++ {
		if schedule.RaftInstance.GetLeader() != lead {
			e = ""
			break
		}
		time.Sleep(time.Second)
	}
	res.Error = e
	return res, nil
}

func ClusterRecover(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var data struct {
		Members []cluster.ClusterNode
	}

	j := json.NewDecoder(r.Body)
	if err := j.Decode(&data); err != nil {
		return nil, err
	}

	if err := schedule.RaftInstance.Recover(data.Members); err != nil {
		return nil, err
	}
	return "recovered", nil
}
