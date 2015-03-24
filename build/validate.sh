#!/bin/bash
echo -e "\nChecking go fmt bosun.org/..."
GOFMTOUT=$(go fmt bosun.org/...);
if [ "$GOFMTOUT" != '' ]; then 
    echo "The following files need go fmt:";
    echo "$GOFMTOUT";
    exit 1;
fi

echo -e "\nRunning go test bosun.org/..."
go test bosun.org/...
GOTESTRESULT=$?

exit $GOTESTRESULT