package mssql

import (
	"database/sql"
	"time"

	"fmt"

	"strings"

	"bosun.org/cmd/bosun/database"
	"bosun.org/opentsdb"
)

func New(d *sql.DB) (database.MetadataDataAccess, error) {
	db := &msql{
		d:     d,
		preps: map[string]*sql.Stmt{},
	}
	if err := db.createTables(); err != nil {
		return nil, err
	}
	return db, nil
}

type msql struct {
	d *sql.DB

	preps map[string]*sql.Stmt
}

func (m *msql) PutMetricMetadata(metric string, field string, value string) (err error) {
	if field != "desc" && field != "unit" && field != "rate" {
		return fmt.Errorf("Unknown metric metadata field: %s", field)
	}
	if field == "desc" {
		field = "description"
	}
	const q = `
IF NOT EXISTS (SELECT * FROM dbo.metric_metadata WHERE name = ?1)
    INSERT INTO dbo.metric_metadata(name, last_touched, %s) VALUES (?1, GETUTCDATE(),?2)
ELSE
    UPDATE dbo.metric_metadata SET %s = ?2, last_touched = GETUTCDATE() where name = ?1
`
	key := "put_metric_metadata_" + field
	ps := m.preps[key]
	if ps == nil {
		ps, err = m.d.Prepare(fmt.Sprintf(q, field, field))
		if err != nil {
			return err
		}
		m.preps[key] = ps
	}
	_, err = ps.Exec(metric, value)
	return err
}

func (m *msql) GetMetricMetadata(metric string) (*database.MetricMetadata, error) {

	row := m.d.QueryRow(`SELECT description, rate, unit, last_touched FROM [dbo].[metric_metadata] WHERE name = ?`, metric)
	var rate, unit, desc sql.NullString
	var lt time.Time
	err := row.Scan(&desc, &rate, &unit, &lt)
	if err != nil {
		return nil, err
	}
	mm := &database.MetricMetadata{
		Desc:        desc.String,
		Rate:        rate.String,
		Unit:        unit.String,
		LastTouched: lt.Unix(),
	}
	mm.Desc = desc.String
	mm.Rate = rate.String
	mm.Unit = unit.String
	return mm, nil
}

func (m *msql) PutTagMetadata(tags opentsdb.TagSet, name string, value string, updated time.Time) error {
	const q = `
IF NOT EXISTS (SELECT * FROM dbo.tag_metadata WHERE name = ?1 AND tags = ?2)
    INSERT INTO dbo.tag_metadata(name,value,tags,last_touched) VALUES (?1,?3,?2, GETUTCDATE())
ELSE
	UPDATE dbo.tag_metadata SET value =?3, last_touched = GETUTCDATE() WHERE name = ?1 AND tags = ?2
`
	res, err := m.d.Exec(q, name, tags.Tags(), value)
	if err != nil {
		return err
	}
	const q2 = `INSERT INTO dbo.tag_metadata_lookup(meta_id,tagk,tagv) VALUES (?,?,?)`
	if id, err := res.LastInsertId(); err == nil {
		for k, v := range tags {
			_, err = m.d.Exec(q2, id, k, v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *msql) GetTagMetadata(tags opentsdb.TagSet, name string) ([]*database.TagMetadata, error) {
	subs := make([]string, len(tags))
	const single = "SELECT meta_id from tag_metadata_lookup WHERE tagk = ? AND tagv = ?"
	args := []interface{}{}
	for k, v := range tags {
		args = append(args, k, v)
		subs = append(subs, single)
	}
	joined := strings.Join(subs, " INTERSECT ")
	whole := fmt.Sprintf("SELECT m.name, m.tags, m.value, m.last_touched from tag_metadata AS m JOIN (%s) AS l ON l.meta_id = m.id", joined)
	_, err := m.d.Query(whole, args...)

	return nil, err
}
func (m *msql) DeleteTagMetadata(tags opentsdb.TagSet, name string) error {

	return nil
}
