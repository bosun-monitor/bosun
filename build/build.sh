#!/bin/bash

cd $GOPATH/src
go get bosun.org/cmd/bosun
cd $GOPATH/src/bosun.org/cmd/bosun
go build
ls $GOPATH/src/bosun.org/cmd/bosun
