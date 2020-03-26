package cluster

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"path/filepath"

	"bosun.org/cmd/bosun/cluster/fsm"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule"
	promstat "bosun.org/collect/prometheus"
	"bosun.org/slog"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/logutils"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/hashicorp/serf/serf"
)

type MembersFileWatch func(*conf.SystemConf, *Raft)

type RaftClusterState interface {
	GetClusterStat()
	IsEnabled() bool
}

const (
	SERF_EVENT_CHANGE_MASTER = "change_master"
)

type EventChangeMaster struct {
	Id      string
	Address string
	Source  string
}

type Raft struct {
	Instance   *raft.Raft
	Config     *raft.Config
	logger     hclog.Logger
	Db         *raftboltdb.BoltStore
	Snapshots  *raft.FileSnapshotStore
	Transport  *raft.NetworkTransport
	Serf       *serf.Serf
	Fsm        *fsm.FSM
	serfEvents chan serf.Event
	memberList map[string]struct{}
}

func (r *Raft) IsEnabled() bool {
	if r == nil {
		return false
	}
	return true
}

func (r *Raft) IsLeader() bool {
	if r == nil {
		return true
	}

	return r.Instance.State() == raft.Leader
}

func (r *Raft) GetClusterStat() {
	if r == nil {
		promstat.ClusterMemberMode.Set(-1)
		return
	} else {
		promstat.ClusterMemberMode.Set(float64(r.Instance.State()))
	}
	promstat.ClusterLastContact.Set(float64(r.Instance.LastContact().Unix()))
	promstat.ClusterAppliedIndex.Set(float64(r.Instance.AppliedIndex()))

	future := r.Instance.GetConfiguration()
	if err := future.Error(); err != nil {
		slog.Error("could not get configuration for stats", "error", err)
	} else {
		configuration := future.Configuration()
		promstat.ClusterLatestConfigurationIndex.Set(float64(future.Index()))

		// This is a legacy metric that we've seen people use in the wild.
		hasUs := false
		numPeers := float64(0)
		for _, server := range configuration.Servers {
			if server.Suffrage == raft.Voter {
				if server.ID == r.Config.LocalID {
					hasUs = true
				} else {
					numPeers++
				}
			}
		}
		if !hasUs {
			numPeers = 0
		}
		promstat.ClusterPeersCount.Set(numPeers)
	}
}

func (r *Raft) Apply(cmd *fsm.ClusterCommand, timeout time.Duration) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	f := r.Instance.Apply(data, timeout)
	return f.Error()
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

func (r *Raft) readMembersFile(cfg string) error {
	members, err := readMembersFile(cfg)
	if err != nil {
		return err
	}
	membersMap := make(map[string]struct{}, 0)
	for _, member := range members {
		membersMap[member] = struct{}{}
	}

	if len(membersMap) != len(r.memberList) {
		r.memberList = membersMap
	} else {
		for m := range membersMap {
			if _, ok := r.memberList[m]; !ok {
				r.memberList = membersMap
				break
			}
		}
	}
	return nil
}

func (r *Raft) Watch() {
	for {
		select {
		case <-r.Instance.LeaderCh():
			promstat.ClusterFailovers.Inc()
		case ev := <-r.serfEvents:
			slog.Infof("Received cluster event: %#v.", ev)

			leader := r.Instance.VerifyLeader()
			if memberEvent, ok := ev.(serf.MemberEvent); ok {
				for _, member := range memberEvent.Members {
					changedPeer := member.Addr.String() + ":" + strconv.Itoa(int(member.Port+1))
					slog.Infof("Received cluster event %s: %#v. member: %s", memberEvent.EventType().String(), ev, changedPeer)
					if memberEvent.EventType() == serf.EventMemberJoin {
						if leader.Error() == nil {
							f := r.Instance.AddVoter(raft.ServerID(member.Name), raft.ServerAddress(changedPeer), 0, 0)
							if f.Error() != nil {
								slog.Errorf("error adding voter: %s", f.Error())
							}
						}
					} else if memberEvent.EventType() == serf.EventMemberLeave || memberEvent.EventType() == serf.EventMemberFailed || memberEvent.EventType() == serf.EventMemberReap {
						if leader.Error() == nil {
							f := r.Instance.RemoveServer(raft.ServerID(member.Name), 0, 0)
							if f.Error() != nil {
								slog.Errorf("error removing server: %s", f.Error())
							}
						}
					}
				}
			} else if ev.EventType() == serf.EventUser {
				query := ev.(serf.UserEvent)
				if query.Name == SERF_EVENT_CHANGE_MASTER && r.Instance.State() == raft.Leader {
					var eventChangeMaster EventChangeMaster
					err := json.Unmarshal(query.Payload, &eventChangeMaster)
					if err != nil {
						slog.Errorf("Invalid serf user event change_master payload: %v: %v", string(query.Payload), err)
						continue
					}
					leader := r.Instance.VerifyLeader()

					if leader.Error() == nil {
						f := r.Instance.LeadershipTransferToServer(raft.ServerID(eventChangeMaster.Id), raft.ServerAddress(eventChangeMaster.Address))
						if f.Error() != nil {
							slog.Errorf("error while change leader to node: id %s, address: %s, err: %s", eventChangeMaster.Id, eventChangeMaster.Address, f.Error())
						} else {
							slog.Infof("leader was changed by change_master user event to node: id %s, address: %s", eventChangeMaster.Id, eventChangeMaster.Address)
						}
					} else {
						slog.Errorf("error while check leader: %v", leader.Error())
					}
				} else {
					slog.Errorf("Invalid serf user event: %v", query)
				}
			}
		}
	}
}

type clusterLog struct{}

func (cl clusterLog) Write(data []byte) (n int, err error) {
	line := strings.TrimSuffix(string(data), "\n")
	slog.Infoln(line)
	return len(data), nil
}

func StartCluster(systemConf *conf.SystemConf, setRules func(*rule.Conf), clusterReload func(*rule.Conf) error) (raftInstance *Raft, err error) {
	logFilter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel("INFO"),
		Writer:   clusterLog{},
	}
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "bosun",
		Level: hclog.LevelFromString("INFO"),
	})

	nodeID := systemConf.GetClusterNodeID()
	if nodeID == "" {
		nodeID = systemConf.GetClusterBindAddress()
	}

	serfListen := strings.Split(systemConf.GetClusterBindAddress(), ":")
	if len(serfListen) != 2 {
		return nil, errors.New("Incorrect serf listen address: " + systemConf.GetClusterBindAddress())
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
	serfConfig.NodeName = nodeID
	serfConfig.EventCh = serfEvents
	serfConfig.MemberlistConfig = memberlistConfig
	serfConfig.LogOutput = logFilter

	serfListener, err := serf.Create(serfConfig)
	if err != nil {
		return nil, errors.New("Error while init serf listener: " + err.Error())
	}

	var membersList []string
	if systemConf.GetClusterMembersFile() != "" {
		membersList, err = readMembersFile(systemConf.GetClusterMembersFile())
		if err != nil {
			return nil, errors.New("Error while read members list config file: " + err.Error())
		}
	} else if len(systemConf.GetClusterMembers()) > 0 {
		membersList = systemConf.GetClusterMembers()
	}
	if len(membersList) > 0 {
		if _, err := serfListener.Join(membersList, true); err != nil {
			return nil, errors.New("Error join cluster: " + err.Error())
		}
	}

	if _, err := os.Stat(systemConf.GetClusterMetadataStorePath()); os.IsNotExist(err) {
		slog.Infoln("Create cluster metadata path", systemConf.GetClusterMetadataStorePath())
		if err := os.MkdirAll(systemConf.GetClusterMetadataStorePath(), 0755); err != nil {
			return nil, errors.New("Error while create raft metadata path for cluster: " + err.Error())
		}
	}

	raftPort := serfPort + 1
	raftListen := fmt.Sprintf("%s:%d", serfListen[0], raftPort)

	c := raft.DefaultConfig()
	c.LogOutput = logFilter
	c.LocalID = raft.ServerID(nodeID)
	c.HeartbeatTimeout = systemConf.ClusterHeartbeatTimeout()
	c.LeaderLeaseTimeout = systemConf.ClusterLeaderLeaseTimeout()
	c.ElectionTimeout = systemConf.ClusterElectionTimeout()
	c.SnapshotInterval = systemConf.ClusterSnapshotInterval()
	c.TrailingLogs = 1
	c.SnapshotThreshold = 1

	sysProvider, err := systemConf.GetSystemConfProvider()
	if err != nil {
		slog.Fatalf("Error while get system conf provider: %v", err)
	}

	raftInstance = &Raft{
		Config:     c,
		logger:     logger,
		Serf:       serfListener,
		serfEvents: serfEvents,
		Fsm:        &fsm.FSM{Reload: clusterReload, SysProvider: sysProvider, SetRules: setRules},
	}

	err = raftInstance.CreateRaftDb(systemConf.GetClusterMetadataStorePath())
	if err != nil {
		return nil, errors.New("Error while init raft bold db for main cluster database: " + err.Error())
	}

	err = raftInstance.CreateSnapshotStore(systemConf.GetClusterMetadataStorePath())
	if err != nil {
		return nil, errors.New("Error while init snapshots store path: " + err.Error())
	}

	// start raft init
	err = raftInstance.CreateRaftTransport(raftListen)
	if err != nil {
		return nil, errors.New("Error while init Raft transport: " + err.Error())
	}

	raftInstance.Instance, err = raft.NewRaft(c, raftInstance.Fsm, raftInstance.Db, raftInstance.Db, raftInstance.Snapshots, raftInstance.Transport)
	if err != nil {
		return nil, errors.New("Error while create raft instance: " + err.Error())
	}

	if !isBootstrapped(raftInstance) {
		if err := bootstrapCluster(raftInstance, nodeID, raftListen, systemConf.GetClusterMembers()); err != nil {
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

func readMembersFile(cfg string) ([]string, error) {
	membersList := make([]string, 0)
	membersFile, err := os.Open(cfg)
	if err != nil {
		return nil, err
	}
	defer membersFile.Close()

	scanner := bufio.NewScanner(membersFile)
	for scanner.Scan() {
		member := strings.TrimSpace(scanner.Text())
		if member != "" && !strings.HasPrefix(member, "#") {
			slog.Infof("Cluster configuration. Member initial list file: %s. Member: %s", cfg, member)
			membersList = append(membersList, member)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return membersList, nil
}

func bootstrapCluster(instance *Raft, nodeID, raftListen string, members []string) error {
	bootstrapConfig := raft.Configuration{
		Servers: []raft.Server{
			{
				Suffrage: raft.Voter,
				ID:       raft.ServerID(nodeID),
				Address:  raft.ServerAddress(raftListen),
			},
		},
	}

	f := instance.Instance.BootstrapCluster(bootstrapConfig)
	return f.Error()
}
