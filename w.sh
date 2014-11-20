#!/bin/sh

while true; do
	go run main.go -w -dev -r -q
	esc -o web/static.go -pkg web -prefix web/static web/static/
	if [ $? != 0 ] ; then
		exit
	fi
	echo restarting
done
