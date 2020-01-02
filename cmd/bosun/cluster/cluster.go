package cluster

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"log"
	"path/filepath"

	"bosun.org/slog"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/hashicorp/serf/serf"
)

type Raft struct {
	Instance      *raft.Raft
	Config        *raft.Config
	logger        *log.Logger
	Db            *raftboltdb.BoltStore
	Snapshots     *raft.FileSnapshotStore
	Transport     *raft.NetworkTransport
	Serf          *serf.Serf
	ReloadCluster chan bool
	RestartWatch  chan bool
	serfEvents    chan serf.Event
}

func (r *Raft) CreateRaftDb(path string) error {
	raftDbPath := filepath.Join(path, "raft.db")

	db, err := raftboltdb.NewBoltStore(raftDbPath)
	r.Db = db
	return err
}

func (r *Raft) CreateSnapshotStore(path string) error {
	s, err := raft.NewFileSnapshotStoreWithLogger(path, 1, r.logger)
	r.Snapshots = s
	return err
}

func (r *Raft) CreateRaftTransport(listen string) error {
	t, err := raft.NewTCPTransportWithLogger(listen, nil, 3, 10*time.Second, r.logger)
	r.Transport = t
	return err
}

func (r *Raft) Watch(flagQuiet, flagNoChecks *bool) {
	raftCh := r.Instance.LeaderCh()
	for {
		select {
		case <-r.RestartWatch:
			raftCh = r.Instance.LeaderCh()
		case isLeader := <-raftCh:
			if isLeader {
				slog.Infoln("Node was selected as an leader")
				*flagQuiet = false
				*flagNoChecks = false
				r.ReloadCluster <- true
				slog.Infoln("Reloading service for disable read only mode")
			} else {
				slog.Infoln("Node was selected as an follower")
				*flagQuiet = true
				*flagNoChecks = true
				r.ReloadCluster <- true
				slog.Infoln("Reloading service for enable read only mode")
			}
		case ev := <-r.serfEvents:
			slog.Infof("Received cluster event: %#v.", ev)

			leader := r.Instance.VerifyLeader()
			if memberEvent, ok := ev.(serf.MemberEvent); ok {
				for _, member := range memberEvent.Members {
					changedPeer := member.Addr.String() + ":" + strconv.Itoa(int(member.Port+1))
					slog.Infof("Received cluster event %s: %#v. member: %s", memberEvent.EventType().String(), ev, changedPeer)
					if memberEvent.EventType() == serf.EventMemberJoin {
						if leader.Error() == nil {
							f := r.Instance.AddVoter(raft.ServerID(changedPeer), raft.ServerAddress(changedPeer), 0, 0)
							if f.Error() != nil {
								slog.Fatalf("error adding voter: %s", f.Error())
							}
						}
					} else if memberEvent.EventType() == serf.EventMemberLeave || memberEvent.EventType() == serf.EventMemberFailed || memberEvent.EventType() == serf.EventMemberReap {
						if leader.Error() == nil {
							f := r.Instance.RemoveServer(raft.ServerID(changedPeer), 0, 0)
							if f.Error() != nil {
								slog.Fatalf("error removing server: %s", f.Error())
							}
						}
					}
				}
			}
		}
	}
}

type FSM struct {
}

type snapshot struct{}

func (s snapshot) Persist(sink raft.SnapshotSink) error {
	return nil
}

func (s snapshot) Release() {
	return
}

func (f *FSM) Apply(*raft.Log) interface{} {
	return nil
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return snapshot{}, nil
}

func (f *FSM) Restore(io.ReadCloser) error {
	return nil
}

type clusterLog struct{}

func (cl clusterLog) Write(data []byte) (n int, err error) {
	line := strings.TrimSuffix(string(data), "\n")
	slog.Infoln(line)
	return len(data), nil
}

func StartCluster(listen, dbPath string, members []string) (raftInstance *Raft, err error) {
	logFilter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel("INFO"),
		Writer:   clusterLog{},
	}
	logger := log.New(logFilter, "[cluser]", 0)

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
	memberlistConfig.LogOutput = logFilter

	serfEvents := make(chan serf.Event)
	// create hashicorp serf config
	serfConfig := serf.DefaultConfig()
	serfConfig.NodeName = listen
	serfConfig.EventCh = serfEvents
	serfConfig.MemberlistConfig = memberlistConfig
	serfConfig.LogOutput = logFilter

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

	raftPort := serfPort + 1
	raftListen := fmt.Sprintf("%s:%d", serfListen[0], raftPort)

	c := raft.DefaultConfig()
	c.LogOutput = logFilter
	c.LocalID = raft.ServerID(raftListen)

	raftInstance = &Raft{Config: c, logger: logger, Serf: serfListener, RestartWatch: make(chan bool), ReloadCluster: make(chan bool), serfEvents: serfEvents}

	err = raftInstance.CreateRaftDb(dbPath)
	if err != nil {
		return nil, errors.New("Error while init raft bold db for main cluster database: " + err.Error())
	}

	err = raftInstance.CreateSnapshotStore(dbPath)
	if err != nil {
		return nil, errors.New("Error while init snapshots store path: " + err.Error())
	}

	// start raft init
	err = raftInstance.CreateRaftTransport(raftListen)
	if err != nil {
		return nil, errors.New("Error while init Raft transport: " + err.Error())
	}

	raftInstance.Instance, err = raft.NewRaft(c, &FSM{}, raftInstance.Db, raftInstance.Db, raftInstance.Snapshots, raftInstance.Transport)
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

func isBootstrapped(instance *Raft) bool {
	cfuture := instance.Instance.GetConfiguration()
	if err := cfuture.Error(); err != nil {
		return false
	}
	configuration := cfuture.Configuration()
	if len(configuration.Servers) > 0 {
		return true
	}
	return false
}

func bootstrapCluster(instance *Raft, raftListen string, members []string) error {
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

	f := instance.Instance.BootstrapCluster(bootstrapConfig)
	return f.Error()
}
