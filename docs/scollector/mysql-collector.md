---
layout: default
title: MySQL Collectors
---
  &nbsp;

## Using MySQL Collectors with Scollector

In order to setup the MySQL Collectors with Scollector it needs to be configured on the database side as well as the Scollector side

  &nbsp;

### Scollector configuration file for MySQL Collectors setup

When putting Scollector on the same box as the MySQL database, usually will only need the following four lines added to the configuration file.

[[MySQL]]
DBName = "my_friendly_name"
Username = "app_stats"
Password = "type_password_here"

The DBName is a friendly name for your database which will be under the db_name tag. It is not required but can be helpful for looking up databases.
The Username and Password will used to connect to the MySQL database. The Username and Password will need to be the same as what is configured in the database setup section below.

Depending on your exact setup, some additional settings below may be wanted. For a clustered database, the same DBName for each and unique InstId are recommended. A Scollector can be configured to connect to more than one MySQL database. It also can be configured to connect to the same database but pull from different queries tables. This can be useful for splitting up fast running queries from slow running queries.


Here are all the MySQL configuration settings:

**MaxOpenConns** (int) Default: 2 - This is the max number of connections and queries that run at one time
**GetQueriesInterval** (int) Default 30 - This is the number of minutes between reloading the queries from the colection queries table
**DBName** (string) - This is a friendly name for your database that will be put under the db_name tag. This is helpful if you would like to look up databases by their db_name instead of by their host name. It is also helpful for grouping databases, especially if you have a cluster database.
**InstId** (int) Default: 1 - This number will be put under the inst_id tag. This is helpful if you have a group of databases under the same host name or db_name.
**Username** (string) Default: root - This is the username that will be used for connecting to the MySQL database
**Password** (string) - If this is blank, no password will be used. Otherwise this is the password that will be used to connect to the MySQL database
**Protocol** (string) Default: tcp - This is the connection protocol used to connect to the MySQL database
**Address** (string) Default: 127.0.0.1 - This is the address used to connect to the MySQL database. It can be an IP address or hostname. It will also accept IPv6 address in the form of [de:ad:be:ef::ca:fe]
**Port** (int) Default: 3306 -This is the connection port used to connect to the MySQL database
**DBOfTable** (string) - The default setting for this is to use the Username provide in the configuration file. If you are using root to connect, the root user will probably not have a database so another database will need to be created and the name of that database set here.
**TableName** (string) Default: scollectorqueries - This is the table name that Scollector will select from to get the collection queries. It will look for this table under the database configured by DBOfTable setting.


All the MySQL database connection settings are just being passed to the MySQL driver. If you would like more details about those settings, please feel free to look at documentation on the driver's website: https://github.com/go-sql-driver/mysql

  &nbsp;

### MySQL setup and configuration for Scollector

Scollector is told what data to collect from MySQL by querying the collection table which has the collection queries. The basic setup is to run in following SQL into the MySQL database and make sure the settings below match the settings in the section above, for example: Username and Password.

```
CREATE USER 'app_stats'@'%' IDENTIFIED BY 'type_password_here';

CREATE DATABASE app_stats;

CREATE TABLE IF NOT EXISTS `app_stats`.`scollectorqueries` (
  `name` varchar(64) NOT NULL,
  `query` varchar(512) NOT NULL,
  `description` varchar(256) NOT NULL,
  `hastime` tinyint(1) NOT NULL,
  `queryinterval` int(11) NOT NULL,
  UNIQUE KEY `name` (`name`)
);

GRANT INSERT, SELECT, UPDATE, DELETE on `app_stats`.* to `app_stats`;

GRANT SELECT, PROCESS on *.* to `app_stats`;

INSERT INTO `app_stats`.`scollectorqueries` (`name`, `query`, `description`, `hastime`, `queryinterval`) VALUES
('global_status', 'select variable_value, ''name'', lower(variable_name) from information_schema.global_status where variable_value > 0', 'Variables from information_schema.global_status that are not zero. The name tag is the variable name.', 0, 20);

INSERT INTO `app_stats`.`scollectorqueries` (`name`, `query`, `description`, `hastime`, `queryinterval`) VALUES
('processlist.time', 'select time, ''user'', lower(user), ''command'', lower(command), ''db'', lower(db), ''state'', lower(state) from information_schema.processlist where command != ''Sleep'' and user != ''system user'' and command != ''Binlog Dump'' group by user, command, db, state', 'Processes from information_schema.processlist that are not in Sleep. Tags: user, command, db, and state.', 0, 20);

INSERT INTO `app_stats`.`scollectorqueries` (`name`, `query`, `description`, `hastime`, `queryinterval`) VALUES
('processlist.count', 'select count(1), ''user'', lower(user), ''command'', lower(command), ''db'', lower(db), ''state'', lower(state) from information_schema.processlist where command != ''Sleep'' and user != ''system user'' and command != ''Binlog Dump'' group by user, command, db, state', 'Processes from information_schema.processlist that are not in Sleep. Tags: user, command, db, and state.', 0, 20);
```

Here are the details for each of the columns:

**name** - This is the name of the metric. Will automatically have mysql. added to the front of it.
**query** - This is the query that you want to be run to collect the data. The first column is always the value of the metric. The second column may be a timestamp if hastime has been set to 1. The timestamp must be a Unix timestamp. All the rest of the columns are the tag name followed by tag value. No more than 5 tags may be added.
**description** - This is the metadata that will be sent to Bosun.
**hastime** - This is set to 0 if you want to use the timestamp from system time when the query is finished being processed. Set this to 1 if the query will provide the timestamp to use, as the second column. The data can be passed as a string or a int type, but it must be a Unix timestamp.
**queryinterval** This is how often the query will run in seconds. If set to 0, then the query will run at the default interval set for Scollector.


### Additional queries that may be wanted

You are welcome to add your own queries however be mindful of long running queries and the MaxOpenConns setting which defaults to 2. This means if you have it set to 2 and 2 queries are taking a long time to run, no other queries will be to be run to gather metrics.

Below are some additional queries that may be wanted for your database. Would expect this list to grow over time, so if you have a good query, please add it here by updating the Scollector GitHub repository.

```
INSERT INTO `app_stats`.`scollectorqueries` (`name`, `query`, `description`, `hastime`, `queryinterval`) VALUES
('table_rows', 'select table_rows, ''schema'', lower(TABLE_SCHEMA), ''name'', lower(TABLE_NAME) from information_schema.tables where table_rows > 0', 'Table rows from information_schema.tables that are greater than 0. Tags: schema and name.', 0, 300);


INSERT INTO `app_stats`.`scollectorqueries` (`name`, `query`, `description`, `hastime`, `queryinterval`) VALUES
('table_length', 'select data_length, ''schema'', lower(TABLE_SCHEMA), ''name'', lower(TABLE_NAME) from information_schema.tables where data_length > 0;', 'Table data length from information_schema.tables that are greater than 0. Tags: schema and name.', 0, 300);
```
