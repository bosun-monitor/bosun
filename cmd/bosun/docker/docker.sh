#!/bin/bash

set -e

branch=${1-master}

if  type boot2docker >/dev/null 2>&1; then
        boot2docker ssh sudo ntpclient -s -h pool.ntp.org
fi

TMP=run/tmp
rm -rf build/bosun $TMP

echo Cloning $branch to cmd/bosun/docker/build/bosun
git clone -b $branch --single-branch ../../.. build/bosun

echo Creating bosun-build container using cmd/bosun/docker/build/Dockerfile
docker build -t bosun-build build
ID=$(docker create bosun-build)

echo Using bosun-build container ID=$ID
mkdir -p $TMP/hbase $TMP/bosun $TMP/tsdb $TMP/scollector $TMP/tsdbrelay

echo Copying files from bosun-build to cmd/bosun/docker/$TMP/
docker cp ${ID}:/go/bin/bosun $TMP/bosun/.
docker cp ${ID}:/go/bin/scollector $TMP/scollector/.
docker cp ${ID}:/go/bin/tsdbrelay $TMP/tsdbrelay/.
docker cp ${ID}:/hbase $TMP
docker cp ${ID}:/tsdb $TMP

echo Removing bosun-build container and extra hbase/tsdb files
docker rm ${ID}
rm -rf $TMP/hbase/hbase/docs $TMP/tsdb/build/gwt-unitCache

echo Creating bosun container using cmd/bosun/docker/run/Dockerfile
docker build -t stackexchange/bosun run

echo Removing build/bosun and cmd/bosun/docker/$TMP/ files
rm -rf build/bosun $TMP

echo Finished building bosun docker image
