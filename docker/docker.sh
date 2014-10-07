#!/bin/bash

TMP=run/tmp
rm -rf build/bosun $TMP
git clone -b master --single-branch .. build/bosun
docker build -t bosun-build build
ID=$(docker run -d bosun-build)
mkdir -p $TMP/hbase $TMP/bosun $TMP/tsdb
docker cp ${ID}:/go/bin/bosun $TMP/bosun
docker cp ${ID}:/hbase $TMP
docker cp ${ID}:/tsdb $TMP
docker kill ${ID}
docker rm ${ID}
rm -rf $TMP/hbase/hbase-*/docs $TMP/tsdb/build/gwt-unitCache
cp -R ../web $TMP/bosun
docker build -t stackexchange/bosun run
rm -rf build/bosun $TMP
echo bosun docker image built