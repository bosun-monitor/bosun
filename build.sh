#!/bin/bash
set -euxo pipefail


LDFLAGS="-X bosun.org/_version.VersionSHA=$(git rev-parse HEAD) -X bosun.org/_version.VersionDate=$(date -u "+%Y%m%d%H%M%S")"

mkdir -p dist

go generate ./...
gofmt -s -d .

go build -ldflags "$LDFLAGS" ./...
go build -ldflags "$LDFLAGS" -o dist/bosun bosun.org/cmd/bosun
go build -ldflags "$LDFLAGS" -o dist/tsdbrelay bosun.org/cmd/tsdbrelay
go build -ldflags "$LDFLAGS" -o dist/scollector bosun.org/cmd/scollector
go build -ldflags "$LDFLAGS" -o dist/silence bosun.org/cmd/silence

go vet ./...
go test -v ./...
