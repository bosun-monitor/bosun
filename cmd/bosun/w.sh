#!/bin/sh

while echo "(RE)STARTING BOSUN"; do
	go run main.go -w -dev -q  || exit
done
