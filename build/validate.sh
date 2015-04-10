#!/bin/bash
cd $GOPATH/src/bosun.org
DIRS=`find . -maxdepth 1 -type d -iregex './[^._].*'`

echo -e "\nChecking gofmt -l -s -w for all folders that don't start with . or _"
GOFMTRESULT=0
GOFMTOUT=$(gofmt -l -s -w $DIRS);
if [ "$GOFMTOUT" != '' ]; then
    echo "The following files need 'gofmt -s -w':"
    echo "$GOFMTOUT"
    GOFMTRESULT=1
fi

echo -e "\nRunning go vet bosun.org/..."
go vet bosun.org/...
GOVETRESULT=$?

echo -e "\nRunning go test bosun.org/..."
go test bosun.org/...
GOTESTRESULT=$?

let "RESULT = $GOFMTRESULT | $GOVETRESULT$ | GOTESTRESULT"
exit $RESULT
