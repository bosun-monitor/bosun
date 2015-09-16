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
	"bosun.org/cmd/bosun/expr"
	"bosun.org/collect"
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
	dbMetric           = "metric"
	dbTagk             = "tagk"
	dbTagv             = "tagv"
	dbMetricTags       = "metrictags"
	dbNotifications    = "notifications"
	dbSilence          = "silence"
	dbStatus           = "status"
	dbMetadata         = "metadata"
	dbMetricMetadata   = "metadata-metric"
	dbIncidents        = "incidents"
)

func (s *Schedule) save() {
	if s.db == nil {
		return
	}
	s.Lock("Save")
	store := map[string]interface{}{
		dbMetric:         s.Search.Read.Metric,
		dbTagk:           s.Search.Read.Tagk,
		dbTagv:           s.Search.Read.Tagv,
		dbMetricTags:     s.Search.Read.MetricTags,
		dbNotifications:  s.Notifications,
		dbSilence:        s.Silence,
		dbStatus:         s.status,
		dbMetadata:       s.Metadata,
		dbIncidents:      s.Incidents,
		dbMetricMetadata: s.metricMetadata,
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
	decode := func(name string, dst interface{}) error {
		var data []byte
		err := s.db.View(func(tx *bolt.Tx) error {
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
	if err := decode(dbMetadata, &s.Metadata); err != nil {
		slog.Errorln(dbMetadata, err)
	}
	if err := decode(dbMetricMetadata, &s.metricMetadata); err != nil {
		slog.Errorln(dbMetricMetadata, err)
	}
	for k, v := range s.Metadata {
		if k.Name == "desc" || k.Name == "rate" || k.Name == "unit" {
			s.PutMetadata(k, v.Value)
			delete(s.Metadata, k)
		}
	}
	if err := decode(dbMetric, &s.Search.Metric); err != nil {
		slog.Errorln(dbMetric, err)
	}
	if err := decode(dbTagk, &s.Search.Tagk); err != nil {
		slog.Errorln(dbTagk, err)
	}
	if err := decode(dbTagv, &s.Search.Tagv); err != nil {
		slog.Errorln(dbTagv, err)
	}
	if err := decode(dbMetricTags, &s.Search.MetricTags); err != nil {
		slog.Errorln(dbMetricTags, err)
	}
	notifications := make(map[expr.AlertKey]map[string]time.Time)
	if err := decode(dbNotifications, &notifications); err != nil {
		slog.Errorln(dbNotifications, err)
	}
	if err := decode(dbSilence, &s.Silence); err != nil {
		slog.Errorln(dbSilence, err)
	}
	if err := decode(dbIncidents, &s.Incidents); err != nil {
		slog.Errorln(dbIncidents, err)
	}

	// Calculate next incident id.
	for _, i := range s.Incidents {
		if i.Id > s.maxIncidentId {
			s.maxIncidentId = i.Id
		}
	}
	status := make(States)
	if err := decode(dbStatus, &status); err != nil {
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
		for _, e := range st.History {
			clear(e.Warn)
			clear(e.Crit)
			clear(e.Error)
		}
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

	s.Search.Copy()
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
