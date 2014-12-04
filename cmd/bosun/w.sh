#!/bin/sh

while true; do
	esc -o web/static.go -pkg web -prefix web/static web/static/
	go run main.go -w -dev -r -q
	if [ $? != 0 ] ; then
		esc -o web/static.go -pkg web -prefix web/static web/static/
		exit
	fi
	echo restarting
done
