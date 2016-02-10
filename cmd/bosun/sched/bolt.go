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
	"bosun.org/models"
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
)

func (s *Schedule) save() {
	if s.db == nil {
		return
	}
	s.Lock("Save")
	store := map[string]interface{}{
		dbNotifications: s.Notifications,
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
	if len(data) == 0 {
		return nil
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
	notifications := make(map[models.AlertKey]map[string]time.Time)
	if err := decode(db, dbNotifications, &notifications); err != nil {
		slog.Errorln(dbNotifications, err)
	}
	s.Notifications = notifications

	//status := make(States)
	//	if err := decode(db, dbStatus, &status); err != nil {
	//		slog.Errorln(dbStatus, err)
	//	}
	//	clear := func(r *models.Result) {
	//		if r == nil {
	//			return
	//		}
	//		r.Computations = nil
	//}
	//TODO: ???
	//	for ak, st := range status {
	//		a, present := s.Conf.Alerts[ak.Name()]
	//		if !present {
	//			slog.Errorln("sched: alert no longer present, ignoring:", ak)
	//			continue
	//		} else if s.Conf.Squelched(a, st.Group) {
	//			slog.Infoln("sched: alert now squelched:", ak)
	//			continue
	//		} else {
	//			t := a.Unknown
	//			if t == 0 {
	//				t = s.Conf.CheckFrequency
	//			}
	//			if t == 0 && st.Last().Status == StUnknown {
	//				st.Append(&Event{Status: StNormal, IncidentId: st.Last().IncidentId})
	//			}
	//		}
	//		clear(st.Result)
	//		newHistory := []Event{}
	//		for _, e := range st.History {
	//			clear(e.Warn)
	//			clear(e.Crit)
	//			// Remove error events which no longer are a thing.
	//			if e.Status <= StUnknown {
	//				newHistory = append(newHistory, e)
	//			}
	//		}
	//		st.History = newHistory
	//		s.status[ak] = st
	//		if a.Log && st.Open {
	//			st.Open = false
	//			slog.Infof("sched: alert %s is now log, closing, was %s", ak, st.Status())
	//		}
	//	for name, t := range notifications[ak] {
	//		n, present := s.Conf.Notifications[name]
	//		if !present {
	//			slog.Infoln("sched: notification not present during restore:", name)
	//			continue
	//		}
	//		if a.Log {
	//			slog.Infoln("sched: alert is now log, removing notification:", ak)
	//			continue
	//		}
	//		s.AddNotification(ak, n, t)
	//	}
	//}
	if err := migrateOldDataToRedis(db, s.DataAccess); err != nil {
		return err
	}
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
	if err := migrateSilence(db, data); err != nil {
		return err
	}
	if err := migrateState(db, data); err != nil {
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
		if err = setMigrated(db, "search"); err != nil {
			return err
		}
	}
	return nil
}

func migrateSilence(db *bolt.DB, data database.DataAccess) error {
	migrated, err := isMigrated(db, "silence")
	if err != nil {
		return err
	}
	if migrated {
		return nil
	}
	slog.Info("migrating silence")
	silence := map[string]*models.Silence{}
	if err := decode(db, "silence", &silence); err != nil {
		return err
	}
	for _, v := range silence {
		v.TagString = v.Tags.Tags()
		data.Silence().AddSilence(v)
	}
	if err = setMigrated(db, "silence"); err != nil {
		return err
	}
	return nil
}

func migrateState(db *bolt.DB, data database.DataAccess) error {
	migrated, err := isMigrated(db, "state")
	if err != nil {
		return err
	}
	if migrated {
		return nil
	}
	//redefine the structs as they were when we gob encoded them
	type Result struct {
		*expr.Result
		Expr string
	}
	mResult := func(r *Result) *models.Result {
		if r == nil || r.Result == nil {
			return &models.Result{}
		}
		v, _ := valueToFloat(r.Result.Value)
		return &models.Result{
			Computations: r.Result.Computations,
			Value:        models.Float(v),
			Expr:         r.Expr,
		}
	}
	type Event struct {
		Warn, Crit  *Result
		Status      models.Status
		Time        time.Time
		Unevaluated bool
		IncidentId  uint64
	}
	type State struct {
		*Result
		History      []Event
		Actions      []models.Action
		Touched      time.Time
		Alert        string
		Tags         string
		Group        opentsdb.TagSet
		Subject      string
		Body         string
		EmailBody    []byte
		EmailSubject []byte
		Attachments  []*models.Attachment
		NeedAck      bool
		Open         bool
		Forgotten    bool
		Unevaluated  bool
		LastLogTime  time.Time
	}
	type OldStates map[models.AlertKey]*State
	slog.Info("migrating state")
	states := OldStates{}
	if err := decode(db, "status", &states); err != nil {
		return err
	}
	for ak, state := range states {
		if len(state.History) == 0 {
			continue
		}
		var thisId uint64
		events := []Event{}
		addIncident := func(saveBody bool) error {
			if thisId == 0 || len(events) == 0 || state == nil {
				return nil
			}
			incident := NewIncident(ak)
			incident.Expr = state.Expr

			incident.NeedAck = state.NeedAck
			incident.Open = state.Open
			incident.Result = mResult(state.Result)
			incident.Unevaluated = state.Unevaluated
			incident.Start = events[0].Time
			incident.Id = int64(thisId)
			incident.Subject = state.Subject
			if saveBody {
				incident.Body = state.Body
			}
			for _, ev := range events {
				incident.CurrentStatus = ev.Status
				mEvent := models.Event{
					Crit:        mResult(ev.Crit),
					Status:      ev.Status,
					Time:        ev.Time,
					Unevaluated: ev.Unevaluated,
					Warn:        mResult(ev.Warn),
				}
				incident.Events = append(incident.Events, mEvent)
				if ev.Status > incident.WorstStatus {
					incident.WorstStatus = ev.Status
				}
				if ev.Status > models.StNormal {
					incident.LastAbnormalStatus = ev.Status
					incident.LastAbnormalTime = ev.Time.UTC().Unix()
				}
			}
			for _, ac := range state.Actions {
				if ac.Time.Before(incident.Start) {
					continue
				}
				incident.Actions = append(incident.Actions, ac)
				if ac.Time.After(incident.Events[len(incident.Events)-1].Time) && ac.Type == models.ActionClose {
					incident.End = &ac.Time
					break
				}
			}
			if err := data.State().ImportIncidentState(incident); err != nil {
				return err
			}
			return nil
		}
		//essentially a rle algorithm to assign events to incidents
		for _, e := range state.History {
			if e.Status > models.StUnknown {
				continue
			}
			if e.IncidentId == 0 {
				//include all non-assigned incidents up to the next non-match
				events = append(events, e)
				continue
			}
			if thisId == 0 {
				thisId = e.IncidentId
				events = append(events, e)
			}
			if e.IncidentId != thisId {
				if err := addIncident(false); err != nil {
					return err
				}
				thisId = e.IncidentId
				events = []Event{e}

			} else {
				events = append(events, e)
			}
		}
		if err := addIncident(true); err != nil {
			return err
		}
	}
	if err = setMigrated(db, "state"); err != nil {
		return err
	}
	return nil
}
func isMigrated(db *bolt.DB, name string) (bool, error) {
	found := false
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(dbBucket))
		if b == nil {
			found = true
			return nil
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
