package tsdb

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"bosun.org/_third_party/github.com/influxdb/influxdb/influxql"
	"bosun.org/_third_party/github.com/influxdb/influxdb/models"
)

func NewStore(path string) *Store {
	opts := NewEngineOptions()
	opts.Config = NewConfig()

	return &Store{
		path:          path,
		EngineOptions: opts,
		Logger:        log.New(os.Stderr, "[store] ", log.LstdFlags),
	}
}

var (
	ErrShardNotFound = fmt.Errorf("shard not found")
)

type Store struct {
	mu   sync.RWMutex
	path string

	databaseIndexes map[string]*DatabaseIndex
	shards          map[uint64]*Shard

	EngineOptions EngineOptions
	Logger        *log.Logger
	closing       chan struct{}
}

// Path returns the store's root path.
func (s *Store) Path() string { return s.path }

// DatabaseIndexN returns the number of databases indicies in the store.
func (s *Store) DatabaseIndexN() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.databaseIndexes)
}

// Shard returns a shard by id.
func (s *Store) Shard(id uint64) *Shard {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shards[id]
}

// ShardN returns the number of shard in the store.
func (s *Store) ShardN() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.shards)
}

func (s *Store) CreateShard(database, retentionPolicy string, shardID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.closing:
		return fmt.Errorf("closing")
	default:
	}

	// shard already exists
	if _, ok := s.shards[shardID]; ok {
		return nil
	}

	// created the db and retention policy dirs if they don't exist
	if err := os.MkdirAll(filepath.Join(s.path, database, retentionPolicy), 0700); err != nil {
		return err
	}

	// create the WAL directory
	walPath := filepath.Join(s.EngineOptions.Config.WALDir, database, retentionPolicy, fmt.Sprintf("%d", shardID))
	if err := os.MkdirAll(walPath, 0700); err != nil {
		return err
	}

	// create the database index if it does not exist
	db, ok := s.databaseIndexes[database]
	if !ok {
		db = NewDatabaseIndex()
		s.databaseIndexes[database] = db
	}

	shardPath := filepath.Join(s.path, database, retentionPolicy, strconv.FormatUint(shardID, 10))
	shard := NewShard(shardID, db, shardPath, walPath, s.EngineOptions)
	if err := shard.Open(); err != nil {
		return err
	}

	s.shards[shardID] = shard

	return nil
}

// DeleteShard removes a shard from disk.
func (s *Store) DeleteShard(shardID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// ensure shard exists
	sh, ok := s.shards[shardID]
	if !ok {
		return nil
	}

	if err := sh.Close(); err != nil {
		return err
	}

	if err := os.Remove(sh.path); err != nil {
		return err
	}

	if err := os.RemoveAll(sh.walPath); err != nil {
		return err
	}

	delete(s.shards, shardID)

	return nil
}

// DeleteDatabase will close all shards associated with a database and remove the directory and files from disk.
func (s *Store) DeleteDatabase(name string, shardIDs []uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range shardIDs {
		shard := s.shards[id]
		if shard != nil {
			shard.Close()
		}
	}
	if err := os.RemoveAll(filepath.Join(s.path, name)); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(s.EngineOptions.Config.WALDir, name)); err != nil {
		return err
	}
	delete(s.databaseIndexes, name)
	return nil
}

// ShardIDs returns a slice of all ShardIDs under management.
func (s *Store) ShardIDs() []uint64 {
	ids := make([]uint64, 0, len(s.shards))
	for i, _ := range s.shards {
		ids = append(ids, i)
	}
	return ids
}

func (s *Store) ValidateAggregateFieldsInStatement(shardID uint64, measurementName string, stmt *influxql.SelectStatement) error {
	s.mu.RLock()
	shard := s.shards[shardID]
	s.mu.RUnlock()
	if shard == nil {
		return ErrShardNotFound
	}
	return shard.ValidateAggregateFieldsInStatement(measurementName, stmt)
}

func (s *Store) DatabaseIndex(name string) *DatabaseIndex {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.databaseIndexes[name]
}

// Databases returns all the databases in the indexes
func (s *Store) Databases() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	databases := []string{}
	for db := range s.databaseIndexes {
		databases = append(databases, db)
	}
	return databases
}

func (s *Store) Measurement(database, name string) *Measurement {
	s.mu.RLock()
	db := s.databaseIndexes[database]
	s.mu.RUnlock()
	if db == nil {
		return nil
	}
	return db.Measurement(name)
}

// DiskSize returns the size of all the shard files in bytes.  This size does not include the WAL size.
func (s *Store) DiskSize() (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var size int64
	for _, shardID := range s.ShardIDs() {
		shard := s.Shard(shardID)
		sz, err := shard.DiskSize()
		if err != nil {
			return 0, err
		}
		size += sz
	}
	return size, nil
}

// deleteSeries loops through the local shards and deletes the series data and metadata for the passed in series keys
func (s *Store) deleteSeries(keys []string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sh := range s.shards {
		if err := sh.DeleteSeries(keys); err != nil {
			return err
		}
	}
	return nil
}

// deleteMeasurement loops through the local shards and removes the measurement field encodings from each shard
func (s *Store) deleteMeasurement(name string, seriesKeys []string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sh := range s.shards {
		if err := sh.DeleteMeasurement(name, seriesKeys); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) loadIndexes() error {
	dbs, err := ioutil.ReadDir(s.path)
	if err != nil {
		return err
	}
	for _, db := range dbs {
		if !db.IsDir() {
			s.Logger.Printf("Skipping database dir: %s. Not a directory", db.Name())
			continue
		}
		s.databaseIndexes[db.Name()] = NewDatabaseIndex()
	}
	return nil
}

func (s *Store) loadShards() error {
	// loop through the current database indexes
	for db := range s.databaseIndexes {
		rps, err := ioutil.ReadDir(filepath.Join(s.path, db))
		if err != nil {
			return err
		}

		for _, rp := range rps {
			// retention policies should be directories.  Skip anything that is not a dir.
			if !rp.IsDir() {
				s.Logger.Printf("Skipping retention policy dir: %s. Not a directory", rp.Name())
				continue
			}

			shards, err := ioutil.ReadDir(filepath.Join(s.path, db, rp.Name()))
			if err != nil {
				return err
			}
			for _, sh := range shards {
				path := filepath.Join(s.path, db, rp.Name(), sh.Name())
				walPath := filepath.Join(s.EngineOptions.Config.WALDir, db, rp.Name(), sh.Name())

				// Shard file names are numeric shardIDs
				shardID, err := strconv.ParseUint(sh.Name(), 10, 64)
				if err != nil {
					s.Logger.Printf("Skipping shard: %s. Not a valid path", rp.Name())
					continue
				}

				shard := NewShard(shardID, s.databaseIndexes[db], path, walPath, s.EngineOptions)
				err = shard.Open()
				if err != nil {
					return fmt.Errorf("failed to open shard %d: %s", shardID, err)
				}
				s.shards[shardID] = shard
			}
		}
	}
	return nil

}

func (s *Store) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closing = make(chan struct{})

	s.shards = map[uint64]*Shard{}
	s.databaseIndexes = map[string]*DatabaseIndex{}

	s.Logger.Printf("Using data dir: %v", s.Path())

	// Create directory.
	if err := os.MkdirAll(s.path, 0777); err != nil {
		return err
	}

	// TODO: Start AE for Node
	if err := s.loadIndexes(); err != nil {
		return err
	}

	if err := s.loadShards(); err != nil {
		return err
	}

	return nil
}

func (s *Store) WriteToShard(shardID uint64, points []models.Point) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sh, ok := s.shards[shardID]
	if !ok {
		return ErrShardNotFound
	}

	return sh.WritePoints(points)
}

func (s *Store) CreateMapper(shardID uint64, stmt influxql.Statement, chunkSize int) (Mapper, error) {
	shard := s.Shard(shardID)

	switch stmt := stmt.(type) {
	case *influxql.SelectStatement:
		if (stmt.IsRawQuery && !stmt.HasDistinct()) || stmt.IsSimpleDerivative() {
			m := NewRawMapper(shard, stmt)
			m.ChunkSize = chunkSize
			return m, nil
		}
		return NewAggregateMapper(shard, stmt), nil

	case *influxql.ShowMeasurementsStatement:
		m := NewShowMeasurementsMapper(shard, stmt)
		m.ChunkSize = chunkSize
		return m, nil
	case *influxql.ShowTagKeysStatement:
		return NewShowTagKeysMapper(shard, stmt, chunkSize), nil
	default:
		return nil, fmt.Errorf("can't create mapper for statement type: %T", stmt)
	}
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sh := range s.shards {
		if err := sh.Close(); err != nil {
			return err
		}
	}
	if s.closing != nil {
		close(s.closing)
	}
	s.closing = nil
	s.shards = nil
	s.databaseIndexes = nil

	return nil
}

// IsRetryable returns true if this error is temporary and could be retried
func IsRetryable(err error) bool {
	if err == nil {
		return true
	}

	if strings.Contains(err.Error(), "field type conflict") {
		return false
	}
	return true
}
