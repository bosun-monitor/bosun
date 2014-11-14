#!/bin/bash

set -e

boot2docker=$(which boot2docker)
if [ -x "$boot2docker" ]; then
	boot2docker ssh sudo ntpclient -s -h pool.ntp.org
fi

docker build -t bosun-builder builder
TMP=run/tmp
rm -rf build/bosun $TMP
git clone -b master --single-branch .. build/bosun
docker build -t bosun-build --no-cache build
ID=$(docker run -d bosun-build)
mkdir -p $TMP/hbase $TMP/bosun $TMP/tsdb $TMP/scollector
docker cp ${ID}:/go/bin/bosun $TMP/bosun/.
docker cp ${ID}:/go/bin/scollector $TMP/scollector/.
docker cp ${ID}:/hbase $TMP
docker cp ${ID}:/tsdb $TMP
docker kill ${ID}
docker rm ${ID}
rm -rf $TMP/hbase/hbase-*/docs $TMP/tsdb/build/gwt-unitCache
docker build -t stackexchange/bosun run
rm -rf build/bosun $TMP
echo bosun docker image built
