#!/bin/bash
echo "Sleeping for 15 seconds to give HBase time to warm up"
sleep 15

if [ ! -e $TSDB/opentsdb_tables_created.txt ]; then
	echo "creating tsdb tables"
	bash $TSDB/create_tsdb_tables.sh
	echo "created tsdb tables"
fi

echo "starting opentsdb"
$TSDB/build/tsdb tsd --port=4242 --staticroot=$TSDB/build/staticroot --cachedir=/tmp --auto-metric
