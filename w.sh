#!/bin/sh

(sleep 1; touch w.sh)&

/usr/local/opt/ruby193/bin/filewatcher -l -r . "sh k.sh"