#!/bin/bash

set -e

branch=${1-master}

if  type boot2docker >/dev/null 2>&1; then
        boot2docker ssh sudo ntpclient -s -h pool.ntp.org
fi

TMP=run/tmp
rm -rf build/bosun $TMP
git clone -b $branch --single-branch ../../.. build/bosun
docker build -t bosun-build build
ID=$(docker create bosun-build)
mkdir -p $TMP/hbase $TMP/bosun $TMP/tsdb $TMP/scollector $TMP/tsdbrelay
docker cp ${ID}:/go/bin/bosun $TMP/bosun/.
docker cp ${ID}:/go/bin/scollector $TMP/scollector/.
docker cp ${ID}:/go/bin/tsdbrelay $TMP/tsdbrelay/.
docker cp ${ID}:/hbase $TMP
docker cp ${ID}:/tsdb $TMP
docker rm ${ID}
rm -rf $TMP/hbase/hbase/docs $TMP/tsdb/build/gwt-unitCache
docker build -t stackexchange/bosun run
rm -rf build/bosun $TMP
echo bosun docker image built
