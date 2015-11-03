---
layout: default
title: Database Collector
---
  &nbsp;

## Using a database collector with Scollector

In order to setup the database collector with Scollector it needs to be configured on the database side as well as the Scollector side. The sections below discusses the details.

  &nbsp;


### Main database configuration settings 

When setting up the database collector configuration file, there are two main parts of the setup, the main database configuration settings and the queries database configuration settings. This section discuses the main database configuration settings.

When putting Scollector on the same box as the database, the below five lines is normally what would be required for the main database configuration settings. Make sure to add the queries database configuration settings as well, which is covered in a latter section.

```
[[Database]]
Type = "mysql"
DBName = "my_friendly_name"
Username = "app_db_stats"
Password = "type_password_here"
```

The Type is the type of database it is. It needs to be in lowercase. Currently only mysql is supported. 
The DBName is a friendly name for your database which is what the tag db_name will be set to. It is not required but can be helpful for looking up databases.
The Username and Password will used to connect to the database. The Username and Password will need to be the same as what is configured in the database setup section below.

Depending on your exact setup, some additional settings below may be wanted or needed. For a clustered database, the same DBName for each database and unique InstId is recommended. A Scollector can be configured to connect to more than one database. It also can be configured to connect to the same database but with different queries to run. This can be useful for splitting up fast running queries from slow running queries.


Here is the descriptions of all the main database configuration settings:

**Type** (string) - This is the type of database that is being connected to. It needs to be in lowercase. Currently only mysql is supported.
**MaxOpenConns** (int) Default: 2 - This is the max number of connections and queries that will run at once.
**DBName** (string) - This is a friendly name for your database that the tag db_name will be set to. This is helpful if you would like to look up databases by their db_name instead of by their host name. It is also helpful for grouping databases, especially if you have a cluster database.
**InstId** (int) Default: 1 - This number will be put under the inst_id tag. This is helpful if you have a group of databases under the same host name or db_name.
**Username** (string) - This is the username that will be used for connecting to the database. The MySQL default will be root.
**Password** (string) - If this is blank, no password will be used. Otherwise this is the password that will be used to connect to the database
**Protocol** (string) - Depending on the type of database, there may be different connection protocol that can be used. The MySQL default is tcp.
**Address** (string) Default: 127.0.0.1 - This is the address used to connect to the database. It can be an IP address or hostname. It will also accept IPv6 address in the form of [de:ad:be:ef::ca:fe]
**Port** (int) - This is the connection port used to connect to the database. The default will depend on the type of database.


Note that all the MySQL database connection settings are just being passed to the MySQL driver. If you would like more details about those settings, please feel free to look at documentation on the driver's website: https://github.com/go-sql-driver/mysql

  &nbsp;


### Queries database configuration settings

In order for Scollector to gather metrics, it needs the queries to run. Can jump to a section below that has the correct basic/starter queries for your database.


Here is the description of all the queries database configuration settings:

**Name** - This is the name of the metric. The database type plus a dot added to the front of it for the final metric name. For example if the database Type is "mysql" and the query metric Name is "general.global_status" the full metric name will be "mysql.general.global_status".
**Query** - This is the query that you want to be run to collect the data. The first column is always the value of the metric. The second column may be a timestamp if Hastime has been set to true. The timestamp must be a Unix timestamp. All the rest of the columns are the tag name followed by tag value. Keep in mind that two tags will be used up already by host and inst_id. Another tag could be used up by the DBName. As of OpenTSDB version 2.1, the max number of tags defaults to 8, so that could limit you to only 5 other tags (10 columns total for tags).
**Description** - This is the OpenTSDB metadata description.
**HasTime** - This is set to false if you want to use the timestamp from Scollector time when the query is finished being processed. Set this to true if the query will provide the timestamp to use, as the second column. The data can be passed as a string or a int type, but it must be a Unix timestamp.
**Interval** This is how often the query will run in seconds. If this is omitted or set to 0, the query will run at the default interval set for Scollector.

  &nbsp;


### MySQL basic/starter queries database configuration settings

Below is a basic/starter configuration for MySQL that should work for most MySQL setups. In the additional MySQL queries section below, there are additional queries that you may want to have as well.

```
[[Database.Query]]
Name = "general.global_status"
Query = "select variable_value, 'name', lower(variable_name) from information_schema.global_status where variable_value > 0"
Description = "Variables from information_schema.global_status that are not zero. The name tag is the variable name."
Interval = 20

[[Database.Query]]
Name = "general.processlist.time"
Query = "select time, 'user', lower(user), 'command', lower(command), 'db', lower(db), 'state', lower(state) from information_schema.processlist where command != 'Sleep' and user != 'system user' and command != 'Binlog Dump' group by user, command, db, state"
Description = "Time of processes from information_schema.processlist that are not in Sleep. Tags: user, command, db, and state."
Interval = 20

[[Database.Query]]
Name = "general.processlist.count"
Query = "select count(1), 'user', lower(user), 'command', lower(command), 'db', lower(db), 'state', lower(state) from information_schema.processlist where command != 'Sleep' and user != 'system user' and command != 'Binlog Dump' group by user, command, db, state"
Description = "Number of processes from information_schema.processlist that are not in Sleep. Tags: user, command, db, and state."
Interval = 20
```

  &nbsp;


### MySQL database setup and configuration for Scollector

The MySQL database setup is straight forward, just need a username and password created that has the correct permissions. Depending on how secure you would like your database to be, you may want to configure a user with more limited permissions. Here is a configuration with more broad permissions that should work with most queries wanted.

```
CREATE USER 'app_db_stats'@'%' IDENTIFIED BY 'type_password_here';

GRANT SELECT, PROCESS on *.* to `app_db_stats`;
```

  &nbsp;


### Additional MySQL queries database configuration settings that may be wanted

You are welcome to add your own queries however be mindful of long running queries and the MaxOpenConns setting which defaults to 2. This means if you have it set to 2 and 2 queries are taking a long time to run, no other queries will be to be run to gather metrics.

Below are some additional queries that may be wanted for your MySQL database. Would expect this list to grow over time, so if you have a good query, please add it here by updating the Scollector GitHub repository.

```
[[Database.Query]]
Name = "general.table.rows"
Query = "select table_rows, 'schema', lower(TABLE_SCHEMA), 'name', lower(TABLE_NAME) from information_schema.tables where table_rows > 0"
Description = "The table rows from information_schema.tables that are greater than 0. Tags: schema and name."
Interval = 300

[[Database.Query]]
Name = "general.table.length"
Query = "select data_length, 'schema', lower(TABLE_SCHEMA), 'name', lower(TABLE_NAME) from information_schema.tables where table_rows > 0"
Description = "The table data length from information_schema.tables that are greater than 0. Tags: schema and name."
Interval = 300
```
