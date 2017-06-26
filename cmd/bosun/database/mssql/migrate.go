package mssql

import "fmt"

func (m *msql) createTables() error {
	const tableCheck = `IF  NOT EXISTS (SELECT * FROM sys.objects 
WHERE object_id = OBJECT_ID(N'[dbo].[%s]') AND type in (N'U'))

BEGIN
%s
END`
	tables := []struct{ name, create string }{
		{
			name: "metric_metadata",
			create: `CREATE TABLE [dbo].[metric_metadata]
			(
				name nvarchar(500) NOT NULL, 
				description nvarchar(2000) NULL, 
				unit nvarchar(100) NULL, 
				rate nvarchar(100) NULL,
				last_touched datetime NOT NULL
			)`,
		},
		{
			name: "tag_metadata",
			create: `CREATE TABLE [dbo].[tag_metadata]
			(
				id int not null identity(1,1) primary key,
				name nvarchar(500) NOT NULL,
				value nvarchar(2000) NOT NULL,
				tags nvarchar(500) NOT NULL,
				last_touched datetime NOT NULL
			)`,
		},
		{
			name: "tag_metadata_lookup",
			create: `CREATE TABLE [dbo].[tag_metadata_lookup]
			(
				meta_id int NOT NULL,
				tagk nvarchar(100) NOT NULL,
				tagv nvarchar(100) NOT NULL,
				
			)`,
		},
	}
	for _, t := range tables {
		q := fmt.Sprintf(tableCheck, t.name, t.create)
		if _, err := m.d.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
