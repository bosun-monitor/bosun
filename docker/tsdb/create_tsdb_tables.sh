#!/bin/bash

export COMPRESSION="NONE"

${TSDB_DIR}/src/create_table.sh
if [ $? -ne 0 ]; then
    echo "Opentsdb /src/create_table.sh failed"
    exit 1
else
    touch ${TSDB_DIR}/opentsdb_tables_created.txt
fi
