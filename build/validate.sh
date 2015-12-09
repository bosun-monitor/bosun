#!/bin/bash
cd $GOPATH/src/bosun.org
DIRS=`find . -maxdepth 1 -type d -iregex './[^._].*'`

O=bosun-monitor
R=bosun
SHA=`git rev-parse ${TRAVIS_COMMIT}^2`
BUILDMSG=""
if [ "$TRAVIS" != '' ]; then
	setStatus -o $O -r $R -s pending -c bosun -d="Running validation build in travis" -sha=$SHA
fi


echo -e "\nBuilding/..."

GOBUILDRESULT=0
GBUILDRESULT=0
for GOOS in darwin windows linux ; do
	export GOOS=$GOOS
	go build bosun.org/...
	GBUILDRESULT=$?
	if [ "$GBUILDRESULT" != 0 ]; then
		BUILDMSG="${BUILDMSG}Does not build on ${GOOS}. "
		GOBUILDRESULT=$GBUILDRESULT
	fi
done



echo -e "\nChecking gofmt -s -w for all folders that don't start with . or _"
GOFMTRESULT=0
GOFMTOUT=$(gofmt -l -s -w $DIRS);
if [ "$GOFMTOUT" != '' ]; then
    echo "The following files need 'gofmt -s -w':"
    echo "$GOFMTOUT"
    GOFMTRESULT=1
	BUILDMSG="${BUILDMSG}go fmt -s needed. "
fi

echo -e "\nRunning go vet bosun.org/..."
go vet bosun.org/...
GOVETRESULT=$?
if [ "$GOVETRESULT" != 0 ]; then
	BUILDMSG="${BUILDMSG}go vet found problems. "
fi

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
    git diff
	BUILDMSG="${BUILDMSG}go generate needs to run. "
    GOGENERATEDIFFRESULT=1
fi

echo -e "\nRunning go test bosun.org/..."
go test bosun.org/...
GOTESTRESULT=$?
if [ "$GOTESTRESULT" != 0 ]; then
	BUILDMSG="${BUILDMSG}tests fail."
fi

BUILDSTATUS=failure
if [ "$BUILDMSG" == '' ]; then
	BUILDMSG="All checks Passed!"
	BUILDSTATUS=success
fi

if [ "$TRAVIS" != '' ]; then
	setStatus -o $O -r $R -s=$BUILDSTATUS -c bosun -d="$BUILDMSG" -sha=$SHA
fi

let "RESULT = $GOBUILDRESULT | $GOFMTRESULT | $GOVETRESULT | $GOTESTRESULT | $GOGENERATERESULT | $GOGENERATEDIFFRESULT"
exit $RESULT
