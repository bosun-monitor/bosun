#!/bin/bash
cd $GOPATH/src/bosun.org
DIRS=`find . -maxdepth 1 -type d -iregex './[^._].*' | grep -v '/vendor'`
PKGS=`go list bosun.org/... | grep -v /vendor/`
export GO15VENDOREXPERIMENT=1

O=bosun-monitor
R=bosun
ORIGINALGOOS=$GOOS
SHA=${TRAVIS_COMMIT}
BUILDMSG=""
if [ "$TRAVIS" != '' ]; then
	setStatus -o $O -r $R -s pending -c bosun -d="Running validation build in travis" -sha=$SHA
fi


echo -e "\nBuilding/..."
GOBUILDRESULT=0
GBUILDRESULT=0
for GOOS in darwin windows linux ; do
	export GOOS=$GOOS
	export CGO_ENABLED=0
	echo $GOOS
	go build bosun.org/...
	GBUILDRESULT=$?
	if [ "$GBUILDRESULT" != 0 ]; then
		BUILDMSG="${BUILDMSG}Does not build on ${GOOS}. "
		GOBUILDRESULT=$GBUILDRESULT
	fi
done
export GOOS=$ORIGINALGOOS

echo -e "\nBuilding with esv5 tag"
go build -tags="esv5" bosun.org/...
GBUILDRESULT=$?
if [ "$GBUILDRESULT" != 0 ]; then
	BUILDMSG="${BUILDMSG}Does not build on ${GOOS} (esv5). "
	GOBUILDRESULT=$GBUILDRESULT
fi

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
go vet $PKGS
GOVETRESULT=$?
if [ "$GOVETRESULT" != 0 ]; then
	BUILDMSG="${BUILDMSG}go vet found problems. "
fi

echo -e "\nRunning go generate bosun.org/..."
go generate $PKGS
GOGENERATERESULT=$?
GOGENERATEDIFF=$(git diff --exit-code --name-only)
GOGENERATEDIFFRESULT=0
if [ "$GOGENERATEDIFF" != '' ]; then
    echo "Go generate needs to be run. The following files have changed:"
    echo "$GOGENERATEDIFF"
	BUILDMSG="${BUILDMSG}go generate needs to run. "
    GOGENERATEDIFFRESULT=1
    git diff
fi

echo -e "\nRunning go test bosun.org/..."
go test -v $PKGS
GOTESTRESULT=$?
if [ "$GOTESTRESULT" != 0 ]; then
	BUILDMSG="${BUILDMSG}tests fail."
fi

echo -e "\nTesting that bosun starts and stops cleanly"
#TODO: save linux build from above? cant seem to find where it is though.
cd $GOPATH/src/bosun.org/cmd/bosun
go build .
echo -e 'RuleFilePath = "rule.conf"' > bosun.toml
echo "" > rule.conf
timeout 30 ./bosun & bosunpid=$! #Run bosun in background with a 30s timeout and capture the pid
BOSUN_START_RESULT=$?
sleep 5
kill -SIGTERM $bosunpid
BOSUN_SIGNAL_RESULT=$?
wait $bosunpid
BOSUN_STOP_RESULT=$?
if [ "$BOSUN_START_RESULT" != 0 ]; then
    echo "Failed to start bosun cleanly. Exit code $BOSUN_START_RESULT"
fi
if [ "$BOSUN_SIGNAL_RESULT" != 0 ]; then
    echo "Failed to signal bosun to stop cleanly. Likely crashed before signal sent."
fi
if [ "$BOSUN_STOP_RESULT" != 1 ]; then
    echo "Failed to stop bosun cleanly. Exit code $BOSUN_STOP_RESULT (124=60s test timeout reached)"
else # Expected is 1, so reset to 0 if it worked
    BOSUN_STOP_RESULT=0
fi
let "RUN_BOSUN = $BOSUN_START_RESULT | $BOSUN_SIGNAL_RESULT | $BOSUN_STOP_RESULT"
if [ "$RUN_BOSUN" != 0 ]; then
    BUILDMSG="${BUILDMSG}clean start/signal/stop failed. "
fi

BUILDSTATUS=failure
if [ "$BUILDMSG" == '' ]; then
	BUILDMSG="All checks Passed!"
	BUILDSTATUS=success
fi

if [ "$TRAVIS" != '' ]; then
	setStatus -o $O -r $R -s=$BUILDSTATUS -c bosun -d="$BUILDMSG" -sha=$SHA
fi

let "RESULT = $GOBUILDRESULT | $GOFMTRESULT | $GOVETRESULT | $GOTESTRESULT | $GOGENERATERESULT | $GOGENERATEDIFFRESULT | $RUN_BOSUN"
exit $RESULT
