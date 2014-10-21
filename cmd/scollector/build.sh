#!/bin/sh

set -e

rm -rf tmp
mkdir tmp
go run build/build.go > tmp/_config.yml
grep -P "\tVersion" main.go | tr -d '\t'

for GOOS in darwin linux windows; do
	EXT=""
	if [ $GOOS = "windows" ]; then
		EXT=".exe"
	fi
	for GOARCH in 386 amd64; do
		export GOOS=$GOOS
		export GOARCH=$GOARCH
		echo $GOOS $GOARCH $EXT
		go get -d .
		go build -o tmp/scollector-$GOOS-$GOARCH$EXT
	done
done

git checkout main.go
git checkout gh-pages
cp tmp/* .
rm -rf tmp
rm -rf Gemfile*
rm -rf _site