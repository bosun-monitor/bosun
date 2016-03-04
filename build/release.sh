#!/bin/sh
set -e

TIME=`date +%Y%m%d%H%M%S`
export GIT_SHA=`cd $GOPATH/src/bosun.org; git rev-parse HEAD`

build()
{
	export GOOS=$1
	export GOARCH=$2
	EXT=""
	if [ $GOOS = "windows" ]; then
		EXT=".exe"
	fi
	if [ $GOARCH = "arm" ]; then
		export GOARM=${3-6}
		EXT="v${GOARM}"
	fi
	echo $GOOS $GOARCH $EXT
	if $BOSUN; then
		go build -o ${OUTPUTDIR}bosun-$GOOS-$GOARCH$EXT -ldflags "-X bosun.org/_version.VersionSHA=$GIT_SHA -X bosun.org/_version.OfficialBuild=true -X bosun.org/_version.VersionDate=$TIME" bosun.org/cmd/bosun
		go build -o ${OUTPUTDIR}tsdbrelay-$GOOS-$GOARCH$EXT -ldflags "-X bosun.org/_version.VersionSHA=$GIT_SHA -X bosun.org/_version.OfficialBuild=true -X bosun.org/_version.VersionDate=$TIME" bosun.org/cmd/tsdbrelay
	fi
	go build -o ${OUTPUTDIR}scollector-$GOOS-$GOARCH$EXT -ldflags "-X bosun.org/_version.VersionSHA=$GIT_SHA -X bosun.org/_version.OfficialBuild=true -X bosun.org/_version.VersionDate=$TIME" bosun.org/cmd/scollector
}

BOSUN=true
for GOOS in windows linux darwin; do
	for GOARCH in amd64 386; do
		build $GOOS $GOARCH
	done
done
BOSUN=false

build linux arm 5
build linux arm 6
build linux arm 7

if [ "$GITHUB_ACCESS_TOKEN" = "" ]; then
	echo GITHUB_ACCESS_TOKEN not set: not running githubRelease.go
else
	GOOS=linux
	GOARCH=amd64
	export BUILD_NUMBER=`${OUTPUTDIR}bosun-linux-amd64 -version | awk '{print $3}'`
	go run build/release/githubRelease.go
fi
