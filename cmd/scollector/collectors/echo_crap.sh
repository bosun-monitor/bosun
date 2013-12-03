#!/bin/bash
while :; do
	#if it is in the spirit of tcollector, the hostname should be added by the main loop
	echo "put crap $(date +%s) $(( ( RANDOM % 10 )  + 1 ))"
	sleep 15
done
	
