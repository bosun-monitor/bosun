package cluster

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"bosun.org/slog"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/hashicorp/serf/serf"
	"log"
	"path/filepath"
)

type fsm struct {
}

func (f *fsm) Apply(*raft.Log) interface{} {
	return nil
}

func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
}

func (f *fsm) Restore(io.ReadCloser) error {
	return nil
}

type clusterLog struct{}

func (cl clusterLog) Write(data []byte) (n int, err error) {
	line := strings.TrimSuffix(string(data), "\n")
	slog.Infoln(line)
	return len(data), nil
}

func StartCluster(listen, dbPath string, members []string, serfEvents chan serf.Event) (raftInstance *raft.Raft, err error) {
	var logOutput clusterLog
	logger := log.New(logOutput, "[cluser]", 0)
	serfListen := strings.Split(listen, ":")
	if len(serfListen) != 2 {
		return nil, errors.New("Incorrect serf listen address: " + listen)
	}
	serfPort, err := strconv.Atoi(serfListen[1])
	if err != nil {
		return nil, errors.New("Incorrect port definition for serf listener: " + err.Error())
	}

	// create hashicorp memberlist config
	memberlistConfig := memberlist.DefaultLANConfig()
	memberlistConfig.BindAddr = serfListen[0]
	memberlistConfig.BindPort = serfPort
	memberlistConfig.LogOutput = logOutput

	// create hashicorp serf config
	serfConfig := serf.DefaultConfig()
	serfConfig.NodeName = listen
	serfConfig.EventCh = serfEvents
	serfConfig.MemberlistConfig = memberlistConfig
	serfConfig.LogOutput = logOutput

	serfListener, err := serf.Create(serfConfig)
	if err != nil {
		return nil, errors.New("Error while init serf listener: " + err.Error())
	}

	if len(members) > 0 {
		if _, err := serfListener.Join(members, false); err != nil {
			return nil, errors.New("Error join cluster: " + err.Error())
		}
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		slog.Infoln("Create cluster metadata path", dbPath)
		if err := os.MkdirAll(dbPath, 0755); err != nil {
			return nil, errors.New("Error while create raft metadata path for cluster: " + err.Error())
		}
	}

	raftDbPath := filepath.Join(dbPath, "raft.db")

	raftDB, err := raftboltdb.NewBoltStore(raftDbPath)
	snapshotStore, err := raft.NewFileSnapshotStoreWithLogger(dbPath, 1, logger)
	if err != nil {
		return nil, errors.New("Error while init snapshots store path: " + err.Error())
	}

	// start raft init
	raftPort := serfPort + 1
	raftListen := fmt.Sprintf("%s:%d", serfListen[0], raftPort)

	raftTransport, err := raft.NewTCPTransportWithLogger(raftListen, nil, 3, 10*time.Second, logger)
	if err != nil {
		return nil, errors.New("Error while init Raft transport: " + err.Error())
	}

	c := raft.DefaultConfig()
	c.LogOutput = logOutput
	c.LocalID = raft.ServerID(raftListen)

	raftInstance, err = raft.NewRaft(c, &fsm{}, raftDB, raftDB, snapshotStore, raftTransport)
	if err != nil {
		return nil, errors.New("Error while create raft instance: " + err.Error())
	}

	if !isBootstrapped(raftInstance) {
		if err := bootstrapCluster(raftInstance, raftListen, members); err != nil {
			return nil, errors.New("Error while bootstrap cluster: " + err.Error())
		}
	}
	return
}

func isBootstrapped(instance *raft.Raft) bool {
	cfuture := instance.GetConfiguration()
	if err := cfuture.Error(); err != nil {
		return false
	}
	configuration := cfuture.Configuration()
	if len(configuration.Servers) > 0 {
		return true
	}
	return false
}

func bootstrapCluster(instance *raft.Raft, raftListen string, members []string) error {
	bootstrapConfig := raft.Configuration{
		Servers: []raft.Server{
			{
				Suffrage: raft.Voter,
				ID:       raft.ServerID(raftListen),
				Address:  raft.ServerAddress(raftListen),
			},
		},
	}
	// Add known peers to bootstrap
	for _, node := range members {
		if node == raftListen {
			continue
		}
		bootstrapConfig.Servers = append(bootstrapConfig.Servers, raft.Server{
			Suffrage: raft.Voter,
			ID:       raft.ServerID(node),
			Address:  raft.ServerAddress(node),
		})
	}

	f := instance.BootstrapCluster(bootstrapConfig)
	return f.Error()
}
