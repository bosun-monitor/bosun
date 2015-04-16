#!/bin/bash

set -e
set -x

sleep 10

export HBASE_HOME=/usr/lib/hbase
export JAVA_HOME=/usr/lib/jvm/java-1.7.0-openjdk-1.7.0.79.x86_64

if [ ! -e /tsdb/opentsdb_tables_created.txt ]; then
	echo "creating tsdb tables"
	bash /tsdb/create_tsdb_tables.sh
	echo "created tsdb tables"
fi

echo "starting opentsdb"
/tsdb/build/tsdb tsd --port=4242 --staticroot=/tsdb/build/staticroot --cachedir=/tmp --auto-metric --config=/tsdb/opentsdb.conf
