#!/bin/sh

set -e

go get -d -v bosun.org/cmd/bosun
go run build.go > _config.yml
cat _config.yml

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
	go build -o bosun-$GOOS-$GOARCH$EXT bosun.org/cmd/bosun
	go build -o scollector-$GOOS-$GOARCH$EXT bosun.org/cmd/scollector
}

build linux arm 5
build linux arm 6
build linux arm 7

for GOOS in windows linux darwin; do
	for GOARCH in amd64 386; do
		build $GOOS $GOARCH
	done
done