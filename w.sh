#!/bin/sh

while true; do
	go run -race main.go -w
	if [ $? != 0 ] ; then
		exit
	fi
	echo restarting
done
