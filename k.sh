#!/bin/sh
kill `ps ax|grep go-build|grep -v grep | grep -v put |awk '{print $1}'`
sleep 1
go run *.go &
