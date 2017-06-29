
#!/bin/bash

export COMPRESSION="NONE"

cd $TSDB
./src/create_table.sh
if [ $? -ne 0 ]; then
    echo "Opentsdb /src/create_table.sh failed"
else
    touch $TSDB/opentsdb_tables_created.txt
fi