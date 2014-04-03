#!/bin/sh

cd ../ui
esc -p miniprofiler -f ../miniprofiler/static.go *.html *.css *.js *.tmpl
