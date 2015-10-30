package sched

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"bosun.org/_third_party/github.com/boltdb/bolt"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/database"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

func (s *Schedule) performSave() {
	for {
		time.Sleep(60 * 10 * time.Second) // wait 10 minutes to throttle.
		s.save()
	}
}

type counterWriter struct {
	written int
	w       io.Writer
}

func (c *counterWriter) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	c.written += n
	return n, err
}

const (
	dbBucket           = "bindata"
	dbConfigTextBucket = "configText"
	dbNotifications    = "notifications"
	dbSilence          = "silence"
	dbStatus           = "status"
	dbIncidents        = "incidents"
	dbErrors           = "errors"
)

func (s *Schedule) save() {
	if s.db == nil {
		return
	}
	s.Lock("Save")
	store := map[string]interface{}{
		dbNotifications: s.Notifications,
		dbSilence:       s.Silence,
		dbStatus:        s.status,
		dbIncidents:     s.Incidents,
	}
	tostore := make(map[string][]byte)
	for name, data := range store {
		f := new(bytes.Buffer)
		gz := gzip.NewWriter(f)
		cw := &counterWriter{w: gz}
		enc := gob.NewEncoder(cw)
		if err := enc.Encode(data); err != nil {
			slog.Errorf("error saving %s: %v", name, err)
			s.Unlock()
			return
		}
		if err := gz.Flush(); err != nil {
			slog.Errorf("gzip flush error saving %s: %v", name, err)
		}
		if err := gz.Close(); err != nil {
			slog.Errorf("gzip close error saving %s: %v", name, err)
		}
		tostore[name] = f.Bytes()
		slog.Infof("wrote %s: %v", name, conf.ByteSize(cw.written))
		collect.Put("statefile.size", opentsdb.TagSet{"object": name}, cw.written)
	}
	s.Unlock()
	err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(dbBucket))
		if err != nil {
			return err
		}
		for name, data := range tostore {
			if err := b.Put([]byte(name), data); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Errorf("save db update error: %v", err)
		return
	}
	fi, err := os.Stat(s.Conf.StateFile)
	if err == nil {
		collect.Put("statefile.size", opentsdb.TagSet{"object": "total"}, fi.Size())
	}
	slog.Infoln("save to db complete")
}

func decode(db *bolt.DB, name string, dst interface{}) error {
	var data []byte
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(dbBucket))
		if b == nil {
			return fmt.Errorf("unknown bucket: %v", dbBucket)
		}
		data = b.Get([]byte(name))
		return nil
	})
	if err != nil {
		return err
	}
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gr.Close()
	return gob.NewDecoder(gr).Decode(dst)
}

// RestoreState restores notification and alert state from the file on disk.
func (s *Schedule) RestoreState() error {
	defer func() {
		bosunStartupTime = time.Now()
	}()
	slog.Infoln("RestoreState")
	start := time.Now()
	s.Lock("RestoreState")
	defer s.Unlock()
	s.Search.Lock()
	defer s.Search.Unlock()

	s.Notifications = nil
	db := s.db
	notifications := make(map[expr.AlertKey]map[string]time.Time)
	if err := decode(db, dbNotifications, &notifications); err != nil {
		slog.Errorln(dbNotifications, err)
	}
	if err := decode(db, dbSilence, &s.Silence); err != nil {
		slog.Errorln(dbSilence, err)
	}
	if err := decode(db, dbIncidents, &s.Incidents); err != nil {
		slog.Errorln(dbIncidents, err)
	}

	// Calculate next incident id.
	for _, i := range s.Incidents {
		if i.Id > s.maxIncidentId {
			s.maxIncidentId = i.Id
		}
	}
	status := make(States)
	if err := decode(db, dbStatus, &status); err != nil {
		slog.Errorln(dbStatus, err)
	}
	clear := func(r *Result) {
		if r == nil {
			return
		}
		r.Computations = nil
	}
	for ak, st := range status {
		a, present := s.Conf.Alerts[ak.Name()]
		if !present {
			slog.Errorln("sched: alert no longer present, ignoring:", ak)
			continue
		} else if s.Conf.Squelched(a, st.Group) {
			slog.Infoln("sched: alert now squelched:", ak)
			continue
		} else {
			t := a.Unknown
			if t == 0 {
				t = s.Conf.CheckFrequency
			}
			if t == 0 && st.Last().Status == StUnknown {
				st.Append(&Event{Status: StNormal, IncidentId: st.Last().IncidentId})
			}
		}
		clear(st.Result)
		newHistory := []Event{}
		for _, e := range st.History {
			clear(e.Warn)
			clear(e.Crit)
			// Remove error events which no longer are a thing.
			if e.Status <= StUnknown {
				newHistory = append(newHistory, e)
			}
		}
		st.History = newHistory
		s.status[ak] = st
		if a.Log && st.Open {
			st.Open = false
			slog.Infof("sched: alert %s is now log, closing, was %s", ak, st.Status())
		}
		for name, t := range notifications[ak] {
			n, present := s.Conf.Notifications[name]
			if !present {
				slog.Infoln("sched: notification not present during restore:", name)
				continue
			}
			if a.Log {
				slog.Infoln("sched: alert is now log, removing notification:", ak)
				continue
			}
			s.AddNotification(ak, n, t)
		}
	}
	if s.maxIncidentId == 0 {
		s.createHistoricIncidents()
	}
	migrateOldDataToRedis(db, s.DataAccess)
	// delete metrictags if they exist.
	deleteKey(s.db, "metrictags")
	slog.Infoln("RestoreState done in", time.Since(start))
	return nil
}

type storedConfig struct {
	Text     string
	LastUsed time.Time
}

// Saves the provided config text in state file for later access.
// Returns a hash of the file to be used as a retreival key.
func (s *Schedule) SaveTempConfig(text string) (hash string, err error) {
	sig := md5.Sum([]byte(text))
	b64 := base64.StdEncoding.EncodeToString(sig[0:5])
	data := storedConfig{Text: text, LastUsed: time.Now()}
	bindata, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	err = s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(dbConfigTextBucket))
		if err != nil {
			return err
		}
		return b.Put([]byte(b64), bindata)
	})
	if err != nil {
		return "", err
	}
	return b64, nil
}

// Retreive the specified config text from state file.
func (s *Schedule) LoadTempConfig(hash string) (text string, err error) {
	config := storedConfig{}
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(dbConfigTextBucket))
		data := b.Get([]byte(hash))
		if data == nil || len(data) == 0 {
			return fmt.Errorf("Config text '%s' not found", hash)
		}
		return json.Unmarshal(data, &config)
	})
	if err != nil {
		return "", err
	}
	go s.SaveTempConfig(config.Text) //refresh timestamp.
	return config.Text, nil
}

func (s *Schedule) GetStateFileBackup() ([]byte, error) {
	buf := bytes.Buffer{}
	err := s.db.View(func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(&buf)
		return err
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func migrateOldDataToRedis(db *bolt.DB, data database.DataAccess) error {
	if err := migrateMetricMetadata(db, data); err != nil {
		return err
	}
	if err := migrateTagMetadata(db, data); err != nil {
		return err
	}
	if err := migrateSearch(db, data); err != nil {
		return err
	}
	return nil
}

func migrateMetricMetadata(db *bolt.DB, data database.DataAccess) error {
	migrated, err := isMigrated(db, "metadata-metric")
	if err != nil {
		return err
	}
	if !migrated {
		slog.Info("Migrating metric metadata to new database format")
		type MetadataMetric struct {
			Unit        string `json:",omitempty"`
			Type        string `json:",omitempty"`
			Description string
		}
		mms := map[string]*MetadataMetric{}
		if err := decode(db, "metadata-metric", &mms); err == nil {
			for name, mm := range mms {
				if mm.Description != "" {
					err = data.Metadata().PutMetricMetadata(name, "desc", mm.Description)
					if err != nil {
						return err
					}
				}
				if mm.Unit != "" {
					err = data.Metadata().PutMetricMetadata(name, "unit", mm.Unit)
					if err != nil {
						return err
					}
				}
				if mm.Type != "" {
					err = data.Metadata().PutMetricMetadata(name, "rate", mm.Type)
					if err != nil {
						return err
					}
				}
			}
			err = setMigrated(db, "metadata-metric")
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func migrateTagMetadata(db *bolt.DB, data database.DataAccess) error {
	migrated, err := isMigrated(db, "metadata")
	if err != nil {
		return err
	}
	if !migrated {
		slog.Info("Migrating metadata to new database format")
		type Metavalue struct {
			Time  time.Time
			Value interface{}
		}
		metadata := make(map[metadata.Metakey]*Metavalue)
		if err := decode(db, "metadata", &metadata); err == nil {
			for k, v := range metadata {
				err = data.Metadata().PutTagMetadata(k.TagSet(), k.Name, fmt.Sprint(v.Value), v.Time)
				if err != nil {
					return err
				}
			}
			err = deleteKey(db, "metadata")
			if err != nil {
				return err
			}
		}
		err = setMigrated(db, "metadata")
		if err != nil {
			return err
		}
	}
	return nil
}

func migrateSearch(db *bolt.DB, data database.DataAccess) error {
	migrated, err := isMigrated(db, "search")
	if err != nil {
		return err
	}
	if !migrated {
		slog.Info("Migrating Search data to new database format")
		type duple struct{ A, B string }
		type present map[string]int64
		type qmap map[duple]present
		type smap map[string]present

		metric := qmap{}
		if err := decode(db, "metric", &metric); err == nil {
			for k, v := range metric {
				for metric, time := range v {
					data.Search().AddMetricForTag(k.A, k.B, metric, time)
				}
			}
		} else {
			return err
		}
		tagk := smap{}
		if err := decode(db, "tagk", &tagk); err == nil {
			for metric, v := range tagk {
				for tk, time := range v {
					data.Search().AddTagKeyForMetric(metric, tk, time)
				}
				data.Search().AddMetric(metric, time.Now().Unix())
			}
		} else {
			return err
		}

		tagv := qmap{}
		if err := decode(db, "tagv", &tagv); err == nil {
			for k, v := range tagv {
				for val, time := range v {
					data.Search().AddTagValue(k.A, k.B, val, time)
					data.Search().AddTagValue(database.Search_All, k.B, val, time)
				}
			}
		} else {
			return err
		}
		err = setMigrated(db, "search")
		if err != nil {
			return err
		}
	}
	return nil
}

func isMigrated(db *bolt.DB, name string) (bool, error) {
	found := false
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(dbBucket))
		if b == nil {
			return fmt.Errorf("unknown bucket: %v", dbBucket)
		}
		if dat := b.Get([]byte("isMigrated:" + name)); dat != nil {
			found = true
		}
		return nil
	})
	return found, err
}

func setMigrated(db *bolt.DB, name string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(dbBucket))
		if b == nil {
			return fmt.Errorf("unknown bucket: %v", dbBucket)
		}
		return b.Put([]byte("isMigrated:"+name), []byte{1})
	})
}
func deleteKey(db *bolt.DB, name string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(dbBucket))
		if b == nil {
			return fmt.Errorf("unknown bucket: %v", dbBucket)
		}
		return b.Delete([]byte(name))
	})
}
