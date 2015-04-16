#!/bin/bash

set -e
set -x

export COMPRESSION="LZO"

cd /tsdb
./src/create_table.sh
touch /tsdb/opentsdb_tables_created.txt
