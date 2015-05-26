#!/bin/bash
cd $GOPATH/src/bosun.org
DIRS=`find . -maxdepth 1 -type d -iregex './[^._].*'`

echo -e "\nChecking gofmt -s -w for all folders that don't start with . or _"
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

echo -e "\nGetting esc"
go get -u -v github.com/mjibson/esc

echo -e "\nRunning go generate bosun.org/..."
go generate bosun.org/...
GOGENERATERESULT=$?
GOGENERATEDIFF=$(git diff --exit-code --name-only)
GOGENERATEDIFFRESULT=0
if [ "$GOGENERATEDIFF" != '' ]; then
    echo "Go generate needs to be run. The following files have changed:"
    echo "$GOGENERATEDIFF"
    GOGENERATEDIFFRESULT=1
fi

echo -e "\nRunning go test bosun.org/..."
go test bosun.org/...
GOTESTRESULT=$?

let "RESULT = $GOFMTRESULT | $GOVETRESULT | $GOTESTRESULT | $GOGENERATERESULT | $GOGENERATEDIFFRESULT"
exit $RESULT
