#!/bin/bash
echo "Sleeping for 30 seconds to give HBase time to warm up"
sleep 30 

if [ ! -e ${TSDB_DIR}/opentsdb_tables_created.txt ]; then
	echo "creating tsdb tables"
	bash ${TSDB_DIR}/create_tsdb_tables.sh
	echo "created tsdb tables"
fi

echo "starting opentsdb"
${TSDB_DIR}/build/tsdb tsd --port=4242 --staticroot=${TSDB_DIR}/build/staticroot --cachedir=/tmp --auto-metric --config=${TSDB_DIR}/opentsdb.conf
