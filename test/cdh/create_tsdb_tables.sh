#!/bin/bash

set -x
set -e

export COMPRESSION="LZO"
export HBASE_HOME=/usr/lib/hbase
export JAVA_HOME=/usr/lib/jvm/java-1.7.0-openjdk-1.7.0.79.x86_64

cd /tsdb
./src/create_table.sh
touch /tsdb/opentsdb_tables_created.txt
