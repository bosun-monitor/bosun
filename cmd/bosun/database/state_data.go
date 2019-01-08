package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"strings"

	"bosun.org/models"
	"bosun.org/slog"
	"github.com/garyburd/redigo/redis"
)

/*
incidentById:{id} - json encoded state. Authoritative source.

renderedTemplatesById:{id} - json encoded RenderedTemplates by Incident Id

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
func renderedTemplatesKey(id int64) string {
	return fmt.Sprintf("renderedTemplatesById:%d", id)
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

	GetAllIncidentsByAlertKey(ak models.AlertKey) ([]*models.IncidentState, error)
	GetAllIncidentIdsByAlertKey(ak models.AlertKey) ([]int64, error)

	UpdateIncidentState(s *models.IncidentState) (int64, error)
	ImportIncidentState(s *models.IncidentState) error

	// SetIncidentNext gets the incident for previousIncidentId, and sets its NextId field to be nextIncidentId and then saves the incident
	SetIncidentNext(incidentId, nextIncidentId int64) error

	SetRenderedTemplates(incidentId int64, rt *models.RenderedTemplates) error
	GetRenderedTemplates(incidentId int64) (*models.RenderedTemplates, error)
	GetRenderedTemplateKeys() ([]string, error)
	StartTTLEnforcerLoop(renderedTemplatesMaxAge time.Duration, closedIncidentsMaxAge time.Duration)
	CleanupOldIncidents(config RetentionConfig)

	Forget(ak models.AlertKey) error
	SetUnevaluated(ak models.AlertKey, uneval bool) error
	GetUnknownAndUnevalAlertKeys(alert string) ([]models.AlertKey, []models.AlertKey, error)
}

func (d *dataAccess) SetRenderedTemplates(incidentId int64, rt *models.RenderedTemplates) error {
	conn := d.Get()
	defer conn.Close()

	data, err := json.Marshal(rt)
	if err != nil {
		return slog.Wrap(err)
	}
	_, err = conn.Do("SET", renderedTemplatesKey(incidentId), data)
	if err != nil {
		return slog.Wrap(err)
	}
	return nil
}

func (d *dataAccess) GetRenderedTemplates(incidentId int64) (*models.RenderedTemplates, error) {
	conn := d.Get()
	defer conn.Close()

	b, err := redis.Bytes(conn.Do("GET", renderedTemplatesKey(incidentId)))
	renderedT := &models.RenderedTemplates{}
	if err != nil {
		if err == redis.ErrNil {
			return renderedT, nil
		}
		return nil, slog.Wrap(err)
	}
	if err = json.Unmarshal(b, renderedT); err != nil {
		return nil, slog.Wrap(err)
	}
	return renderedT, nil
}

func (d *dataAccess) scanMatchCmd(pattern string) (string, []interface{}, int) {
	//ledis uses XSCAN cursor "KV" MATCH foo
	//redis uses SCAN cursor MATCH foo
	if d.isRedis {
		return "SCAN", []interface{}{"0", "MATCH", pattern}, 0
	}
	return "XSCAN", []interface{}{"KV", "0", "MATCH", pattern}, 1
}

func (d *dataAccess) getScanResults(pattern string) ([]string, error) {
	conn := d.Get()
	defer conn.Close()

	cmd, args, cursorIdx := d.scanMatchCmd(pattern)
	found := []string{}
	for {
		vals, err := redis.Values(conn.Do(cmd, args...))
		if err != nil {
			return nil, slog.Wrap(err)
		}
		cursor, err := redis.String(vals[0], nil)
		if err != nil {
			return nil, slog.Wrap(err)
		}
		args[cursorIdx] = cursor
		keys, err := redis.Strings(vals[1], nil)
		if err != nil {
			return nil, slog.Wrap(err)
		}
		found = append(found, keys...)
		if cursor == "" || cursor == "0" {
			break
		}
	}
	return found, nil
}

func (d *dataAccess) GetRenderedTemplateKeys() ([]string, error) {
	return d.getScanResults("renderedTemplatesById:*")
}

func (d *dataAccess) GetIncidentKeys() ([]string, error) {
	return d.getScanResults("incidentById:*")
}

// Delete references to incident id
func (d *dataAccess) deleteIncidentReferences(conn redis.Conn, incident *models.IncidentState) error {
	if incident.Open {
		return errors.New("state_data: cannot delete open incidents")
	}
	command, args := d.LMCLEAR(incidentsForAlertKeyKey(incident.AlertKey), strconv.FormatInt(incident.Id, 10))
	_, err := conn.Do(command, args...)
	if err != nil {
		return slog.Wrap(err)
	}
	return nil
}

func (d *dataAccess) deleteBatch(conn redis.Conn, keys []interface{}) {
	_, err := conn.Do("DEL", keys...)
	if err != nil {
		slog.Error(err)
	}
}

func (d *dataAccess) deleteKeys(keys <-chan string) {
	conn := d.Get()
	defer conn.Close()
	const batchSize = 1000
	args := make([]interface{}, 0, batchSize)
	for key := range keys {
		args = append(args, key)
		if len(args) == batchSize {
			// Flush after we receive enough keys
			d.deleteBatch(conn, args)
		}
	}
	// Flush remaining events in buffer when channel closes
	if len(args) > 0 {
		d.deleteBatch(conn, args)
	}
}

func (d *dataAccess) State() StateDataAccess {
	return d
}

func (d *dataAccess) TouchAlertKey(ak models.AlertKey, t time.Time) error {
	conn := d.Get()
	defer conn.Close()

	_, err := conn.Do("ZADD", statesLastTouchedKey(ak.Name()), t.UTC().Unix(), string(ak))
	return slog.Wrap(err)
}

func (d *dataAccess) GetUntouchedSince(alert string, time int64) ([]models.AlertKey, error) {
	conn := d.Get()
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
	conn := d.Get()
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
	conn := d.Get()
	defer conn.Close()

	return d.getLatestIncident(ak, conn)
}

func (d *dataAccess) GetAllOpenIncidents() ([]*models.IncidentState, error) {
	conn := d.Get()
	defer conn.Close()

	// get open ids
	ids, err := int64s(conn.Do("HVALS", statesOpenIncidentsKey))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	return d.incidentMultiGet(conn, ids)
}

func (d *dataAccess) GetAllIncidentsByAlertKey(ak models.AlertKey) ([]*models.IncidentState, error) {
	conn := d.Get()
	defer conn.Close()

	ids, err := int64s(conn.Do("LRANGE", incidentsForAlertKeyKey(ak), 0, -1))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	return d.incidentMultiGet(conn, ids)
}

func (d *dataAccess) GetAllIncidentIdsByAlertKey(ak models.AlertKey) ([]int64, error) {
	conn := d.Get()
	defer conn.Close()

	ids, err := int64s(conn.Do("LRANGE", incidentsForAlertKeyKey(ak), 0, -1))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	return ids, nil
}

// In general one should not use the redis KEYS command. So this is only used
// in migration. If we want to use a proper index of all incidents
// then issues with allIncidents must be fixed. Currently it is planned
// to remove allIncidents in a future commit
func (d *dataAccess) getAllIncidentIdsByKeys() ([]int64, error) {
	conn := d.Get()
	defer conn.Close()

	summaries, err := redis.Strings(conn.Do("KEYS", "incidentById:*"))
	if err != nil {
		return nil, slog.Wrap(err)
	}
	ids := make([]int64, len(summaries))
	for i, sum := range summaries {
		var err error
		ids[i], err = strconv.ParseInt(strings.Split(sum, ":")[1], 0, 64)
		if err != nil {
			return nil, slog.Wrap(err)
		}
	}
	return ids, nil
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

// setIncident directly sets the incident as is to the datastore
func (d *dataAccess) setIncident(incident *models.IncidentState, conn redis.Conn) error {
	data, err := json.Marshal(incident)
	if err != nil {
		return slog.Wrap(err)
	}
	if _, err = conn.Do("SET", incidentStateKey(incident.Id), data); err != nil {
		return err
	}
	return nil
}

func (d *dataAccess) GetIncidentState(incidentId int64) (*models.IncidentState, error) {
	conn := d.Get()
	defer conn.Close()
	return d.getIncident(incidentId, conn)
}

// SetIncidentNext gets the incident for previousIncidentId, and sets its NextId field
// to be nextIncidentId and then saves the incident
func (d *dataAccess) SetIncidentNext(previousIncidentId, nextIncidentId int64) error {
	conn := d.Get()
	defer conn.Close()
	previousIncident, err := d.getIncident(previousIncidentId, conn)
	if err != nil {
		return err
	}
	previousIncident.NextId = nextIncidentId
	err = d.setIncident(previousIncident, conn)
	if err != nil {
		return err
	}
	return nil
}

func (d *dataAccess) UpdateIncidentState(s *models.IncidentState) (int64, error) {
	return d.save(s, false)
}

func (d *dataAccess) ImportIncidentState(s *models.IncidentState) error {
	_, err := d.save(s, true)
	return err
}

func (d *dataAccess) save(s *models.IncidentState, isImport bool) (int64, error) {
	conn := d.Get()
	defer conn.Close()

	isNew := false
	//if id is still zero, assign new id.
	if s.Id == 0 {
		id, err := redis.Int64(conn.Do("INCR", "maxIncidentId"))
		if err != nil {
			return s.Id, slog.Wrap(err)
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
				return s.Id, slog.Wrap(err)
			}
		}
		isNew = true
	}
	return s.Id, d.transact(conn, func() error {
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
	conn := d.Get()
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
	conn := d.Get()
	defer conn.Close()

	ids, err := int64s(conn.Do("LRANGE", incidentsForAlertKeyKey(ak), 0, -1))
	if err != nil {
		return slog.Wrap(err)
	}
	alert := ak.Name()
	return d.transact(conn, func() error {
		// last touched.
		if _, err := conn.Do("ZREM", statesLastTouchedKey(alert), ak); err != nil {
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
			if _, err = conn.Do("DEL", renderedTemplatesKey(id)); err != nil {
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
	conn := d.Get()
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

// getAllIds runs each getKeys function to retrieve an array of strings which describe a DB key (e.g. "incidentId:42" or
// "renderedTemplateId:42").  The incident IDs (e.g. 42 in the case above) are returned as map keys.
func (d *dataAccess) getAllIds(getKeys ...func() ([]string, error)) map[int64]bool {
	allIds := make(map[int64]bool)
	for _, keyGetter := range getKeys {
		keys, err := keyGetter()
		if err != nil {
			slog.Error(err)
			continue
		}
		for _, key := range keys {
			parts := strings.Split(key, ":")
			if len(parts) != 2 {
				slog.Errorf("invalid redis key found: %s", key)
				continue
			}
			id, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				slog.Error(err)
				continue
			}
			allIds[id] = true
		}
	}
	return allIds
}

// StartTTLEnforcerLoop will in a loop purge any old rendered templates and incidents
func (d *dataAccess) StartTTLEnforcerLoop(renderedTemplateMaxAge time.Duration, closedIncidentMaxAge time.Duration) {
	if renderedTemplateMaxAge <= 0 && closedIncidentMaxAge <= 0 {
		// No cleanup to do
		return
	}

	// wait for 5 minutes initially (to let bosun stabilize) and then run every hour
	time.Sleep(time.Minute * 5)
	for {
		slog.Info("running state TTL cleanup")
		retentionConfig := NewRetentionConfig(renderedTemplateMaxAge, closedIncidentMaxAge)
		d.CleanupOldIncidents(retentionConfig)
		slog.Info("cleanup complete")
		time.Sleep(time.Hour)
	}
}

type RetentionConfig struct {
	renderedTemplateMaxAge     time.Duration
	closedIncidentMaxAge       time.Duration
	earliestOkRenderedTemplate time.Time
	earliestOkClosedIncident   time.Time
}

func NewRetentionConfig(renderedTemplateMaxAge time.Duration, closedIncidentMaxAge time.Duration) RetentionConfig {
	now := time.Now().UTC()
	return RetentionConfig{
		renderedTemplateMaxAge:     renderedTemplateMaxAge,
		closedIncidentMaxAge:       closedIncidentMaxAge,
		earliestOkRenderedTemplate: now.Add(-1 * renderedTemplateMaxAge),
		earliestOkClosedIncident:   now.Add(-1 * closedIncidentMaxAge),
	}
}

func (config RetentionConfig) ShouldDeleteIncident(state *models.IncidentState, err error) bool {
	if config.closedIncidentMaxAge <= 0 {
		return false
	}
	if err != nil {
		slog.Error(err)
		return false
	}
	if state.End != nil && (*state.End).Before(config.earliestOkClosedIncident) {
		return true
	}
	return false
}

func (config RetentionConfig) ShouldDeleteRenderedTemplate(state *models.IncidentState, err error) bool {
	if config.renderedTemplateMaxAge <= 0 {
		return false
	}
	if err != nil {
		if IsRedisNil(err) {
			return true
		}
		slog.Error(err)
		return false
	}
	if state.End != nil && (*state.End).Before(config.earliestOkRenderedTemplate) {
		return true
	}
	return false
}

func (d *dataAccess) CleanupOldIncidents(config RetentionConfig) {
	idGetters := make([]func() ([]string, error), 0)

	if config.renderedTemplateMaxAge > 0 {
		slog.Infof("running rendered template cleanup with TTL %s", config.renderedTemplateMaxAge.String())
		idGetters = append(idGetters, d.GetRenderedTemplateKeys)
	}
	if config.closedIncidentMaxAge > 0 {
		slog.Infof("running closed incident cleanup with TTL %s", config.closedIncidentMaxAge.String())
		idGetters = append(idGetters, d.GetIncidentKeys)
	}

	incidentsIds := d.getAllIds(idGetters...)
	if incidentsIds == nil || len(incidentsIds) == 0 {
		return
	}

	var wg sync.WaitGroup
	defer wg.Wait()
	keysToPurge := make(chan string, 1000)
	defer close(keysToPurge)

	wg.Add(1)
	go func() {
		defer wg.Done()
		d.deleteKeys(keysToPurge)
	}()

	incidentsDeleted := 0
	renderedTemplatesDeleted := 0

	conn := d.Get()
	defer conn.Close()
	for id := range incidentsIds {
		state, err := d.getIncident(id, conn)
		if config.ShouldDeleteIncident(state, err) {
			err := d.deleteIncidentReferences(conn, state)
			if err != nil {
				slog.Error(err)
				continue
			}
			keysToPurge <- renderedTemplatesKey(id)
			keysToPurge <- incidentStateKey(id)
			incidentsDeleted++
		} else if config.ShouldDeleteRenderedTemplate(state, err) {
			keysToPurge <- renderedTemplatesKey(id)
			renderedTemplatesDeleted++
		}
	}
	if renderedTemplatesDeleted > 0 {
		slog.Infof("deleting %d old rendered templates", renderedTemplatesDeleted)
	}
	if incidentsDeleted > 0 {
		slog.Infof("deleting %d old closed incidents", incidentsDeleted)
	}
}

func IsRedisNil(err error) bool {
	if err != nil && strings.Contains(err.Error(), "nil returned") {
		return true
	}
	return false
}
