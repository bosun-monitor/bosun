package database

import (
	"encoding/json"
	"fmt"
	"time"

	"bosun.org/_third_party/github.com/garyburd/redigo/redis"
	"bosun.org/collect"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

/*
incidentById:{id} - json encoded state. Authoritative source.

lastTouched:{alert} - ZSET of alert key to last touched time stamp
unknown:{alert} - Set of unknown alert keys for alert
unevel:{alert} - Set of unevaluated alert keys for alert

openIncidents - Hash of open incident Ids. Alert Key -> incident id
incidents:{ak} - List of incidents for alert key

allIncidents - List of all incidents ever. Value is "incidentId:timestamp:ak"
*/

const (
	statesOpenIncidentsKey = "openIncidents"
)

func statesLastTouchedKey(alert string) string {
	return fmt.Sprintf("lastTouched:%s", alert)
}
func statesUnknownKey(alert string) string {
	return fmt.Sprintf("unknown:%s", alert)
}
func statesUnevalKey(alert string) string {
	return fmt.Sprintf("uneval:%s", alert)
}
func incidentStateKey(id int64) string {
	return fmt.Sprintf("incidentById:%d", id)
}
func incidentsForAlertKeyKey(ak models.AlertKey) string {
	return fmt.Sprintf("incidents:%s", ak)
}

type StateDataAccess interface {
	TouchAlertKey(ak models.AlertKey, t time.Time) error
	GetUntouchedSince(alert string, time int64) ([]models.AlertKey, error)

	GetOpenIncident(ak models.AlertKey) (*models.IncidentState, error)
	GetLatestIncident(ak models.AlertKey) (*models.IncidentState, error)
	GetAllOpenIncidents() ([]*models.IncidentState, error)
	GetIncidentState(incidentId int64) (*models.IncidentState, error)
	GetAllIncidents(ak models.AlertKey) ([]*models.IncidentState, error)

	UpdateIncidentState(s *models.IncidentState) error
	ImportIncidentState(s *models.IncidentState) error

	Forget(ak models.AlertKey) error
	SetUnevaluated(ak models.AlertKey, uneval bool) error
	GetUnknownAndUnevalAlertKeys(alert string) ([]models.AlertKey, []models.AlertKey, error)
}

func (d *dataAccess) State() StateDataAccess {
	return d
}

func (d *dataAccess) TouchAlertKey(ak models.AlertKey, t time.Time) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "TouchAlertKey"})()
	conn := d.GetConnection()
	defer conn.Close()

	_, err := conn.Do("ZADD", statesLastTouchedKey(ak.Name()), t.UTC().Unix(), string(ak))
	return slog.Wrap(err)
}

func (d *dataAccess) GetUntouchedSince(alert string, time int64) ([]models.AlertKey, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetUntouchedSince"})()
	conn := d.GetConnection()
	defer conn.Close()

	results, err := redis.Strings(conn.Do("ZRANGEBYSCORE", statesLastTouchedKey(alert), "-inf", time))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	aks := make([]models.AlertKey, len(results))
	for i := range results {
		aks[i] = models.AlertKey(results[i])
	}
	return aks, nil
}

func (d *dataAccess) GetOpenIncident(ak models.AlertKey) (*models.IncidentState, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetOpenIncident"})()
	conn := d.GetConnection()
	defer conn.Close()

	inc, err := d.getLatestIncident(ak, conn)
	if err != nil {
		return nil, slog.Wrap(err)
	}
	if inc == nil {
		return nil, nil
	}
	if inc.Open {
		return inc, nil
	}
	return nil, nil
}

func (d *dataAccess) getLatestIncident(ak models.AlertKey, conn redis.Conn) (*models.IncidentState, error) {
	id, err := redis.Int64(conn.Do("LINDEX", incidentsForAlertKeyKey(ak), 0))
	if err != nil {
		if err == redis.ErrNil {
			return nil, nil
		}
		return nil, slog.Wrap(err)
	}
	inc, err := d.getIncident(id, conn)
	if err != nil {
		return nil, slog.Wrap(err)
	}
	return inc, nil
}

func (d *dataAccess) GetLatestIncident(ak models.AlertKey) (*models.IncidentState, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetLatestIncident"})()
	conn := d.GetConnection()
	defer conn.Close()

	return d.getLatestIncident(ak, conn)
}

func (d *dataAccess) GetAllOpenIncidents() ([]*models.IncidentState, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetAllOpenIncidents"})()
	conn := d.GetConnection()
	defer conn.Close()

	// get open ids
	ids, err := int64s(conn.Do("HVALS", statesOpenIncidentsKey))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	return d.incidentMultiGet(conn, ids)
}

func (d *dataAccess) GetAllIncidents(ak models.AlertKey) ([]*models.IncidentState, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetAllIncidents"})()
	conn := d.GetConnection()
	defer conn.Close()

	ids, err := int64s(conn.Do("LRANGE", incidentsForAlertKeyKey(ak), 0, -1))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	return d.incidentMultiGet(conn, ids)
}

func (d *dataAccess) incidentMultiGet(conn redis.Conn, ids []int64) ([]*models.IncidentState, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	// get all incident json keys
	args := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		args = append(args, incidentStateKey(id))
	}
	jsons, err := redis.Strings(conn.Do("MGET", args...))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	results := make([]*models.IncidentState, 0, len(jsons))
	for _, j := range jsons {
		state := &models.IncidentState{}
		if err = json.Unmarshal([]byte(j), state); err != nil {
			return nil, slog.Wrap(err)
		}
		results = append(results, state)
	}
	return results, nil
}

func (d *dataAccess) getIncident(incidentId int64, conn redis.Conn) (*models.IncidentState, error) {
	b, err := redis.Bytes(conn.Do("GET", incidentStateKey(incidentId)))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	state := &models.IncidentState{}
	if err = json.Unmarshal(b, state); err != nil {
		return nil, slog.Wrap(err)
	}
	return state, nil
}

func (d *dataAccess) GetIncidentState(incidentId int64) (*models.IncidentState, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetIncident"})()
	conn := d.GetConnection()
	defer conn.Close()
	return d.getIncident(incidentId, conn)
}

func (d *dataAccess) UpdateIncidentState(s *models.IncidentState) error {
	return d.save(s, false)
}

func (d *dataAccess) ImportIncidentState(s *models.IncidentState) error {
	return d.save(s, true)
}

func (d *dataAccess) save(s *models.IncidentState, isImport bool) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "UpdateIncident"})()
	conn := d.GetConnection()
	defer conn.Close()

	isNew := false
	//if id is still zero, assign new id.
	if s.Id == 0 {
		id, err := redis.Int64(conn.Do("INCR", "maxIncidentId"))
		if err != nil {
			return slog.Wrap(err)
		}
		s.Id = id
		isNew = true
	} else if isImport {
		max, err := redis.Int64(conn.Do("GET", "maxIncidentId"))
		if err != nil {
			max = 0
		}
		if max < s.Id {
			if _, err = conn.Do("SET", "maxIncidentId", s.Id); err != nil {
				return slog.Wrap(err)
			}
		}
		isNew = true
	}
	return d.transact(conn, func() error {
		if isNew {
			// add to list for alert key
			if _, err := conn.Do("LPUSH", incidentsForAlertKeyKey(s.AlertKey), s.Id); err != nil {
				return slog.Wrap(err)
			}
			dat := fmt.Sprintf("%d:%d:%s", s.Id, s.Start.UTC().Unix(), s.AlertKey)
			if _, err := conn.Do("LPUSH", "allIncidents", dat); err != nil {
				return slog.Wrap(err)
			}
		}

		// store the incident json
		data, err := json.Marshal(s)
		if err != nil {
			return slog.Wrap(err)
		}
		_, err = conn.Do("SET", incidentStateKey(s.Id), data)

		addRem := func(b bool) string {
			if b {
				return "SADD"
			}
			return "SREM"
		}
		// appropriately add or remove it from the "open" set
		if s.Open {
			if _, err = conn.Do("HSET", statesOpenIncidentsKey, s.AlertKey, s.Id); err != nil {
				return slog.Wrap(err)
			}
		} else {
			if _, err = conn.Do("HDEL", statesOpenIncidentsKey, s.AlertKey); err != nil {
				return slog.Wrap(err)
			}
		}

		//appropriately add or remove from unknown and uneval sets
		if _, err = conn.Do(addRem(s.CurrentStatus == models.StUnknown), statesUnknownKey(s.Alert), s.AlertKey); err != nil {
			return slog.Wrap(err)
		}
		if _, err = conn.Do(addRem(s.Unevaluated), statesUnevalKey(s.Alert), s.AlertKey); err != nil {
			return slog.Wrap(err)
		}
		return nil
	})
}

func (d *dataAccess) SetUnevaluated(ak models.AlertKey, uneval bool) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "SetUnevaluated"})()
	conn := d.GetConnection()
	defer conn.Close()

	op := "SREM"
	if uneval {
		op = "SADD"
	}
	_, err := conn.Do(op, statesUnevalKey(ak.Name()), ak)
	return slog.Wrap(err)
}

// The nucular option. Delete all we know about this alert key
func (d *dataAccess) Forget(ak models.AlertKey) error {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "Forget"})()
	conn := d.GetConnection()
	defer conn.Close()

	ids, err := int64s(conn.Do("LRANGE", incidentsForAlertKeyKey(ak), 0, -1))
	if err != nil {
		return slog.Wrap(err)
	}
	alert := ak.Name()
	return d.transact(conn, func() error {
		// last touched.
		if _, err := conn.Do("HDEL", statesLastTouchedKey(alert), ak); err != nil {
			return slog.Wrap(err)
		}
		// unknown/uneval sets
		if _, err := conn.Do("SREM", statesUnknownKey(alert), ak); err != nil {
			return slog.Wrap(err)
		}
		if _, err := conn.Do("SREM", statesUnevalKey(alert), ak); err != nil {
			return slog.Wrap(err)
		}
		//open set
		if _, err := conn.Do("HDEL", statesOpenIncidentsKey, ak); err != nil {
			return slog.Wrap(err)
		}
		if _, err = conn.Do("HDEL", statesOpenIncidentsKey, ak); err != nil {
			return slog.Wrap(err)
		}
		for _, id := range ids {

			if _, err = conn.Do("DEL", incidentStateKey(id)); err != nil {
				return slog.Wrap(err)
			}
		}
		if _, err := conn.Do(d.LCLEAR(), incidentsForAlertKeyKey(ak)); err != nil {
			return slog.Wrap(err)
		}
		return nil
	})
}

func (d *dataAccess) GetUnknownAndUnevalAlertKeys(alert string) ([]models.AlertKey, []models.AlertKey, error) {
	defer collect.StartTimer("redis", opentsdb.TagSet{"op": "GetUnknownAndUnevalAlertKeys"})()
	conn := d.GetConnection()
	defer conn.Close()

	unknownS, err := redis.Strings(conn.Do("SMEMBERS", statesUnknownKey(alert)))
	if err != nil {
		return nil, nil, slog.Wrap(err)
	}
	unknown := make([]models.AlertKey, len(unknownS))
	for i, u := range unknownS {
		unknown[i] = models.AlertKey(u)
	}

	unEvals, err := redis.Strings(conn.Do("SMEMBERS", statesUnevalKey(alert)))
	if err != nil {
		return nil, nil, slog.Wrap(err)
	}
	unevals := make([]models.AlertKey, len(unEvals))
	for i, u := range unEvals {
		unevals[i] = models.AlertKey(u)
	}

	return unknown, unevals, nil
}

func int64s(reply interface{}, err error) ([]int64, error) {
	if err != nil {
		return nil, slog.Wrap(err)
	}
	ints := []int64{}
	values, err := redis.Values(reply, err)
	if err != nil {
		return ints, slog.Wrap(err)
	}
	if err := redis.ScanSlice(values, &ints); err != nil {
		return ints, slog.Wrap(err)
	}
	return ints, nil
}

func (d *dataAccess) transact(conn redis.Conn, f func() error) error {
	if !d.isRedis {
		return f()
	}
	if _, err := conn.Do("MULTI"); err != nil {
		return slog.Wrap(err)
	}
	if err := f(); err != nil {
		return slog.Wrap(err)
	}
	if _, err := conn.Do("EXEC"); err != nil {
		return slog.Wrap(err)
	}
	return nil
}
